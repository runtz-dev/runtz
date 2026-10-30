package api

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/cloudgcp"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type cloudCounts struct {
	Total    int `bson:"total" json:"total"`
	Critical int `bson:"critical" json:"critical"`
	High     int `bson:"high" json:"high"`
	Medium   int `bson:"medium" json:"medium"`
	Low      int `bson:"low" json:"low"`
	Unknown  int `bson:"unknown" json:"unknown"`
}

type cloudConnection struct {
	ID             bson.ObjectID `bson:"_id" json:"id"`
	WorkspaceID    bson.ObjectID `bson:"workspace_id" json:"workspaceId"`
	Provider       string        `bson:"provider" json:"provider"`
	Name           string        `bson:"name" json:"name"`
	ProjectID      string        `bson:"project_id" json:"projectId"`
	Location       string        `bson:"location" json:"location"`
	ServiceAccount string        `bson:"service_account" json:"serviceAccount"`
	Credential     string        `bson:"credential" json:"-"`
	CreatedBy      bson.ObjectID `bson:"created_by" json:"createdBy"`
	Status         string        `bson:"status" json:"status"`
	LastError      string        `bson:"last_error" json:"lastError,omitempty"`
	Summary        cloudCounts   `bson:"summary" json:"summary"`
	SnapshotID     bson.ObjectID `bson:"snapshot_id" json:"-"`
	LastScanAt     *time.Time    `bson:"last_scan_at,omitempty" json:"lastScanAt,omitempty"`
	ScanLockUntil  *time.Time    `bson:"scan_lock_until,omitempty" json:"-"`
	CreatedAt      time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt      time.Time     `bson:"updated_at" json:"updatedAt"`
}

type cloudFindingRecord struct {
	cloudgcp.Finding `bson:",inline"`
	ConnectionID     bson.ObjectID `bson:"connection_id"`
	WorkspaceID      bson.ObjectID `bson:"workspace_id"`
	SnapshotID       bson.ObjectID `bson:"snapshot_id"`
}

type cloudConnectRequest struct {
	WorkspaceID string          `json:"workspaceId"`
	Name        string          `json:"name"`
	ProjectID   string          `json:"projectId"`
	Location    string          `json:"location"`
	Credentials json.RawMessage `json:"credentials"`
}

func (s *Server) registerCloudRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/cloud/connections", s.auth(http.HandlerFunc(s.handleListCloudConnections)))
	mux.Handle("POST /api/v1/cloud/gcp/validate", s.auth(http.HandlerFunc(s.handleValidateGCP)))
	mux.Handle("POST /api/v1/cloud/gcp/connections", s.auth(http.HandlerFunc(s.handleConnectGCP)))
	mux.Handle("GET /api/v1/cloud/connections/{id}", s.auth(http.HandlerFunc(s.handleGetCloudConnection)))
	mux.Handle("GET /api/v1/cloud/connections/{id}/findings", s.auth(http.HandlerFunc(s.handleCloudFindings)))
	mux.Handle("POST /api/v1/cloud/connections/{id}/scan", s.auth(http.HandlerFunc(s.handleScanCloud)))
	mux.Handle("PATCH /api/v1/cloud/connections/{id}/credentials", s.auth(http.HandlerFunc(s.handleRotateCloudCredentials)))
	mux.Handle("DELETE /api/v1/cloud/connections/{id}", s.auth(http.HandlerFunc(s.handleDeleteCloudConnection)))
}

func (s *Server) ensureCloudIndexes(ctx context.Context) error {
	_, err := s.cloudConnections.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "workspace_id", Value: 1}, {Key: "provider", Value: 1}, {Key: "project_id", Value: 1}, {Key: "location", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("create cloud connection indexes: %w", err)
	}
	_, err = s.cloudFindings.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "connection_id", Value: 1}, {Key: "snapshot_id", Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "workspace_id", Value: 1}, {Key: "snapshot_id", Value: 1}, {Key: "severity", Value: 1}}},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func cloudAAD(c cloudConnection) []byte {
	return []byte("runtz:cloud:v1:" + c.WorkspaceID.Hex() + ":" + c.ID.Hex() + ":" + c.Provider)
}
func cloudCipher(key string) (cipher.AEAD, error) {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(b) != 32 {
		return nil, errors.New("cloud credential encryption is not configured; set RUNTZ_CLOUD_ENCRYPTION_KEY")
	}
	block, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func sealCloudCredential(key string, c cloudConnection, raw []byte) (string, error) {
	aead, err := cloudCipher(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return "v1:" + base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, raw, cloudAAD(c))), nil
}
func openCloudCredential(key string, c cloudConnection) ([]byte, error) {
	aead, err := cloudCipher(key)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(c.Credential, "v1:") {
		return nil, errors.New("unknown cloud credential format")
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(c.Credential, "v1:"))
	if err != nil || len(b) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("invalid encrypted credential")
	}
	return aead.Open(nil, b[:aead.NonceSize()], b[aead.NonceSize():], cloudAAD(c))
}

func (s *Server) cloudScope(w http.ResponseWriter, r *http.Request) (bson.M, bool) {
	user, _ := currentUser(r.Context())
	filter := bson.M{}
	if raw := r.URL.Query().Get("workspaceId"); raw != "" && raw != "all" {
		id, err := bson.ObjectIDFromHex(raw)
		if err != nil {
			writeError(w, 400, "invalid workspace id")
			return nil, false
		}
		if !s.userCanAccessWorkspace(user, id) {
			writeError(w, 403, "workspace access required")
			return nil, false
		}
		filter["workspace_id"] = id
	} else if !s.globalDataScope(user) {
		filter["workspace_id"] = bson.M{"$in": user.WorkspaceIDs}
	}
	return filter, true
}

// Only workspace owners (or self-hosted admins) can manage long-lived keys.
func (s *Server) cloudWorkspace(w http.ResponseWriter, r *http.Request, id bson.ObjectID, manage bool) bool {
	user, _ := currentUser(r.Context())
	if !s.userCanAccessWorkspace(user, id) {
		writeError(w, 403, "workspace access required")
		return false
	}
	var workspace Workspace
	if err := s.workspaces.FindOne(r.Context(), bson.M{"_id": id, "deleting": bson.M{"$ne": true}}).Decode(&workspace); err != nil {
		writeError(w, 404, "workspace not found")
		return false
	}
	if workspace.Kind == "playground" {
		writeError(w, 403, "cloud integration is unavailable in the playground")
		return false
	}
	if manage && ((s.cfg.DeploymentMode == hostingCloud && workspace.CreatedBy != user.ID) || (s.cfg.DeploymentMode != hostingCloud && user.Role != "admin")) {
		writeError(w, 403, "only the workspace owner or self-hosted admin can manage cloud connections")
		return false
	}
	return true
}

func (s *Server) cloudFromRequest(w http.ResponseWriter, r *http.Request, manage bool) (cloudConnection, bool) {
	filter, ok := s.cloudScope(w, r)
	if !ok {
		return cloudConnection{}, false
	}
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid connection id")
		return cloudConnection{}, false
	}
	filter["_id"] = id
	var c cloudConnection
	if err := s.cloudConnections.FindOne(r.Context(), filter).Decode(&c); err != nil {
		writeError(w, 404, "cloud connection not found")
		return c, false
	}
	if !s.cloudWorkspace(w, r, c.WorkspaceID, manage) {
		return c, false
	}
	c = cloudConnectionView(c)
	return c, true
}

func cloudConnectionView(c cloudConnection) cloudConnection {
	if c.Status == "syncing" && (c.ScanLockUntil == nil || c.ScanLockUntil.Before(time.Now())) {
		c.Status = "error"
		c.LastError = "the previous import was interrupted; retry importing findings"
	}
	return c
}

func (s *Server) decodeCloudConnect(w http.ResponseWriter, r *http.Request) (cloudConnectRequest, cloudgcp.Credentials, bool) {
	var request cloudConnectRequest
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	if !decodeJSON(w, r, &request) {
		return request, cloudgcp.Credentials{}, false
	}
	id, err := bson.ObjectIDFromHex(request.WorkspaceID)
	if err != nil {
		writeError(w, 400, "select a workspace for this connection")
		return request, cloudgcp.Credentials{}, false
	}
	if !s.cloudWorkspace(w, r, id, true) {
		return request, cloudgcp.Credentials{}, false
	}
	if _, err := cloudCipher(s.cfg.CloudEncryptionKey); err != nil {
		writeError(w, 503, err.Error())
		return request, cloudgcp.Credentials{}, false
	}
	c, err := cloudgcp.ParseCredentials(request.Credentials)
	if err != nil {
		writeError(w, 400, err.Error())
		return request, c, false
	}
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	if request.ProjectID == "" {
		request.ProjectID = c.ProjectID
	}
	if request.Location == "" {
		request.Location = "global"
	}
	if !cloudgcp.ValidProject(request.ProjectID) || !cloudgcp.ValidLocation(request.Location) {
		writeError(w, 400, "invalid project ID or SCC data location")
		return request, c, false
	}
	return request, c, true
}

func (s *Server) handleListCloudConnections(w http.ResponseWriter, r *http.Request) {
	filter, ok := s.cloudScope(w, r)
	if !ok {
		return
	}
	cursor, err := s.cloudConnections.Find(r.Context(), filter, options.Find().SetProjection(bson.M{"credential": 0}).SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		writeError(w, 500, "failed to list cloud connections")
		return
	}
	defer closeCursor(r.Context(), cursor)
	connections := make([]cloudConnection, 0)
	if err := cursor.All(r.Context(), &connections); err != nil {
		writeError(w, 500, "failed to read cloud connections")
		return
	}
	for i := range connections {
		connections[i] = cloudConnectionView(connections[i])
	}
	_, keyErr := cloudCipher(s.cfg.CloudEncryptionKey)
	writeJSON(w, 200, map[string]any{"connections": connections, "configured": keyErr == nil})
}

func (s *Server) handleValidateGCP(w http.ResponseWriter, r *http.Request) {
	request, creds, ok := s.decodeCloudConnect(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	if _, err := s.readGCP(ctx, request.Credentials, request.ProjectID, request.Location, true); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"valid": true, "projectId": request.ProjectID, "serviceAccount": creds.ClientEmail})
}

func (s *Server) handleConnectGCP(w http.ResponseWriter, r *http.Request) {
	request, creds, ok := s.decodeCloudConnect(w, r)
	if !ok {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 80 {
		writeError(w, 400, "enter a connection name of up to 80 characters")
		return
	}
	user, _ := currentUser(r.Context())
	workspaceID, _ := bson.ObjectIDFromHex(request.WorkspaceID)
	now := time.Now().UTC()
	c := cloudConnection{ID: bson.NewObjectID(), WorkspaceID: workspaceID, Provider: "gcp", Name: request.Name, ProjectID: request.ProjectID, Location: request.Location, ServiceAccount: creds.ClientEmail, CreatedBy: user.ID, Status: "connected", CreatedAt: now, UpdatedAt: now, SnapshotID: bson.NewObjectID()}
	var err error
	c.Credential, err = sealCloudCredential(s.cfg.CloudEncryptionKey, c, request.Credentials)
	if err != nil {
		writeError(w, 500, "failed to encrypt cloud credential")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	findings, err := s.readGCP(ctx, request.Credentials, c.ProjectID, c.Location, false)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	if err := s.storeCloudSnapshot(ctx, c, findings); err != nil {
		s.removeCloudSnapshot(c.ID, c.SnapshotID)
		writeError(w, 500, "failed to save cloud findings")
		return
	}
	c.Summary = countCloudFindings(findings)
	finished := time.Now().UTC()
	c.LastScanAt = &finished
	c.UpdatedAt = finished
	if _, err := s.cloudConnections.InsertOne(ctx, c); err != nil {
		s.removeCloudSnapshot(c.ID, c.SnapshotID)
		if mongo.IsDuplicateKeyError(err) {
			writeError(w, 409, "this project and SCC location are already connected in this workspace")
			return
		}
		writeError(w, 500, "failed to save cloud connection")
		return
	}
	// A workspace purge marks the workspace before deleting connections. The
	// post-insert check closes the race with an import started before the purge.
	count, err := s.workspaces.CountDocuments(ctx, bson.M{"_id": c.WorkspaceID, "deleting": bson.M{"$ne": true}})
	if err != nil || count == 0 {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = s.cloudConnections.DeleteOne(cleanupCtx, bson.M{"_id": c.ID})
		s.removeCloudSnapshot(c.ID, c.SnapshotID)
		writeError(w, 409, "workspace was removed while connecting; the connection was not saved")
		return
	}
	writeJSON(w, 201, map[string]any{"connection": c})
}

func (s *Server) handleGetCloudConnection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, false)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"connection": c})
}

func (s *Server) handleCloudFindings(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, false)
	if !ok {
		return
	}
	filter := bson.M{"connection_id": c.ID, "snapshot_id": c.SnapshotID}
	state := r.URL.Query().Get("state")
	if state != "all" {
		filter["state"] = "ACTIVE"
		filter["mute"] = bson.M{"$ne": "MUTED"}
	}
	if severity := r.URL.Query().Get("severity"); severity != "" && severity != "all" {
		filter["severity"] = severity
	}
	if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
		if len(search) > 200 {
			writeError(w, 400, "search must be 200 characters or fewer")
			return
		}
		// Literal, case-insensitive substring search, never an uploaded regex.
		filter["$or"] = []bson.M{{"title": bson.M{"$regex": regexp.QuoteMeta(search), "$options": "i"}}, {"resource_name": bson.M{"$regex": regexp.QuoteMeta(search), "$options": "i"}}, {"source": bson.M{"$regex": regexp.QuoteMeta(search), "$options": "i"}}}
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		page = 10000
	}
	total, err := s.cloudFindings.CountDocuments(r.Context(), filter)
	if err != nil {
		writeError(w, 500, "failed to count cloud findings")
		return
	}
	cursor, err := s.cloudFindings.Find(r.Context(), filter, options.Find().SetSort(bson.D{{Key: "severity_rank", Value: 1}, {Key: "id", Value: 1}}).SetSkip(int64((page-1)*50)).SetLimit(50))
	if err != nil {
		writeError(w, 500, "failed to list cloud findings")
		return
	}
	defer closeCursor(r.Context(), cursor)
	records := make([]cloudFindingRecord, 0)
	if err := cursor.All(r.Context(), &records); err != nil {
		writeError(w, 500, "failed to read cloud findings")
		return
	}
	findings := make([]cloudgcp.Finding, 0, len(records))
	for _, record := range records {
		findings = append(findings, record.Finding)
	}
	writeJSON(w, 200, map[string]any{"findings": findings, "total": total, "page": page, "pageSize": 50})
}

func countCloudFindings(findings []cloudgcp.Finding) cloudCounts {
	var c cloudCounts
	for _, f := range findings {
		if f.State != "ACTIVE" || f.Mute == "MUTED" {
			continue
		}
		c.Total++
		switch f.Severity {
		case "critical":
			c.Critical++
		case "high":
			c.High++
		case "medium":
			c.Medium++
		case "low":
			c.Low++
		default:
			c.Unknown++
		}
	}
	return c
}

func (s *Server) storeCloudSnapshot(ctx context.Context, c cloudConnection, findings []cloudgcp.Finding) error {
	ranks := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "unknown": 4}
	for start := 0; start < len(findings); start += 100 {
		end := min(start+100, len(findings))
		docs := make([]any, 0, end-start)
		for _, f := range findings[start:end] {
			raw, err := bson.Marshal(cloudFindingRecord{Finding: f, ConnectionID: c.ID, WorkspaceID: c.WorkspaceID, SnapshotID: c.SnapshotID})
			if err != nil {
				return err
			}
			var doc bson.M
			if err := bson.Unmarshal(raw, &doc); err != nil {
				return err
			}
			doc["severity_rank"] = ranks[f.Severity]
			docs = append(docs, doc)
		}
		if _, err := s.cloudFindings.InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) removeCloudSnapshot(connectionID, snapshotID bson.ObjectID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.cloudFindings.DeleteMany(ctx, bson.M{"connection_id": connectionID, "snapshot_id": snapshotID})
}

func (s *Server) retireCloudSnapshot(connectionID, snapshotID bson.ObjectID) {
	// Keep an old snapshot briefly for reads that captured its ID immediately
	// before publication. TTL removes it after in-flight readers have finished.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.cloudFindings.UpdateMany(ctx, bson.M{"connection_id": connectionID, "snapshot_id": snapshotID}, bson.M{"$set": bson.M{"expires_at": time.Now().Add(5 * time.Minute)}})
}

func cloudUnlocked(now time.Time) bson.M {
	return bson.M{"$or": []bson.M{{"scan_lock_until": bson.M{"$exists": false}}, {"scan_lock_until": bson.M{"$lt": now}}}}
}

func (s *Server) handleScanCloud(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	lockID := bson.NewObjectID()
	filter := cloudUnlocked(time.Now())
	filter["_id"] = c.ID
	err := s.cloudConnections.FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"scan_lock_id": lockID, "scan_lock_until": time.Now().Add(2 * time.Minute), "status": "syncing"}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeError(w, 409, "a cloud import is already in progress")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to start cloud import")
		return
	}
	completed := false
	finalError := "cloud import was interrupted; previous results were preserved"
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		update := bson.M{"$unset": bson.M{"scan_lock_id": "", "scan_lock_until": ""}}
		if !completed {
			update["$set"] = bson.M{"status": "error", "last_error": finalError}
		}
		_, _ = s.cloudConnections.UpdateOne(releaseCtx, bson.M{"_id": c.ID, "scan_lock_id": lockID}, update)
	}()
	raw, err := openCloudCredential(s.cfg.CloudEncryptionKey, c)
	var findings []cloudgcp.Finding
	message := "could not decrypt the credential; restore the encryption key or replace the service account JSON"
	if err == nil {
		findings, err = s.readGCP(ctx, raw, c.ProjectID, c.Location, false)
		if err != nil {
			message = err.Error()
		}
	}
	oldSnapshot := c.SnapshotID
	c.SnapshotID = bson.NewObjectID()
	if err == nil {
		err = s.storeCloudSnapshot(ctx, c, findings)
		if err != nil {
			message = "could not save findings; previous results were preserved"
		}
	}
	if err != nil {
		s.removeCloudSnapshot(c.ID, c.SnapshotID)
		finalError = message
		writeError(w, 422, message)
		return
	}
	now := time.Now().UTC()
	c.Summary = countCloudFindings(findings)
	c.Status = "connected"
	c.LastError = ""
	c.LastScanAt = &now
	c.UpdatedAt = now
	result, err := s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID, "scan_lock_id": lockID}, bson.M{"$set": bson.M{"snapshot_id": c.SnapshotID, "summary": c.Summary, "status": c.Status, "last_error": "", "last_scan_at": now, "updated_at": now}})
	if err != nil || result.MatchedCount == 0 {
		s.removeCloudSnapshot(c.ID, c.SnapshotID)
		writeError(w, 500, "could not publish findings; previous results were preserved")
		return
	}
	completed = true
	s.retireCloudSnapshot(c.ID, oldSnapshot)
	writeJSON(w, 200, map[string]any{"connection": c})
}

func (s *Server) handleRotateCloudCredentials(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, true)
	if !ok {
		return
	}
	var request struct {
		Credentials json.RawMessage `json:"credentials"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	if !decodeJSON(w, r, &request) {
		return
	}
	creds, err := cloudgcp.ParseCredentials(request.Credentials)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	if _, err := s.readGCP(ctx, request.Credentials, c.ProjectID, c.Location, true); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	sealed, err := sealCloudCredential(s.cfg.CloudEncryptionKey, c, request.Credentials)
	if err != nil {
		writeError(w, 503, "cloud credential encryption is not configured")
		return
	}
	filter := cloudUnlocked(time.Now())
	filter["_id"] = c.ID
	result, err := s.cloudConnections.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"credential": sealed, "service_account": creds.ClientEmail, "updated_at": time.Now().UTC(), "status": "connected", "last_error": ""}})
	if err != nil {
		writeError(w, 500, "could not replace cloud credential")
		return
	}
	if result.MatchedCount == 0 {
		writeError(w, 409, "wait for the current import before replacing credentials")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteCloudConnection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, true)
	if !ok {
		return
	}
	filter := cloudUnlocked(time.Now())
	filter["_id"] = c.ID
	result, err := s.cloudConnections.DeleteOne(r.Context(), filter)
	if err != nil {
		writeError(w, 500, "failed to disconnect cloud")
		return
	}
	if result.DeletedCount == 0 {
		writeError(w, 409, "wait for the current import before disconnecting")
		return
	}
	if _, err := s.cloudFindings.DeleteMany(r.Context(), bson.M{"connection_id": c.ID}); err != nil {
		writeError(w, 500, "credential removed, but findings cleanup failed")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "disconnected"})
}
