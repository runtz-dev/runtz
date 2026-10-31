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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/cloudgcp"
	"github.com/runtz-dev/runtz/engine/internal/securityclient"
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
	SecurityConnectionID string                  `bson:"security_connection_id,omitempty" json:"-"`
	RemoteAttemptID      string                  `bson:"remote_attempt_id,omitempty" json:"-"`
	RemoteScanID         string                  `bson:"remote_scan_id,omitempty" json:"-"`
	Coverage             *securityclient.Summary `bson:"coverage,omitempty" json:"coverage,omitempty"`
	ID                   bson.ObjectID           `bson:"_id" json:"id"`
	WorkspaceID          bson.ObjectID           `bson:"workspace_id" json:"workspaceId"`
	Provider             string                  `bson:"provider" json:"provider"`
	Name                 string                  `bson:"name" json:"name"`
	ProjectID            string                  `bson:"project_id" json:"projectId"`
	Location             string                  `bson:"location" json:"location"`
	ServiceAccount       string                  `bson:"service_account" json:"serviceAccount"`
	Credential           string                  `bson:"credential" json:"-"`
	CreatedBy            bson.ObjectID           `bson:"created_by" json:"createdBy"`
	Status               string                  `bson:"status" json:"status"`
	LastError            string                  `bson:"last_error" json:"lastError,omitempty"`
	Summary              cloudCounts             `bson:"summary" json:"summary"`
	SnapshotID           bson.ObjectID           `bson:"snapshot_id" json:"-"`
	LastScanAt           *time.Time              `bson:"last_scan_at,omitempty" json:"lastScanAt,omitempty"`
	ScanLockUntil        *time.Time              `bson:"scan_lock_until,omitempty" json:"-"`
	CreatedAt            time.Time               `bson:"created_at" json:"createdAt"`
	UpdatedAt            time.Time               `bson:"updated_at" json:"updatedAt"`
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
	mux.Handle("GET /api/v1/cloud/connections/{id}/checks", s.auth(http.HandlerFunc(s.handleCloudChecks)))
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
	if c.SecurityConnectionID == "" && c.Status != "connecting" {
		c.Status = "reconnect_required"
		c.LastError = "Run a scan or replace the key to move this connection to Runtz configuration checks."
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
	if !s.security.Configured() {
		writeError(w, 503, "Runtz Security is not configured; ask your administrator to configure the service URL and API key")
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
	if !cloudgcp.ValidProject(request.ProjectID) || request.Location != "global" {
		writeError(w, 400, "invalid project ID; configuration checks run at project scope")
		return request, c, false
	}
	return request, c, true
}

func (s *Server) handleListCloudConnections(w http.ResponseWriter, r *http.Request) {
	filter, ok := s.cloudScope(w, r)
	if !ok {
		return
	}
	cursor, err := s.cloudConnections.Find(r.Context(), filter, options.Find().SetProjection(bson.M{"credential": 0}).SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(1000))
	if err != nil {
		writeError(w, 500, "failed to list cloud connections")
		return
	}
	defer closeCursor(r.Context(), cursor)
	connections := []cloudConnection{}
	if err := cursor.All(r.Context(), &connections); err != nil {
		writeError(w, 500, "failed to read cloud connections")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	jobs := make(chan int, len(connections))
	for i := range connections {
		jobs <- i
	}
	close(jobs)
	var workers sync.WaitGroup
	for n := 0; n < min(4, len(connections)); n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				connections[i] = s.refreshCloudConnection(ctx, connections[i])
			}
		}()
	}
	workers.Wait()
	writeJSON(w, 200, map[string]any{"connections": connections, "configured": s.security.Configured()})
}

func (s *Server) handleValidateGCP(w http.ResponseWriter, r *http.Request) {
	request, creds, ok := s.decodeCloudConnect(w, r)
	if !ok {
		return
	}
	defer clear(request.Credentials)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.security.Validate(ctx, request.WorkspaceID, request.ProjectID, request.Credentials); err != nil {
		writeCloudServiceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"valid": true, "projectId": request.ProjectID, "serviceAccount": creds.ClientEmail})
}

func (s *Server) handleConnectGCP(w http.ResponseWriter, r *http.Request) {
	request, creds, ok := s.decodeCloudConnect(w, r)
	if !ok {
		return
	}
	defer clear(request.Credentials)
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 80 {
		writeError(w, 400, "enter a connection name of up to 80 characters")
		return
	}
	user, _ := currentUser(r.Context())
	workspaceID, _ := bson.ObjectIDFromHex(request.WorkspaceID)
	now := time.Now().UTC()
	c := cloudConnection{ID: bson.NewObjectID(), WorkspaceID: workspaceID, Provider: "gcp", Name: request.Name, ProjectID: request.ProjectID, Location: "global", ServiceAccount: creds.ClientEmail, CreatedBy: user.ID, Status: "connecting", CreatedAt: now, UpdatedAt: now}
	if _, err := s.cloudConnections.InsertOne(r.Context(), c); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeError(w, 409, "this project is already connected in this workspace")
		} else {
			writeError(w, 500, "could not reserve cloud connection")
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	if err := s.attachSecurityConnection(ctx, &c, request.Credentials); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.cloudConnections.DeleteOne(cleanup, bson.M{"_id": c.ID, "workspace_id": c.WorkspaceID})
		writeCloudServiceError(w, err)
		return
	}
	c = s.refreshCloudConnection(ctx, c)
	writeJSON(w, 201, map[string]any{"connection": c})
}

func (s *Server) handleGetCloudConnection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, false)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	c = s.refreshCloudConnection(ctx, c)
	writeJSON(w, 200, map[string]any{"connection": c})
}

func (s *Server) handleLegacyCloudFindings(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	if c.SecurityConnectionID == "" {
		raw, err := openCloudCredential(s.cfg.CloudEncryptionKey, c)
		if err != nil {
			writeError(w, 422, "replace the service account JSON to reconnect this project")
			return
		}
		defer clear(raw)
		if err := s.attachSecurityConnection(ctx, &c, raw); err != nil {
			writeCloudServiceError(w, err)
			return
		}
	} else {
		job, err := s.security.Enqueue(ctx, c.WorkspaceID.Hex(), c.SecurityConnectionID)
		if err != nil {
			writeCloudServiceError(w, err)
			return
		}
		c.Status = "syncing"
		c.LastError = ""
		c.RemoteAttemptID = job.ID
		_, _ = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID, "workspace_id": c.WorkspaceID}, bson.M{"$set": bson.M{"status": c.Status, "last_error": "", "remote_attempt_id": job.ID, "updated_at": time.Now().UTC()}})
	}
	writeJSON(w, 202, map[string]any{"connection": c})
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
	defer clear(request.Credentials)
	creds, err := cloudgcp.ParseCredentials(request.Credentials)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	if c.SecurityConnectionID == "" {
		err = s.attachSecurityConnection(ctx, &c, request.Credentials)
	} else {
		err = s.security.Rotate(ctx, c.WorkspaceID.Hex(), c.SecurityConnectionID, request.Credentials)
	}
	if err != nil {
		writeCloudServiceError(w, err)
		return
	}
	_, err = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID, "workspace_id": c.WorkspaceID}, bson.M{"$set": bson.M{"service_account": creds.ClientEmail, "updated_at": time.Now().UTC()}, "$unset": bson.M{"credential": ""}})
	if err != nil {
		writeError(w, 500, "key replaced in Security; could not update connection metadata")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteCloudConnection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, true)
	if !ok {
		return
	}
	if c.SecurityConnectionID != "" {
		if err := s.security.Delete(r.Context(), c.WorkspaceID.Hex(), c.SecurityConnectionID); err != nil {
			writeCloudServiceError(w, err)
			return
		}
	}
	if _, err := s.cloudConnections.DeleteOne(r.Context(), bson.M{"_id": c.ID, "workspace_id": c.WorkspaceID}); err != nil {
		writeError(w, 500, "could not remove local cloud reference; retry disconnecting")
		return
	}
	if _, err := s.cloudFindings.DeleteMany(r.Context(), bson.M{"connection_id": c.ID, "workspace_id": c.WorkspaceID}); err != nil {
		writeError(w, 500, "cloud disconnected, but previous findings cleanup failed")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "disconnected"})
}

func writeCloudServiceError(w http.ResponseWriter, err error) {
	var serviceErr *securityclient.Error
	if errors.As(err, &serviceErr) {
		status := serviceErr.Status
		if status == 401 || status == 403 {
			status = 503
		}
		if status == 404 {
			status = 422
		}
		if status < 400 || status > 599 {
			status = 502
		}
		writeError(w, status, serviceErr.Message)
		return
	}
	writeError(w, 502, "cloud service operation failed; previous results were preserved")
}

func (s *Server) attachSecurityConnection(ctx context.Context, c *cloudConnection, raw []byte) error {
	created, err := s.security.Create(ctx, c.WorkspaceID.Hex(), c.Name, c.ProjectID, json.RawMessage(raw))
	if err != nil {
		return err
	}
	c.SecurityConnectionID = created.Connection.ID
	c.ServiceAccount = created.Connection.ServiceAccount
	c.RemoteAttemptID = created.Scan.ID
	c.Status = "syncing"
	c.LastError = ""
	c.Credential = ""
	result, err := s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID, "workspace_id": c.WorkspaceID}, bson.M{"$set": bson.M{"security_connection_id": c.SecurityConnectionID, "service_account": c.ServiceAccount, "remote_attempt_id": c.RemoteAttemptID, "status": c.Status, "last_error": "", "updated_at": time.Now().UTC()}, "$unset": bson.M{"credential": "", "scan_lock_id": "", "scan_lock_until": ""}})
	count, workspaceErr := s.workspaces.CountDocuments(ctx, bson.M{"_id": c.WorkspaceID, "deleting": bson.M{"$ne": true}})
	if err != nil || result == nil || result.MatchedCount == 0 || workspaceErr != nil || count == 0 {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.security.Delete(cleanup, c.WorkspaceID.Hex(), c.SecurityConnectionID)
		_, _ = s.cloudConnections.DeleteOne(cleanup, bson.M{"_id": c.ID, "workspace_id": c.WorkspaceID})
		return &securityclient.Error{Status: 409, Message: "workspace or connection was removed while connecting; retry from an active workspace"}
	}
	return nil
}

func (s *Server) refreshCloudConnection(ctx context.Context, c cloudConnection) cloudConnection {
	c = cloudConnectionView(c)
	if c.SecurityConnectionID == "" {
		return c
	}
	previous := c
	unavailable := func(err error) cloudConnection { c.Status = "unavailable"; c.LastError = err.Error(); return c }
	remote, err := s.security.Connection(ctx, c.WorkspaceID.Hex(), c.SecurityConnectionID)
	if err != nil {
		return unavailable(err)
	}
	if remote.ProjectID != c.ProjectID {
		return unavailable(errors.New("cloud project reference does not match this connection"))
	}
	publish := func(job securityclient.Job) bool {
		if job.ConnectionID != c.SecurityConnectionID || job.Result == nil || job.Result.ProjectID != c.ProjectID || job.Result.Provider != "gcp" {
			return false
		}
		coverage := job.Result.Summary
		c.Coverage = &coverage
		c.Summary = cloudCounts{Total: coverage.Critical + coverage.High + coverage.Medium + coverage.Low, Critical: coverage.Critical, High: coverage.High, Medium: coverage.Medium, Low: coverage.Low}
		finished := job.Result.FinishedAt
		c.LastScanAt = &finished
		c.RemoteScanID = job.ID
		return true
	}
	if remote.LastAttemptID != "" && (remote.LastAttemptID != c.RemoteAttemptID || c.Status == "syncing" || c.Status == "connecting" || c.Status == "unavailable") {
		job, err := s.security.Job(ctx, c.WorkspaceID.Hex(), remote.LastAttemptID)
		if err != nil {
			return unavailable(err)
		}
		if job.ConnectionID != c.SecurityConnectionID {
			return unavailable(errors.New("cloud scan reference does not match this connection"))
		}
		c.RemoteAttemptID = job.ID
		c.LastError = ""
		switch job.State {
		case "queued", "running":
			c.Status = "syncing"
		case "completed":
			c.Status = "connected"
		case "partial":
			c.Status = "partial"
		case "failed":
			c.Status = "error"
			c.LastError = job.Error
		default:
			return unavailable(errors.New("cloud service returned an unknown scan state"))
		}
		if job.Result != nil && !publish(job) {
			return unavailable(errors.New("cloud result does not match this project"))
		}
	}
	if remote.LastScanID != "" && (c.RemoteScanID != remote.LastScanID || c.Coverage == nil) {
		job, err := s.security.Job(ctx, c.WorkspaceID.Hex(), remote.LastScanID)
		if err != nil {
			return unavailable(err)
		}
		if !publish(job) {
			return unavailable(errors.New("cloud result does not match this project"))
		}
	}
	if c.Status != previous.Status || c.LastError != previous.LastError || c.RemoteScanID != previous.RemoteScanID || c.RemoteAttemptID != previous.RemoteAttemptID {
		c.UpdatedAt = time.Now().UTC()
		_, _ = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID, "workspace_id": c.WorkspaceID, "security_connection_id": c.SecurityConnectionID}, bson.M{"$set": bson.M{"status": c.Status, "last_error": c.LastError, "remote_attempt_id": c.RemoteAttemptID, "remote_scan_id": c.RemoteScanID, "coverage": c.Coverage, "summary": c.Summary, "last_scan_at": c.LastScanAt, "updated_at": c.UpdatedAt}})
	}
	return c
}

func cloudResultPage(r *http.Request) (page int, search string, err error) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		page = 10000
	}
	search = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	if len(search) > 200 {
		err = errors.New("search must be 200 characters or fewer")
	}
	return
}
func (s *Server) handleCloudFindings(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, false)
	if !ok {
		return
	}
	if c.SecurityConnectionID == "" {
		s.handleLegacyCloudFindings(w, r)
		return
	}
	page, search, err := cloudResultPage(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	job, err := s.security.Latest(r.Context(), c.WorkspaceID.Hex(), c.SecurityConnectionID)
	if securityclient.IsStatus(err, 404) {
		if !c.SnapshotID.IsZero() {
			s.handleLegacyCloudFindings(w, r)
			return
		}
		writeJSON(w, 200, map[string]any{"findings": []cloudgcp.Finding{}, "total": 0, "page": page, "pageSize": 50, "resultAvailable": false})
		return
	}
	if err != nil {
		writeCloudServiceError(w, err)
		return
	}
	if job.Result.ProjectID != c.ProjectID {
		writeError(w, 502, "cloud result does not match this project")
		return
	}
	findings := []cloudgcp.Finding{}
	severity := r.URL.Query().Get("severity")
	for _, f := range job.Result.Findings {
		if severity != "" && severity != "all" && f.Severity != severity {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(f.Title+" "+f.ResourceName+" "+f.Source+" "+f.RuleID+" "+f.Evidence), search) {
			continue
		}
		findings = append(findings, cloudgcp.Finding{ID: f.ID, RuleID: f.RuleID, Evidence: f.Evidence, Title: f.Title, Category: f.Category, Description: f.Description, Severity: f.Severity, State: "ACTIVE", Mute: "UNMUTED", ResourceName: f.ResourceName, ProjectName: f.ProjectID, Source: f.Source, ExternalURI: f.Documentation, EventTime: job.Result.FinishedAt.Format(time.RFC3339), Remediation: f.Remediation})
	}
	rank := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "unknown": 4}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return rank[findings[i].Severity] < rank[findings[j].Severity]
		}
		return findings[i].ID < findings[j].ID
	})
	total := len(findings)
	start := min((page-1)*50, total)
	end := min(start+50, total)
	writeJSON(w, 200, map[string]any{"findings": findings[start:end], "total": total, "page": page, "pageSize": 50, "resultAvailable": true, "coverage": job.Result.Summary, "scanId": job.ID, "scannedAt": job.Result.FinishedAt})
}
func (s *Server) handleCloudChecks(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cloudFromRequest(w, r, false)
	if !ok {
		return
	}
	page, search, err := cloudResultPage(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if c.SecurityConnectionID == "" {
		writeJSON(w, 200, map[string]any{"checks": []securityclient.Check{}, "total": 0, "page": page, "pageSize": 50, "resultAvailable": false})
		return
	}
	job, err := s.security.Latest(r.Context(), c.WorkspaceID.Hex(), c.SecurityConnectionID)
	if securityclient.IsStatus(err, 404) {
		writeJSON(w, 200, map[string]any{"checks": []securityclient.Check{}, "total": 0, "page": page, "pageSize": 50, "resultAvailable": false})
		return
	}
	if err != nil {
		writeCloudServiceError(w, err)
		return
	}
	if job.Result.ProjectID != c.ProjectID {
		writeError(w, 502, "cloud result does not match this project")
		return
	}
	checks := []securityclient.Check{}
	status := r.URL.Query().Get("status")
	for _, check := range job.Result.Checks {
		if status != "" && status != "all" && check.Status != status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(check.Title+" "+check.RuleID+" "+check.Resource+" "+check.Evidence), search) {
			continue
		}
		checks = append(checks, check)
	}
	total := len(checks)
	start := min((page-1)*50, total)
	end := min(start+50, total)
	writeJSON(w, 200, map[string]any{"checks": checks[start:end], "total": total, "page": page, "pageSize": 50, "resultAvailable": true, "coverage": job.Result.Summary, "scanId": job.ID, "scannedAt": job.Result.FinishedAt})
}
