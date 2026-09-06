package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type workspaceDeletionImpact struct {
	WorkspaceID                       string `json:"workspaceId"`
	WorkspaceName                     string `json:"workspaceName"`
	ScanCount                         int64  `json:"scanCount"`
	APIKeyCount                       int64  `json:"apiKeyCount"`
	OtherMemberCount                  int64  `json:"otherMemberCount"`
	ReplacementWorkspaceWillBeCreated bool   `json:"replacementWorkspaceWillBeCreated"`
}

type accountDeletionImpact struct {
	ConfirmationValue          string `json:"confirmationValue"`
	OwnedWorkspaceCount        int64  `json:"ownedWorkspaceCount"`
	SharedWorkspaceCount       int64  `json:"sharedWorkspaceCount"`
	ScanCount                  int64  `json:"scanCount"`
	APIKeyCount                int64  `json:"apiKeyCount"`
	SharedOwnedWorkspaceCount  int64  `json:"sharedOwnedWorkspaceCount"`
	SubscriptionWillBeCanceled bool   `json:"subscriptionWillBeCanceled"`
	CanDelete                  bool   `json:"canDelete"`
}

type deletionConfirmationRequest struct {
	Confirmation string `json:"confirmation"`
}

func (s *Server) handleWorkspaceDeletionImpact(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingCloud {
		writeError(w, http.StatusNotFound, "workspace deletion is only available in cloud mode")
		return
	}

	user, _ := currentUser(r.Context())
	workspace, ok := s.ownedWorkspaceFromRequest(w, r, user)
	if !ok {
		return
	}

	impact, err := s.workspaceDeletionImpact(r.Context(), workspace, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to calculate workspace deletion impact")
		return
	}

	writeJSON(w, http.StatusOK, impact)
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingCloud {
		writeError(w, http.StatusNotFound, "workspace deletion is only available in cloud mode")
		return
	}

	user, _ := currentUser(r.Context())
	workspace, ok := s.ownedWorkspaceFromRequest(w, r, user)
	if !ok {
		return
	}

	var request deletionConfirmationRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if !workspaceDeletionConfirmed(request.Confirmation, workspace.Name) {
		writeError(w, http.StatusBadRequest, "type the workspace name exactly to confirm deletion")
		return
	}

	impact, err := s.workspaceDeletionImpact(r.Context(), workspace, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to calculate workspace deletion impact")
		return
	}

	if err := s.purgeWorkspaces(r.Context(), []bson.ObjectID{workspace.ID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete workspace data")
		return
	}

	response := map[string]any{
		"status":         "workspace_deleted",
		"deletedScans":   impact.ScanCount,
		"deletedAPIKeys": impact.APIKeyCount,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAccountDeletionImpact(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingCloud {
		writeError(w, http.StatusNotFound, "account deletion is only available in cloud mode")
		return
	}

	user, _ := currentUser(r.Context())
	impact, err := s.accountDeletionImpact(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to calculate account deletion impact")
		return
	}

	writeJSON(w, http.StatusOK, impact)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingCloud {
		writeError(w, http.StatusNotFound, "account deletion is only available in cloud mode")
		return
	}

	user, _ := currentUser(r.Context())
	var request deletionConfirmationRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if !accountDeletionConfirmed(request.Confirmation, user) {
		writeError(w, http.StatusBadRequest, "type the requested value to confirm account deletion")
		return
	}

	impact, err := s.accountDeletionImpact(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to calculate account deletion impact")
		return
	}
	if !impact.CanDelete {
		writeError(w, http.StatusConflict, "delete shared workspaces you own before deleting your account")
		return
	}

	if err := s.cancelCloudSubscriptionForDeletion(r.Context(), user); err != nil {
		writeError(w, http.StatusBadGateway, "failed to cancel the active subscription; the account was not deleted")
		return
	}

	ownedWorkspaces, err := s.ownedWorkspaces(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load account workspaces")
		return
	}
	ownedWorkspaceIDs := make([]bson.ObjectID, 0, len(ownedWorkspaces))
	for _, workspace := range ownedWorkspaces {
		ownedWorkspaceIDs = append(ownedWorkspaceIDs, workspace.ID)
	}

	if err := s.purgeAccountData(r.Context(), user, ownedWorkspaceIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete account data")
		return
	}

	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "account_deleted",
		"deletedWorkspaces": impact.OwnedWorkspaceCount,
		"deletedScans":      impact.ScanCount,
		"deletedAPIKeys":    impact.APIKeyCount,
	})
}

func (s *Server) ownedWorkspaceFromRequest(w http.ResponseWriter, r *http.Request, user User) (Workspace, bool) {
	workspaceID, err := bson.ObjectIDFromHex(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return Workspace{}, false
	}

	var workspace Workspace
	if err := s.workspaces.FindOne(r.Context(), bson.M{"_id": workspaceID}).Decode(&workspace); err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return Workspace{}, false
	}
	if workspace.CreatedBy != user.ID {
		writeError(w, http.StatusForbidden, "only the workspace owner can delete it")
		return Workspace{}, false
	}

	return workspace, true
}

func (s *Server) workspaceDeletionImpact(ctx context.Context, workspace Workspace, ownerID bson.ObjectID) (workspaceDeletionImpact, error) {
	scanCount, err := s.scans.CountDocuments(ctx, bson.M{"workspace_id": workspace.ID})
	if err != nil {
		return workspaceDeletionImpact{}, err
	}
	apiKeyCount, err := s.apiKeys.CountDocuments(ctx, bson.M{"workspace_id": workspace.ID})
	if err != nil {
		return workspaceDeletionImpact{}, err
	}
	otherMemberCount, err := s.users.CountDocuments(ctx, bson.M{
		"_id":           bson.M{"$ne": ownerID},
		"workspace_ids": workspace.ID,
	})
	if err != nil {
		return workspaceDeletionImpact{}, err
	}

	return workspaceDeletionImpact{
		WorkspaceID:                       workspace.ID.Hex(),
		WorkspaceName:                     workspace.Name,
		ScanCount:                         scanCount,
		APIKeyCount:                       apiKeyCount,
		OtherMemberCount:                  otherMemberCount,
		ReplacementWorkspaceWillBeCreated: false,
	}, nil
}

func (s *Server) accountDeletionImpact(ctx context.Context, user User) (accountDeletionImpact, error) {
	ownedWorkspaces, err := s.ownedWorkspaces(ctx, user.ID)
	if err != nil {
		return accountDeletionImpact{}, err
	}

	ownedWorkspaceIDs := make([]bson.ObjectID, 0, len(ownedWorkspaces))
	sharedOwnedWorkspaceCount := int64(0)
	for _, workspace := range ownedWorkspaces {
		ownedWorkspaceIDs = append(ownedWorkspaceIDs, workspace.ID)
		members, err := s.users.CountDocuments(ctx, bson.M{
			"_id":           bson.M{"$ne": user.ID},
			"workspace_ids": workspace.ID,
		})
		if err != nil {
			return accountDeletionImpact{}, err
		}
		if members > 0 {
			sharedOwnedWorkspaceCount++
		}
	}

	scanCount := int64(0)
	if len(ownedWorkspaceIDs) > 0 {
		scanCount, err = s.scans.CountDocuments(ctx, bson.M{"workspace_id": bson.M{"$in": ownedWorkspaceIDs}})
		if err != nil {
			return accountDeletionImpact{}, err
		}
	}

	apiKeyFilter := bson.M{"created_by": user.ID}
	if len(ownedWorkspaceIDs) > 0 {
		apiKeyFilter = bson.M{"$or": []bson.M{
			{"created_by": user.ID},
			{"workspace_id": bson.M{"$in": ownedWorkspaceIDs}},
		}}
	}
	apiKeyCount, err := s.apiKeys.CountDocuments(ctx, apiKeyFilter)
	if err != nil {
		return accountDeletionImpact{}, err
	}

	ownedWorkspaceSet := make(map[bson.ObjectID]struct{}, len(ownedWorkspaceIDs))
	for _, workspaceID := range ownedWorkspaceIDs {
		ownedWorkspaceSet[workspaceID] = struct{}{}
	}
	sharedWorkspaceCount := int64(0)
	for _, workspaceID := range user.WorkspaceIDs {
		if _, owned := ownedWorkspaceSet[workspaceID]; !owned {
			sharedWorkspaceCount++
		}
	}

	subscriptions, err := s.billingSubscriptionsForUser(ctx, user)
	if err != nil {
		return accountDeletionImpact{}, err
	}
	subscriptionWillBeCanceled := false
	for _, subscription := range subscriptions {
		if subscriptionStatusActive(subscription.Status) {
			subscriptionWillBeCanceled = true
			break
		}
	}

	return accountDeletionImpact{
		ConfirmationValue:          accountDeletionConfirmationValue(user),
		OwnedWorkspaceCount:        int64(len(ownedWorkspaces)),
		SharedWorkspaceCount:       sharedWorkspaceCount,
		ScanCount:                  scanCount,
		APIKeyCount:                apiKeyCount,
		SharedOwnedWorkspaceCount:  sharedOwnedWorkspaceCount,
		SubscriptionWillBeCanceled: subscriptionWillBeCanceled,
		CanDelete:                  sharedOwnedWorkspaceCount == 0,
	}, nil
}

func (s *Server) ownedWorkspaces(ctx context.Context, ownerID bson.ObjectID) ([]Workspace, error) {
	cursor, err := s.workspaces.Find(
		ctx,
		bson.M{"created_by": ownerID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cursor)

	workspaces := make([]Workspace, 0)
	if err := cursor.All(ctx, &workspaces); err != nil {
		return nil, err
	}
	return workspaces, nil
}

func (s *Server) purgeWorkspaces(ctx context.Context, workspaceIDs []bson.ObjectID) error {
	if len(workspaceIDs) == 0 {
		return nil
	}
	filter := bson.M{"$in": workspaceIDs}

	// Revoke ingest credentials before removing scan data so no new scan can
	// arrive halfway through the purge.
	if _, err := s.apiKeys.DeleteMany(ctx, bson.M{"workspace_id": filter}); err != nil {
		return fmt.Errorf("delete workspace api keys: %w", err)
	}
	if _, err := s.scans.DeleteMany(ctx, bson.M{"workspace_id": filter}); err != nil {
		return fmt.Errorf("delete workspace scans: %w", err)
	}
	if _, err := s.workspaces.DeleteMany(ctx, bson.M{"_id": filter}); err != nil {
		return fmt.Errorf("delete workspaces: %w", err)
	}
	if _, err := s.users.UpdateMany(ctx, bson.M{"workspace_ids": filter}, bson.M{
		"$pull": bson.M{"workspace_ids": filter},
	}); err != nil {
		return fmt.Errorf("remove workspace memberships: %w", err)
	}
	return nil
}

func (s *Server) cancelCloudSubscriptionForDeletion(ctx context.Context, user User) error {
	subscriptions, err := s.billingSubscriptionsForUser(ctx, user)
	if err != nil {
		return err
	}
	for _, subscription := range subscriptions {
		if !subscriptionStatusActive(subscription.Status) {
			continue
		}

		stripeSubscriptionID := strings.TrimSpace(subscription.StripeSubscriptionID)
		if stripeSubscriptionID != "" {
			if strings.TrimSpace(s.cfg.StripeSecretKey) == "" {
				return errors.New("stripe is not configured")
			}
			request, err := http.NewRequestWithContext(
				ctx,
				http.MethodDelete,
				stripeAPIBase+"/subscriptions/"+url.PathEscape(stripeSubscriptionID),
				nil,
			)
			if err != nil {
				return err
			}
			err = s.sendStripeRequest(request, nil)
			var stripeErr stripeHTTPError
			if err != nil && (!errors.As(err, &stripeErr) || stripeErr.StatusCode != http.StatusNotFound) {
				return err
			}
		}

		if _, err := s.billingSubscriptions.UpdateOne(ctx, bson.M{"_id": subscription.ID}, bson.M{
			"$set": bson.M{
				"status":               "canceled",
				"cancel_at_period_end": false,
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) billingSubscriptionsForUser(ctx context.Context, user User) ([]BillingSubscription, error) {
	clauses := []bson.M{}
	if !user.ID.IsZero() {
		clauses = append(clauses, bson.M{"user_id": user.ID})
	}
	if email := strings.ToLower(strings.TrimSpace(user.Email)); email != "" {
		clauses = append(clauses, bson.M{"email": email})
	}
	if len(clauses) == 0 {
		return []BillingSubscription{}, nil
	}

	cursor, err := s.billingSubscriptions.Find(ctx, bson.M{
		"deployment_mode": hostingCloud,
		"$or":             clauses,
	})
	if errors.Is(err, mongo.ErrNoDocuments) {
		return []BillingSubscription{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cursor)

	subscriptions := make([]BillingSubscription, 0)
	if err := cursor.All(ctx, &subscriptions); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (s *Server) purgeAccountData(ctx context.Context, user User, ownedWorkspaceIDs []bson.ObjectID) error {
	if len(ownedWorkspaceIDs) > 0 {
		if err := s.purgeWorkspaces(ctx, ownedWorkspaceIDs); err != nil {
			return err
		}
	}

	// Keys created by this user in somebody else's workspace must also stop
	// working when their identity is removed.
	if _, err := s.apiKeys.DeleteMany(ctx, bson.M{"created_by": user.ID}); err != nil {
		return fmt.Errorf("delete user api keys: %w", err)
	}
	if _, err := s.sessions.DeleteMany(ctx, bson.M{"user_id": user.ID}); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}

	if email := strings.ToLower(strings.TrimSpace(user.Email)); email != "" {
		if _, err := s.emailCodes.DeleteMany(ctx, bson.M{"email": email}); err != nil {
			return fmt.Errorf("delete email login codes: %w", err)
		}
	}
	lockoutKeys := []string{passwordLockoutKey(user.Username)}
	if email := strings.ToLower(strings.TrimSpace(user.Email)); email != "" {
		lockoutKeys = append(lockoutKeys, emailLockoutKey(email))
	}
	if _, err := s.loginLockouts.DeleteMany(ctx, bson.M{"key": bson.M{"$in": lockoutKeys}}); err != nil {
		return fmt.Errorf("delete login lockouts: %w", err)
	}

	billingClauses := []bson.M{{"user_id": user.ID}}
	if email := strings.ToLower(strings.TrimSpace(user.Email)); email != "" {
		billingClauses = append(billingClauses, bson.M{"email": email})
	}
	if _, err := s.billingSubscriptions.DeleteMany(ctx, bson.M{"$or": billingClauses}); err != nil {
		return fmt.Errorf("delete billing records: %w", err)
	}
	if _, err := s.users.DeleteOne(ctx, bson.M{"_id": user.ID}); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func workspaceDeletionConfirmed(value, workspaceName string) bool {
	return strings.TrimSpace(value) == strings.TrimSpace(workspaceName)
}

func accountDeletionConfirmationValue(user User) string {
	if email := strings.ToLower(strings.TrimSpace(user.Email)); email != "" {
		return email
	}
	return strings.TrimSpace(user.Username)
}

func accountDeletionConfirmed(value string, user User) bool {
	return strings.EqualFold(strings.TrimSpace(value), accountDeletionConfirmationValue(user))
}
