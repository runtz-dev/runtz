package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Rolling windows used by the Usage panel in the platform settings. They are
// rolling (last 7 / last 30 days) instead of calendar based so the counters
// never reset to zero on a Monday or on the first day of a month.
const (
	usageWeeklyWindow  = 7 * 24 * time.Hour
	usageMonthlyWindow = 30 * 24 * time.Hour
)

// usageScanTypes is the order the frontend renders the per-type breakdown in.
var usageScanTypes = []string{"sca", "sast", "container", "host", "k8s"}

type usageWindow struct {
	Total  int64            `json:"total"`
	ByType map[string]int64 `json:"byType"`
	Since  string           `json:"since"`
}

type usageLimitError struct {
	Window string
	Limit  int64
}

func (e *usageLimitError) Error() string {
	return fmt.Sprintf(
		"%s scan limit reached (%s); upgrade your plan or wait for older scans to leave the usage window",
		e.Window,
		formatUsageLimit(e.Limit),
	)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	plan := s.currentEntitlement(r.Context(), &user).Plan

	filter := bson.M{}
	if workspaceIDParam := r.URL.Query().Get("workspaceId"); workspaceIDParam != "" {
		workspaceID, err := bson.ObjectIDFromHex(workspaceIDParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid workspace id")
			return
		}
		if !s.userCanAccessWorkspace(user, workspaceID) {
			writeError(w, http.StatusForbidden, "workspace access required")
			return
		}
		var workspace Workspace
		if err := s.workspaces.FindOne(r.Context(), bson.M{"_id": workspaceID}).Decode(&workspace); err != nil {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		plan, err = s.planForWorkspace(r.Context(), workspace.CreatedBy)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load workspace plan")
			return
		}
		filter["workspace_id"] = workspaceID
	} else if !s.globalDataScope(user) {
		filter["workspace_id"] = bson.M{"$in": user.WorkspaceIDs}
	} else {
		// Playground data is generated for the public demo and is never an
		// ingested scan, so it must not consume an installation's allowance.
		var playground Workspace
		if err := s.workspaces.FindOne(r.Context(), bson.M{"slug": playgroundWorkspaceSlug}).Decode(&playground); err == nil {
			filter["workspace_id"] = bson.M{"$ne": playground.ID}
		}
	}

	now := time.Now().UTC()
	weekly, monthly, err := s.countScansInWindows(r.Context(), filter, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count scans")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"weekly":      weekly,
		"monthly":     monthly,
		"limits":      usageLimitsForPlan(plan),
		"plan":        plan,
		"scanTypes":   usageScanTypes,
		"generatedAt": now.Format(time.RFC3339),
	})
}

// enforceScanUsageLimit applies the plan allowance to the workspace resolved
// from the API key. Keeping the check workspace-scoped makes it match the
// counters users see after selecting that workspace in Settings > Usage.
func (s *Server) enforceScanUsageLimit(ctx context.Context, workspaceID, workspaceOwnerID bson.ObjectID) error {
	plan, err := s.planForWorkspace(ctx, workspaceOwnerID)
	if err != nil {
		return err
	}

	weekly, monthly, err := s.countScansInWindows(ctx, bson.M{"workspace_id": workspaceID}, time.Now().UTC())
	if err != nil {
		return err
	}

	return scanUsageLimitError(weekly.Total, monthly.Total, usageLimitsForPlan(plan))
}

func (s *Server) planForWorkspace(ctx context.Context, workspaceOwnerID bson.ObjectID) (string, error) {
	if s.cfg.DeploymentMode == hostingSelfHosted {
		return s.currentEntitlement(ctx, nil).Plan, nil
	}
	if workspaceOwnerID.IsZero() {
		return planFree, nil
	}

	var user User
	if err := s.users.FindOne(ctx, bson.M{"_id": workspaceOwnerID}).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return planFree, nil
		}
		return "", err
	}

	return s.currentEntitlement(ctx, &user).Plan, nil
}

func scanUsageLimitError(weekly, monthly int64, limits scanUsageLimits) error {
	if weekly >= limits.Weekly {
		return &usageLimitError{Window: "weekly", Limit: limits.Weekly}
	}
	if monthly >= limits.Monthly {
		return &usageLimitError{Window: "monthly", Limit: limits.Monthly}
	}
	return nil
}

func formatUsageLimit(limit int64) string {
	switch limit {
	case 1_000_000:
		return "1,000,000"
	case 10_000:
		return "10,000"
	case 2_500:
		return "2,500"
	default:
		return fmt.Sprintf("%d", limit)
	}
}

// countScansInWindows counts the scans matching filter in a single aggregation:
// everything since the monthly cut-off is grouped by scan type, and the weekly
// counter is a conditional sum over the same documents.
func (s *Server) countScansInWindows(ctx context.Context, filter bson.M, now time.Time) (usageWindow, usageWindow, error) {
	weekStart := now.Add(-usageWeeklyWindow)
	monthStart := now.Add(-usageMonthlyWindow)

	match := bson.M{}
	for key, value := range filter {
		match[key] = value
	}
	match["created_at"] = bson.M{"$gte": monthStart}

	cursor, err := s.scans.Aggregate(ctx, []bson.M{
		{"$match": match},
		{"$group": bson.M{
			"_id":     "$type",
			"monthly": bson.M{"$sum": 1},
			"weekly": bson.M{"$sum": bson.M{
				"$cond": []any{bson.M{"$gte": []any{"$created_at", weekStart}}, 1, 0},
			}},
		}},
	})
	if err != nil {
		return usageWindow{}, usageWindow{}, err
	}
	defer closeCursor(ctx, cursor)

	weekly := newUsageWindow(weekStart)
	monthly := newUsageWindow(monthStart)
	for cursor.Next(ctx) {
		var row struct {
			Type    string `bson:"_id"`
			Weekly  int64  `bson:"weekly"`
			Monthly int64  `bson:"monthly"`
		}
		if err := cursor.Decode(&row); err != nil {
			continue
		}
		if row.Type == "" {
			continue
		}
		weekly.ByType[row.Type] += row.Weekly
		weekly.Total += row.Weekly
		monthly.ByType[row.Type] += row.Monthly
		monthly.Total += row.Monthly
	}
	if err := cursor.Err(); err != nil {
		return usageWindow{}, usageWindow{}, err
	}

	return weekly, monthly, nil
}

func newUsageWindow(since time.Time) usageWindow {
	byType := make(map[string]int64, len(usageScanTypes))
	for _, scanType := range usageScanTypes {
		byType[scanType] = 0
	}

	return usageWindow{ByType: byType, Since: since.Format(time.RFC3339)}
}
