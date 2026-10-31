package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/cloudgcp"
	"github.com/runtz-dev/runtz/engine/internal/config"
	"github.com/runtz-dev/runtz/engine/internal/securityclient"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type securityFixture struct {
	mu          sync.Mutex
	connections map[string]securityclient.Connection
	jobs        map[string]securityclient.Job
	failScan    bool
	afterCreate func()
	tenants     []string
}

func (f *securityFixture) queue(c securityclient.Connection) securityclient.Job {
	if job, ok := f.jobs[c.LastAttemptID]; ok && (job.State == "queued" || job.State == "running") {
		return job
	}
	job := securityclient.Job{ID: bson.NewObjectID().Hex(), TenantID: c.TenantID, ConnectionID: c.ID, State: "queued", CreatedAt: time.Now().UTC()}
	c.LastAttemptID = job.ID
	f.connections[c.ID] = c
	f.jobs[job.ID] = job
	return job
}
func (f *securityFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := r.Header.Get("X-Runtz-Tenant-ID")
	f.tenants = append(f.tenants, tenant)
	respond := func(status int, body any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 40) {
		respond(401, map[string]string{"error": "service auth missing"})
		return
	}
	if r.URL.Path == "/v1/gcp/validate" {
		var body struct {
			ProjectID string `json:"projectId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		respond(200, map[string]any{"valid": true, "project": map[string]string{"id": body.ProjectID}})
		return
	}
	if r.URL.Path == "/v1/connections" {
		if r.Method == "GET" {
			rows := []securityclient.Connection{}
			for _, c := range f.connections {
				if c.TenantID == tenant {
					rows = append(rows, c)
				}
			}
			respond(200, map[string]any{"connections": rows})
			return
		}
		var body struct {
			Name        string               `json:"name"`
			ProjectID   string               `json:"projectId"`
			Credentials cloudgcp.Credentials `json:"credentials"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, c := range f.connections {
			if c.TenantID == tenant && c.ProjectID == body.ProjectID {
				respond(409, map[string]string{"error": "already connected"})
				return
			}
		}
		c := securityclient.Connection{ID: bson.NewObjectID().Hex(), TenantID: tenant, Provider: "gcp", Name: body.Name, ProjectID: body.ProjectID, ServiceAccount: body.Credentials.ClientEmail}
		f.connections[c.ID] = c
		job := f.queue(c)
		c = f.connections[c.ID]
		if hook := f.afterCreate; hook != nil {
			f.afterCreate = nil
			f.mu.Unlock()
			hook()
			f.mu.Lock()
		}
		respond(201, securityclient.CreateResponse{Connection: c, Scan: job})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v1/scans/") {
		job, ok := f.jobs[strings.TrimPrefix(r.URL.Path, "/v1/scans/")]
		if !ok || job.TenantID != tenant {
			respond(404, map[string]string{"error": "not found"})
			return
		}
		respond(200, map[string]any{"scan": job})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/connections/"), "/")
	c, ok := f.connections[parts[0]]
	if !ok || c.TenantID != tenant {
		respond(404, map[string]string{"error": "not found"})
		return
	}
	if len(parts) == 1 {
		if r.Method == "DELETE" {
			delete(f.connections, c.ID)
			for id, j := range f.jobs {
				if j.ConnectionID == c.ID {
					delete(f.jobs, id)
				}
			}
			respond(200, map[string]string{"status": "disconnected"})
			return
		}
		respond(200, map[string]any{"connection": c})
		return
	}
	switch parts[1] {
	case "scans":
		if f.failScan {
			respond(503, map[string]string{"error": "scan queue unavailable"})
			return
		}
		respond(202, map[string]any{"scan": f.queue(c)})
	case "results":
		job, ok := f.jobs[c.LastScanID]
		if !ok {
			respond(404, map[string]string{"error": "no completed scan"})
			return
		}
		respond(200, map[string]any{"scan": job})
	case "credentials":
		respond(200, map[string]string{"status": "updated"})
	default:
		respond(404, map[string]string{"error": "not found"})
	}
}
func (f *securityFixture) publish(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.connections[id]
	job := f.jobs[c.LastAttemptID]
	job.State = "partial"
	now := time.Now().UTC()
	job.FinishedAt = &now
	job.Result = &securityclient.Result{Provider: "gcp", ProjectID: c.ProjectID, FinishedAt: now, Summary: securityclient.Summary{Passed: 1, Failed: 2, Skipped: 1, High: 1, Medium: 1}, Findings: []securityclient.Finding{{ID: "one", RuleID: "RZ-GCP-027", Title: "Public bucket", Severity: "high", ProjectID: c.ProjectID, Source: "Runtz GCP configuration checks", Evidence: "public grant"}, {ID: "two", RuleID: "RZ-GCP-003", Title: "DNSSEC disabled", Severity: "medium", ProjectID: c.ProjectID}}, Checks: []securityclient.Check{{RuleID: "RZ-GCP-027", Status: "fail", Title: "Public bucket"}, {RuleID: "RZ-GCP-006", Status: "skipped", Title: "Read permission missing", Evidence: "compute.instances.list"}}}
	f.jobs[job.ID] = job
	c.LastScanID = job.ID
	f.connections[c.ID] = c
}

func TestGCPConnectionLifecycleAndWorkspaceIsolation(t *testing.T) {
	uri := os.Getenv("RUNTZ_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set RUNTZ_TEST_MONGO_URI for MongoDB integration tests")
	}
	fixture := &securityFixture{connections: map[string]securityclient.Connection{}, jobs: map[string]securityclient.Job{}}
	remote := httptest.NewServer(fixture)
	defer remote.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s, err := New(ctx, config.Config{DeploymentMode: hostingCloud, MongoURI: uri, MongoDatabase: "runtz_cloud_test_" + bson.NewObjectID().Hex(), CloudEncryptionKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)), SecurityAPIURL: remote.URL, SecurityAPIKey: strings.Repeat("s", 40)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	defer s.db.Drop(context.Background())
	owner, workspaces, err := s.findOrCreateEmailUser(ctx, "cloud-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	workspace := workspaces[0]
	other, _, err := s.findOrCreateEmailUser(ctx, "cloud-other@example.com")
	if err != nil {
		t.Fatal(err)
	}
	member, _, err := s.findOrCreateEmailUser(ctx, "cloud-member@example.com")
	if err != nil {
		t.Fatal(err)
	}
	member.WorkspaceIDs = append(member.WorkspaceIDs, workspace.ID)
	if _, err := s.users.UpdateOne(ctx, bson.M{"_id": member.ID}, bson.M{"$set": bson.M{"workspace_ids": member.WorkspaceIDs}}); err != nil {
		t.Fatal(err)
	}
	tokens := map[string]string{}
	for label, user := range map[string]User{"owner": owner, "other": other, "member": member} {
		token, err := s.issueSession(ctx, user, httptest.NewRequest("GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		tokens[label] = token
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	creds := cloudgcp.Credentials{Type: "service_account", ProjectID: "test-project", ClientEmail: "scanner@test-project.iam.gserviceaccount.com", PrivateKeyID: "ephemeral-test-key", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), TokenURI: "https://oauth2.googleapis.com/token"}
	credential, _ := json.Marshal(creds)
	request := func(who, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw)).WithContext(ctx)
		r.Header.Set("X-Runtz-Tenant-ID", other.WorkspaceIDs[0].Hex()) // must never be forwarded
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: tokens[who]})
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "PRIVATE KEY") || strings.Contains(w.Body.String(), s.cfg.SecurityAPIKey) {
			t.Fatal("credential leaked")
		}
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("HTTP %d expected %d: %s", w.Code, status, w.Body.String())
		}
	}
	body := cloudConnectRequest{WorkspaceID: workspace.ID.Hex(), Name: "Production", ProjectID: "test-project", Credentials: credential}
	expect(request("member", "POST", "/api/v1/cloud/gcp/validate", body), 403)
	expect(request("other", "POST", "/api/v1/cloud/gcp/validate", body), 403)
	expect(request("owner", "POST", "/api/v1/cloud/gcp/validate", body), 200)
	w := request("owner", "POST", "/api/v1/cloud/gcp/connections", body)
	expect(w, 201)
	var created struct {
		Connection cloudConnection `json:"connection"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	c := created.Connection
	path := "/api/v1/cloud/connections/" + c.ID.Hex()
	if c.Status != "syncing" || c.Coverage != nil {
		t.Fatal("initial scan must be asynchronous and not reported as compliant", c)
	}
	var stored cloudConnection
	if err := s.cloudConnections.FindOne(ctx, bson.M{"_id": c.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Credential != "" || stored.SecurityConnectionID == "" {
		t.Fatal("engine must store a reference, not duplicate customer keys")
	}
	expect(request("owner", "POST", "/api/v1/cloud/gcp/connections", body), 409)
	w = request("owner", "GET", path+"/findings", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"resultAvailable":false`) {
		t.Fatal("pending result was reported as complete")
	}
	fixture.publish(stored.SecurityConnectionID)
	w = request("owner", "GET", path, nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"status":"partial"`) || !strings.Contains(w.Body.String(), `"skipped":1`) {
		t.Fatal("incomplete coverage not preserved", w.Body.String())
	}
	for _, who := range []string{"owner", "member"} {
		w = request(who, "GET", path+"/findings", nil)
		expect(w, 200)
		if !strings.Contains(w.Body.String(), `"total":2`) {
			t.Fatal("custom findings missing")
		}
	}
	w = request("owner", "GET", path+"/checks?status=skipped", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":1`) || !strings.Contains(w.Body.String(), "compute.instances.list") {
		t.Fatal("coverage evidence missing")
	}
	w = request("owner", "GET", path+"/findings?severity=high", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatal("severity filter failed")
	}
	w = request("owner", "GET", path+"/findings?search=.*", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal("search evaluated as regex")
	}
	for _, suffix := range []string{"", "/findings", "/checks"} {
		expect(request("other", "GET", path+suffix, nil), 404)
	}
	for _, who := range []string{"member", "other"} {
		status := 403
		if who == "other" {
			status = 404
		}
		expect(request(who, "POST", path+"/scan", nil), status)
		expect(request(who, "PATCH", path+"/credentials", map[string]any{"credentials": creds}), status)
		expect(request(who, "DELETE", path, nil), status)
	}
	fixture.mu.Lock()
	fixture.failScan = true
	fixture.mu.Unlock()
	expect(request("owner", "POST", path+"/scan", nil), 503)
	w = request("owner", "GET", path+"/findings", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":2`) {
		t.Fatal("failed scan lost last valid findings")
	}
	fixture.mu.Lock()
	fixture.failScan = false
	fixture.mu.Unlock()
	expect(request("owner", "POST", path+"/scan", nil), 202)
	expect(request("owner", "POST", path+"/scan", nil), 202)
	expect(request("owner", "PATCH", path+"/credentials", map[string]any{"credentials": creds}), 200)
	// Existing encrypted engine credentials migrate on an owner-initiated scan.
	sealed, err := sealCloudCredential(s.cfg.CloudEncryptionKey, stored, credential)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": stored.ID}, bson.M{"$set": bson.M{"credential": sealed}, "$unset": bson.M{"security_connection_id": ""}})
	if err != nil {
		t.Fatal(err)
	}
	expect(request("owner", "POST", path+"/scan", nil), 202)
	expect(request("owner", "DELETE", path, nil), 200)
	fixture.mu.Lock()
	remaining := len(fixture.connections)
	fixture.mu.Unlock()
	if remaining != 0 {
		t.Fatal("disconnect left credentials in Security")
	}
	for _, project := range []string{"project-one", "project-two"} {
		body.ProjectID = project
		expect(request("owner", "POST", "/api/v1/cloud/gcp/connections", body), 201)
	}
	fixture.mu.Lock()
	fixture.afterCreate = func() {
		if err := s.purgeWorkspaces(ctx, []bson.ObjectID{workspace.ID}); err != nil {
			t.Error(err)
		}
	}
	fixture.mu.Unlock()
	body.ProjectID = "project-last"
	expect(request("owner", "POST", "/api/v1/cloud/gcp/connections", body), 409)
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if len(fixture.connections) != 0 {
		t.Fatal("workspace deletion left remote credentials or resurrected a connection")
	}
	for _, forwarded := range fixture.tenants {
		if forwarded != workspace.ID.Hex() {
			t.Fatal("engine forwarded caller-supplied tenant header")
		}
	}
	count, err := s.cloudConnections.CountDocuments(ctx, bson.M{})
	if err != nil || count != 0 {
		t.Fatal("workspace cleanup left cloud references")
	}
}
