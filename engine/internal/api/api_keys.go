package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	filter := activeAPIKeyFilter()

	if workspaceIDParam := strings.TrimSpace(r.URL.Query().Get("workspaceId")); workspaceIDParam != "" {
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

	cursor, err := s.apiKeys.Find(
		r.Context(),
		filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}
	defer closeCursor(r.Context(), cursor)

	apiKeys := make([]APIKey, 0)
	if err := cursor.All(r.Context(), &apiKeys); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decode api keys")
		return
	}

	response := make([]publicAPIKey, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		response = append(response, serializeAPIKey(apiKey))
	}

	writeJSON(w, http.StatusOK, map[string]any{"apiKeys": response})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	var request struct {
		WorkspaceID   string   `json:"workspaceId"`
		Name          string   `json:"name"`
		Scopes        []string `json:"scopes"`
		ExpiresInDays int      `json:"expiresInDays"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	workspaceID, err := bson.ObjectIDFromHex(strings.TrimSpace(request.WorkspaceID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspaceId is required")
		return
	}
	if !s.userCanAccessWorkspace(user, workspaceID) {
		writeError(w, http.StatusForbidden, "workspace access required")
		return
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "CLI key"
	}

	var workspace Workspace
	if err := s.workspaces.FindOne(r.Context(), bson.M{"_id": workspaceID}).Decode(&workspace); err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if workspace.Kind == "playground" {
		writeError(w, http.StatusBadRequest, "playground workspace cannot issue api keys")
		return
	}

	rawKey, prefix, err := generateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate api key")
		return
	}

	now := time.Now().UTC()
	apiKey := APIKey{
		ID:          bson.NewObjectID(),
		WorkspaceID: workspace.ID,
		Name:        name,
		Prefix:      prefix,
		KeyHash:     hashAPIKey(rawKey),
		Scopes:      normalizeAPIKeyScopes(request.Scopes),
		CreatedBy:   user.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   expiresAtFromDays(now, request.ExpiresInDays),
	}

	if _, err := s.apiKeys.InsertOne(r.Context(), apiKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store api key")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"apiKey": serializeAPIKey(apiKey),
		"key":    rawKey,
	})
}

func (s *Server) handleUpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	apiKeyID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid api key id")
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(name) > 80 {
		writeError(w, http.StatusBadRequest, "name must be 80 characters or fewer")
		return
	}

	var apiKey APIKey
	if err := s.apiKeys.FindOne(r.Context(), bson.M{"_id": apiKeyID}).Decode(&apiKey); err != nil {
		writeError(w, http.StatusNotFound, "api key not found")
		return
	}
	if !s.userCanAccessWorkspace(user, apiKey.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workspace access required")
		return
	}

	result := s.apiKeys.FindOneAndUpdate(
		r.Context(),
		bson.M{"_id": apiKey.ID},
		bson.M{"$set": bson.M{"name": name, "updated_at": time.Now().UTC()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if err := result.Decode(&apiKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update api key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"apiKey": serializeAPIKey(apiKey)})
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	apiKeyID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid api key id")
		return
	}

	var apiKey APIKey
	if err := s.apiKeys.FindOne(r.Context(), bson.M{"_id": apiKeyID}).Decode(&apiKey); err != nil {
		writeError(w, http.StatusNotFound, "api key not found")
		return
	}
	if !s.userCanAccessWorkspace(user, apiKey.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workspace access required")
		return
	}

	result, err := s.apiKeys.DeleteOne(r.Context(), bson.M{"_id": apiKey.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete api key")
		return
	}
	if result.DeletedCount == 0 {
		writeError(w, http.StatusNotFound, "api key not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	apiKeyID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid api key id")
		return
	}

	var apiKey APIKey
	if err := s.apiKeys.FindOne(r.Context(), bson.M{"_id": apiKeyID}).Decode(&apiKey); err != nil {
		writeError(w, http.StatusNotFound, "api key not found")
		return
	}
	if !s.userCanAccessWorkspace(user, apiKey.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workspace access required")
		return
	}

	now := time.Now().UTC()
	result := s.apiKeys.FindOneAndUpdate(
		r.Context(),
		bson.M{"_id": apiKey.ID},
		bson.M{"$set": bson.M{"revoked_at": now, "updated_at": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if err := result.Decode(&apiKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"apiKey": serializeAPIKey(apiKey)})
}

// authenticateIngest resolves the workspace a scan may be written to. It is
// always the workspace the API key belongs to — never one named in the request
// body — so a key issued for one workspace cannot deposit findings in another.
func (s *Server) authenticateIngest(w http.ResponseWriter, r *http.Request) (Workspace, bool) {
	apiKeyValue := apiKeyFromRequest(r)
	if apiKeyValue == "" {
		writeError(w, http.StatusUnauthorized, "api key required")
		return Workspace{}, false
	}

	workspace, _, err := s.workspaceFromAPIKey(r.Context(), apiKeyValue)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid api key")
		return Workspace{}, false
	}
	if err := s.enforceScanUsageLimit(r.Context(), workspace.ID, workspace.CreatedBy); err != nil {
		var limitError *usageLimitError
		if errors.As(err, &limitError) {
			writeError(w, http.StatusTooManyRequests, limitError.Error())
			return Workspace{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to check scan usage")
		return Workspace{}, false
	}

	return workspace, true
}

func (s *Server) workspaceFromAPIKey(ctx context.Context, rawKey string) (Workspace, APIKey, error) {
	prefix, ok := extractAPIKeyPrefix(rawKey)
	if !ok {
		return Workspace{}, APIKey{}, errors.New("invalid api key format")
	}

	var apiKey APIKey
	if err := s.apiKeys.FindOne(ctx, activeAPIKeyFilterWithPrefix(prefix)).Decode(&apiKey); err != nil {
		return Workspace{}, APIKey{}, err
	}

	keyHash := hashAPIKey(rawKey)
	if subtle.ConstantTimeCompare([]byte(keyHash), []byte(apiKey.KeyHash)) != 1 {
		return Workspace{}, APIKey{}, errors.New("api key hash mismatch")
	}
	if !apiKeyAllowsScope(apiKey, "ingest:write") {
		return Workspace{}, APIKey{}, errors.New("api key scope denied")
	}

	var workspace Workspace
	if err := s.workspaces.FindOne(ctx, bson.M{"_id": apiKey.WorkspaceID}).Decode(&workspace); err != nil {
		return Workspace{}, APIKey{}, err
	}

	now := time.Now().UTC()
	_, _ = s.apiKeys.UpdateOne(ctx, bson.M{"_id": apiKey.ID}, bson.M{
		"$set": bson.M{"last_used_at": now, "updated_at": now},
	})

	return workspace, apiKey, nil
}

// handleVerifyKey lets `runtz login` check a token before storing it locally.
// It authenticates exactly like ingest (same active-key lookup, hash compare
// and ingest:write scope) but counts no scan usage, and answers with the
// workspace the key unlocks so the CLI can greet the user.
func (s *Server) handleVerifyKey(w http.ResponseWriter, r *http.Request) {
	apiKeyValue := apiKeyFromRequest(r)
	if apiKeyValue == "" {
		writeError(w, http.StatusUnauthorized, "api key required")
		return
	}

	workspace, apiKey, err := s.workspaceFromAPIKey(r.Context(), apiKeyValue)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid api key")
		return
	}

	writeJSON(w, http.StatusOK, verifyKeyResponse(workspace, apiKey))
}

// verifyKeyResponse deliberately exposes less than serializeAPIKey: a scan
// token entitles its holder to know where it lands and when it dies, not the
// workspace's internal ids (creator, scopes, key id).
func verifyKeyResponse(workspace Workspace, apiKey APIKey) map[string]any {
	key := map[string]any{
		"name":   apiKey.Name,
		"prefix": apiKey.Prefix,
	}
	if apiKey.ExpiresAt != nil {
		key["expiresAt"] = apiKey.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"workspace": map[string]any{
			"id":   workspace.ID.Hex(),
			"name": workspace.Name,
		},
		"apiKey": key,
	}
}

// globalDataScope reports whether user may read data outside the workspaces
// they belong to. Only a self-hosted admin may: there the whole installation
// belongs to one customer, and administering it means seeing all of it. In
// cloud every workspace belongs to a different customer and its scans carry
// that customer's source-code findings, so no role widens data access beyond
// membership — not even admin. Administration itself (users, workspaces,
// licensing) stays role-gated in both modes; see adminOnly.
func (s *Server) globalDataScope(user User) bool {
	return user.Role == "admin" && s.cfg.DeploymentMode != hostingCloud
}

func (s *Server) userCanAccessWorkspace(user User, workspaceID bson.ObjectID) bool {
	if s.globalDataScope(user) {
		return true
	}

	for _, visibleWorkspaceID := range user.WorkspaceIDs {
		if visibleWorkspaceID == workspaceID {
			return true
		}
	}

	return false
}

// expiresAtFromDays returns the expiry timestamp for a newly created key, or
// nil when days is 0/negative (no expiration).
func expiresAtFromDays(from time.Time, days int) *time.Time {
	if days <= 0 {
		return nil
	}
	expiresAt := from.AddDate(0, 0, days)
	return &expiresAt
}

func activeAPIKeyFilter() bson.M {
	now := time.Now().UTC()
	return bson.M{"$and": []bson.M{
		{"$or": []bson.M{
			{"revoked_at": bson.M{"$exists": false}},
			{"revoked_at": nil},
		}},
		{"$or": []bson.M{
			{"expires_at": bson.M{"$exists": false}},
			{"expires_at": nil},
			{"expires_at": bson.M{"$gt": now}},
		}},
	}}
}

func activeAPIKeyFilterWithPrefix(prefix string) bson.M {
	filter := activeAPIKeyFilter()
	filter["prefix"] = prefix
	return filter
}

func generateAPIKey() (rawKey, prefix string, err error) {
	keyID, err := randomHex(8)
	if err != nil {
		return "", "", err
	}
	secret, err := randomHex(32)
	if err != nil {
		return "", "", err
	}

	prefix = "rtz_live_" + keyID
	return fmt.Sprintf("%s_%s", prefix, secret), prefix, nil
}

func randomHex(byteCount int) (string, error) {
	bytes := make([]byte, byteCount)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func hashAPIKey(rawKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawKey)))
	return hex.EncodeToString(sum[:])
}

func extractAPIKeyPrefix(rawKey string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(rawKey), "_")
	if len(parts) != 4 || parts[0] != "rtz" || parts[1] != "live" || parts[2] == "" || parts[3] == "" {
		return "", false
	}

	return strings.Join(parts[:3], "_"), true
}

func apiKeyFromRequest(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Runtz-API-Key")); value != "" {
		return value
	}

	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}

	value := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if strings.HasPrefix(value, "rtz_") {
		return value
	}

	return ""
}

func normalizeAPIKeyScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{"ingest:write"}
	}

	seen := map[string]bool{}
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(strings.ToLower(scope))
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		normalized = append(normalized, scope)
	}
	if len(normalized) == 0 {
		return []string{"ingest:write"}
	}

	return normalized
}

func apiKeyAllowsScope(apiKey APIKey, requiredScope string) bool {
	for _, scope := range apiKey.Scopes {
		if scope == requiredScope || scope == "*" {
			return true
		}
	}

	return false
}
