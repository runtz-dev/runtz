package api

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type workspaceMember struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (s *Server) sharingWorkspace(w http.ResponseWriter, r *http.Request) (Workspace, User, bool) {
	user, _ := currentUser(r.Context())
	if s.cfg.DeploymentMode != hostingCloud {
		writeError(w, http.StatusNotFound, "workspace sharing is only available in cloud mode")
		return Workspace{}, user, false
	}
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return Workspace{}, user, false
	}
	var workspace Workspace
	err = s.workspaces.FindOne(r.Context(), bson.M{"_id": id, "created_by": user.ID}).Decode(&workspace)
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeError(w, http.StatusNotFound, "workspace not found or you are not its owner")
		return Workspace{}, user, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace")
		return Workspace{}, user, false
	}
	return workspace, user, true
}

func (s *Server) handleListWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	workspace, _, ok := s.sharingWorkspace(w, r)
	if !ok {
		return
	}
	cursor, err := s.users.Find(r.Context(), bson.M{"workspace_ids": workspace.ID}, options.Find().SetSort(bson.D{{Key: "email", Value: 1}}))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace members")
		return
	}
	defer closeCursor(r.Context(), cursor)
	var users []User
	if err := cursor.All(r.Context(), &users); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace members")
		return
	}
	members := make([]workspaceMember, 0, len(users))
	for _, user := range users {
		role := "member"
		if user.ID == workspace.CreatedBy {
			role = "owner"
		}
		name := user.DisplayName
		if name == "" {
			name = user.Username
		}
		members = append(members, workspaceMember{ID: user.ID.Hex(), Email: user.Email, Name: name, Role: role})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleAddWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspace, owner, ok := s.sharingWorkspace(w, r)
	if !ok {
		return
	}
	plan := s.currentEntitlement(r.Context(), &owner).Plan
	if plan != planPro && plan != planEnterprise {
		writeError(w, http.StatusPaymentRequired, "workspace sharing requires Pro or Enterprise")
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(request.Email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	var member User
	err = s.users.FindOne(r.Context(), bson.M{"email": email}).Decode(&member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeError(w, http.StatusNotFound, "no Runtz account found for this email; ask them to sign up first")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find account")
		return
	}
	// Serialize seat checks across engine replicas. Keep the operation shorter
	// than the lease so concurrent additions cannot exceed the account limit.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	lockID := bson.NewObjectID()
	now := time.Now().UTC()
	lock, err := s.users.UpdateOne(ctx, bson.M{
		"_id": owner.ID,
		"$or": []bson.M{{"sharing_lock_until": bson.M{"$exists": false}}, {"sharing_lock_until": bson.M{"$lt": now}}},
	}, bson.M{"$set": bson.M{"sharing_lock_id": lockID, "sharing_lock_until": now.Add(30 * time.Second)}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check workspace seats")
		return
	}
	if lock.MatchedCount == 0 {
		writeError(w, http.StatusConflict, "another sharing change is in progress; please try again")
		return
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		_, _ = s.users.UpdateOne(releaseCtx, bson.M{"_id": owner.ID, "sharing_lock_id": lockID}, bson.M{"$unset": bson.M{"sharing_lock_id": "", "sharing_lock_until": ""}})
	}()
	ownedIDs, err := s.workspaceIDsOwnedBy(ctx, owner.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check workspace seats")
		return
	}
	// A member of another workspace owned by this account already occupies a seat.
	alreadySeated, err := s.users.CountDocuments(ctx, bson.M{"_id": member.ID, "workspace_ids": bson.M{"$in": ownedIDs}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check workspace seats")
		return
	}
	if limit := userLimitForPlan(plan, hostingCloud); limit >= 0 && alreadySeated == 0 {
		seats, err := s.users.CountDocuments(ctx, bson.M{"workspace_ids": bson.M{"$in": ownedIDs}})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check workspace seats")
			return
		}
		if seats >= limit {
			writeError(w, http.StatusPaymentRequired, "user limit reached for your plan; upgrade to add more members")
			return
		}
	}
	result, err := s.users.UpdateOne(ctx, bson.M{"_id": member.ID}, bson.M{
		"$addToSet": bson.M{"workspace_ids": workspace.ID},
		"$set":      bson.M{"updated_at": now},
	})
	if err != nil || result.MatchedCount == 0 {
		writeError(w, http.StatusInternalServerError, "failed to share workspace")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "workspace_shared"})
}

func (s *Server) handleRemoveWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspace, _, ok := s.sharingWorkspace(w, r)
	if !ok {
		return
	}
	memberID, err := bson.ObjectIDFromHex(r.PathValue("memberID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}
	if memberID == workspace.CreatedBy {
		writeError(w, http.StatusBadRequest, "the workspace owner cannot be removed")
		return
	}
	// Removal remains available after a downgrade, including revoking keys
	// created by that member in this workspace.
	now := time.Now().UTC()
	_, err = s.apiKeys.UpdateMany(r.Context(), bson.M{"workspace_id": workspace.ID, "created_by": memberID}, bson.M{"$set": bson.M{"revoked_at": now, "updated_at": now}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke member API keys")
		return
	}
	_, err = s.users.UpdateOne(r.Context(), bson.M{"_id": memberID}, bson.M{
		"$pull": bson.M{"workspace_ids": workspace.ID},
		"$set":  bson.M{"updated_at": now},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove workspace member")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "member_removed"})
}
