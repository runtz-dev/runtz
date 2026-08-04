package api

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
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

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())

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
		filter["workspace_id"] = workspaceID
	} else if !s.globalDataScope(user) {
		filter["workspace_id"] = bson.M{"$in": user.WorkspaceIDs}
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
		"scanTypes":   usageScanTypes,
		"generatedAt": now.Format(time.RFC3339),
	})
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
