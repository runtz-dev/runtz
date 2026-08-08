package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	planFree       = "free"
	planPro        = "pro"
	planEnterprise = "enterprise"

	// unlimitedLimit marks a plan dimension (scans, seats, workspaces) as
	// having no cap. Enterprise is negotiated/custom, so it always reports
	// this instead of a number.
	unlimitedLimit int64 = -1

	freeWeeklyScanLimit  int64 = 250
	freeMonthlyScanLimit int64 = 1_000
	proWeeklyScanLimit   int64 = 2_500
	proMonthlyScanLimit  int64 = 10_000

	// Free's user limit is the one dimension that splits by hosting mode:
	// cloud Free is single-player (1 seat), self-hosted Free may be shared
	// by a small team (25 seats) since it costs runtz no cloud infra either
	// way. Pro/Enterprise don't split — see userLimitForPlan.
	freeUserLimitCloud      int64 = 1
	freeUserLimitSelfHosted int64 = 25
	proUserLimit            int64 = 50

	freeWorkspaceLimit int64 = 1
	proWorkspaceLimit  int64 = 5

	hostingCloud      = "cloud"
	hostingSelfHosted = "self-hosted"

	featureGoogleGitHubAuth = "google_github_auth"
	featureSmartReports     = "smart_reports"
	featureSmartAlerts      = "smart_alerts"
	featureAIAlertAgent     = "ai_alert_agent"

	instanceStateKey = "default"
)

type scanUsageLimits struct {
	Weekly  int64 `json:"weekly"`
	Monthly int64 `json:"monthly"`
}

type Entitlement struct {
	Plan              string   `json:"plan"`
	DeploymentMode    string   `json:"deploymentMode"`
	Status            string   `json:"status"`
	Features          []string `json:"features"`
	LicenseKeyPrefix  string   `json:"licenseKeyPrefix,omitempty"`
	InstallationID    string   `json:"installationId,omitempty"`
	ExpiresAt         string   `json:"expiresAt,omitempty"`
	CurrentPeriodEnd  string   `json:"currentPeriodEnd,omitempty"`
	CancelAtPeriodEnd bool     `json:"cancelAtPeriodEnd,omitempty"`
}

func normalizePlan(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case planPro:
		return planPro
	case planEnterprise:
		return planEnterprise
	default:
		return planFree
	}
}

func normalizeHostingMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), hostingCloud) {
		return hostingCloud
	}
	return hostingSelfHosted
}

func featuresForPlan(plan, deploymentMode string) []string {
	plan = normalizePlan(plan)
	deploymentMode = normalizeHostingMode(deploymentMode)

	features := []string{}
	if deploymentMode == hostingCloud || plan == planPro || plan == planEnterprise {
		features = append(features, featureGoogleGitHubAuth)
	}
	if plan == planPro || plan == planEnterprise {
		features = append(features, featureSmartReports, featureSmartAlerts, featureAIAlertAgent)
	}

	return features
}

func featureEnabled(features []string, feature string) bool {
	for _, item := range features {
		if item == feature {
			return true
		}
	}
	return false
}

func subscriptionStatusActive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "trialing":
		return true
	default:
		return false
	}
}

func planRank(plan string) int {
	switch normalizePlan(plan) {
	case planEnterprise:
		return 3
	case planPro:
		return 2
	default:
		return 1
	}
}

func usageLimitsForPlan(plan string) scanUsageLimits {
	switch normalizePlan(plan) {
	case planEnterprise:
		return scanUsageLimits{Weekly: unlimitedLimit, Monthly: unlimitedLimit}
	case planPro:
		return scanUsageLimits{Weekly: proWeeklyScanLimit, Monthly: proMonthlyScanLimit}
	default:
		return scanUsageLimits{Weekly: freeWeeklyScanLimit, Monthly: freeMonthlyScanLimit}
	}
}

// workspaceLimitForPlan returns the max number of workspaces an account on
// plan may create, or unlimitedLimit for Enterprise (negotiated/custom).
func workspaceLimitForPlan(plan string) int64 {
	switch normalizePlan(plan) {
	case planEnterprise:
		return unlimitedLimit
	case planPro:
		return proWorkspaceLimit
	default:
		return freeWorkspaceLimit
	}
}

// userLimitForPlan returns the max number of users an account on plan may
// have, or unlimitedLimit for Enterprise (negotiated/custom). Free is the
// only tier that splits by deploymentMode: cloud stays single-player, while
// self-hosted may share its one workspace with a small team.
func userLimitForPlan(plan, deploymentMode string) int64 {
	switch normalizePlan(plan) {
	case planEnterprise:
		return unlimitedLimit
	case planPro:
		return proUserLimit
	default:
		if normalizeHostingMode(deploymentMode) == hostingSelfHosted {
			return freeUserLimitSelfHosted
		}
		return freeUserLimitCloud
	}
}

func (s *Server) currentEntitlement(ctx context.Context, user *User) Entitlement {
	if s.cfg.DeploymentMode == hostingSelfHosted {
		return s.selfHostedEntitlement(ctx)
	}

	return s.cloudEntitlement(ctx, user)
}

func (s *Server) cloudEntitlement(ctx context.Context, user *User) Entitlement {
	entitlement := Entitlement{
		Plan:           planFree,
		DeploymentMode: hostingCloud,
		Status:         "free",
		Features:       featuresForPlan(planFree, hostingCloud),
	}
	if user == nil {
		return entitlement
	}

	filter := bson.M{"deployment_mode": hostingCloud}
	clauses := []bson.M{}
	if !user.ID.IsZero() {
		clauses = append(clauses, bson.M{"user_id": user.ID})
	}
	if user.Email != "" {
		clauses = append(clauses, bson.M{"email": strings.ToLower(strings.TrimSpace(user.Email))})
	}
	if len(clauses) == 0 {
		return entitlement
	}
	filter["$or"] = clauses

	cursor, err := s.billingSubscriptions.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if err != nil {
		return entitlement
	}
	defer closeCursor(ctx, cursor)

	var best BillingSubscription
	for cursor.Next(ctx) {
		var subscription BillingSubscription
		if err := cursor.Decode(&subscription); err != nil {
			continue
		}
		if !subscriptionStatusActive(subscription.Status) {
			continue
		}
		if best.ID.IsZero() || planRank(subscription.Plan) > planRank(best.Plan) {
			best = subscription
		}
	}
	if best.ID.IsZero() {
		return entitlement
	}

	if !user.ID.IsZero() && best.UserID.IsZero() {
		_, _ = s.billingSubscriptions.UpdateOne(ctx, bson.M{"_id": best.ID}, bson.M{
			"$set": bson.M{"user_id": user.ID, "updated_at": time.Now().UTC()},
		})
	}

	entitlement.Plan = normalizePlan(best.Plan)
	entitlement.Status = best.Status
	entitlement.Features = featuresForPlan(best.Plan, hostingCloud)
	entitlement.CancelAtPeriodEnd = best.CancelAtPeriodEnd
	if best.CurrentPeriodEnd != nil {
		entitlement.CurrentPeriodEnd = best.CurrentPeriodEnd.Format(time.RFC3339)
	}
	return entitlement
}

func (s *Server) selfHostedEntitlement(ctx context.Context) Entitlement {
	entitlement := Entitlement{
		Plan:           planFree,
		DeploymentMode: hostingSelfHosted,
		Status:         "free",
		Features:       featuresForPlan(planFree, hostingSelfHosted),
	}

	var state InstanceState
	err := s.instanceState.FindOne(ctx, bson.M{"key": instanceStateKey}).Decode(&state)
	if errors.Is(err, mongo.ErrNoDocuments) || err != nil {
		return entitlement
	}

	// Re-verify the signed certificate on every read: never trust the plan or
	// features stored in the database without checking the signature first.
	payload, ok := s.verifiedStoredLicense(state)
	if !ok {
		entitlement.Status = "validation_failed"
		return entitlement
	}
	if payload.ExpiresAt.IsZero() || time.Now().UTC().After(payload.ExpiresAt) || !subscriptionStatusActive(payload.Status) {
		entitlement.Status = "expired"
		if state.LastValidationError != "" {
			entitlement.Status = "validation_failed"
		}
		return entitlement
	}

	entitlement.Plan = normalizePlan(payload.Plan)
	entitlement.Status = payload.Status
	entitlement.Features = payload.Features
	entitlement.LicenseKeyPrefix = payload.LicenseKeyPrefix
	entitlement.InstallationID = payload.InstallationID
	entitlement.ExpiresAt = payload.ExpiresAt.Format(time.RFC3339)
	entitlement.CurrentPeriodEnd = payload.CurrentPeriodEnd
	return entitlement
}

func (s *Server) hasFeature(ctx context.Context, user *User, feature string) bool {
	return featureEnabled(s.currentEntitlement(ctx, user).Features, feature)
}
