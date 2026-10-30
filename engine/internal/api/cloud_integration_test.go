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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/cloudgcp"
	"github.com/runtz-dev/runtz/engine/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestGCPConnectionLifecycleAndWorkspaceIsolation(t *testing.T) {
	uri := os.Getenv("RUNTZ_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set RUNTZ_TEST_MONGO_URI for MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s, err := New(ctx, config.Config{DeploymentMode: hostingCloud, MongoURI: uri, MongoDatabase: "runtz_cloud_test_" + bson.NewObjectID().Hex(), CloudEncryptionKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))})
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
	creds := cloudgcp.Credentials{Type: "service_account", ProjectID: "test-project", ClientEmail: "runtz@test-project.iam.gserviceaccount.com", PrivateKeyID: "ephemeral-test-key", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), TokenURI: "https://oauth2.googleapis.com/token"}
	credentials, _ := json.Marshal(creds)
	findings := []cloudgcp.Finding{{ID: "one", Title: "Public bucket", State: "ACTIVE", Severity: "high"}, {ID: "two", Title: "Audit logging", State: "ACTIVE", Severity: "medium"}, {ID: "three", State: "ACTIVE", Severity: "unknown"}, {ID: "four", State: "INACTIVE", Severity: "critical"}, {ID: "five", State: "ACTIVE", Mute: "MUTED", Severity: "low"}}
	var providerErr error
	s.readGCP = func(_ context.Context, _ []byte, project, location string, _ bool) ([]cloudgcp.Finding, error) {
		if project == "" || location != "global" {
			t.Fatal("missing target scope")
		}
		return findings, providerErr
	}
	request := func(who, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: tokens[who]})
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("HTTP %d, expected %d: %s", w.Code, status, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "PRIVATE KEY") {
			t.Fatal("private credential leaked in response")
		}
	}
	body := cloudConnectRequest{WorkspaceID: workspace.ID.Hex(), Name: "Production", ProjectID: "test-project", Location: "global", Credentials: credentials}
	expect(request("member", "POST", "/api/v1/cloud/gcp/validate", body), 403)
	expect(request("other", "POST", "/api/v1/cloud/gcp/validate", body), 403)
	expect(request("owner", "POST", "/api/v1/cloud/gcp/validate", body), 200)
	w := request("owner", "POST", "/api/v1/cloud/gcp/connections", body)
	expect(w, 201)
	var created struct {
		Connection cloudConnection `json:"connection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	c := created.Connection
	if c.Summary.Total != 3 || c.Summary.High != 1 || c.Summary.Medium != 1 || c.Summary.Critical != 0 {
		t.Fatalf("wrong summary: %+v", c.Summary)
	}
	path := "/api/v1/cloud/connections/" + c.ID.Hex()
	var stored cloudConnection
	if err := s.cloudConnections.FindOne(ctx, bson.M{"_id": c.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Credential == "" || strings.Contains(stored.Credential, "PRIVATE KEY") {
		t.Fatal("credential not encrypted")
	}
	plain, err := openCloudCredential(s.cfg.CloudEncryptionKey, stored)
	if err != nil || !bytes.Equal(plain, credentials) {
		t.Fatal("stored key not recoverable")
	}
	expect(request("owner", "POST", "/api/v1/cloud/gcp/connections", body), 409)
	for _, who := range []string{"owner", "member"} {
		expect(request(who, "GET", path, nil), 200)
		w = request(who, "GET", path+"/findings", nil)
		expect(w, 200)
		if !strings.Contains(w.Body.String(), `"total":3`) {
			t.Fatal("active filtering failed")
		}
	}
	w = request("owner", "GET", path+"/findings?state=all", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":5`) {
		t.Fatal("all states missing findings")
	}
	w = request("owner", "GET", path+"/findings?severity=high", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatal("severity filtering failed")
	}
	w = request("owner", "GET", path+"/findings?search=.*", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal("search treated as regex")
	}
	for _, suffix := range []string{"", "/findings"} {
		expect(request("other", "GET", path+suffix, nil), 404)
	}
	w = request("other", "GET", "/api/v1/cloud/connections", nil)
	expect(w, 200)
	if strings.Contains(w.Body.String(), c.ID.Hex()) {
		t.Fatal("cross-workspace connection disclosure")
	}
	for _, who := range []string{"other", "member"} {
		status := 403
		if who == "other" {
			status = 404
		}
		expect(request(who, "POST", path+"/scan", nil), status)
		expect(request(who, "PATCH", path+"/credentials", map[string]any{"credentials": creds}), status)
		expect(request(who, "DELETE", path, nil), status)
	}
	providerErr = errors.New("GCP denied access")
	expect(request("owner", "POST", path+"/scan", nil), 422)
	w = request("owner", "GET", path+"/findings", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":3`) {
		t.Fatal("failed scan removed previous findings")
	}
	providerErr = nil
	findings = []cloudgcp.Finding{{ID: "new", Title: "New finding", State: "ACTIVE", Severity: "critical"}}
	expect(request("owner", "POST", path+"/scan", nil), 200)
	expect(request("owner", "POST", path+"/scan", nil), 200)
	w = request("owner", "GET", path+"/findings", nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"total":1`) || strings.Contains(w.Body.String(), "Public bucket") {
		t.Fatal("snapshot replacement failed")
	}
	count, err := s.cloudFindings.CountDocuments(ctx, bson.M{"connection_id": c.ID, "expires_at": bson.M{"$exists": false}})
	if err != nil || count != 1 {
		t.Fatal("more than one current snapshot retained")
	}
	_, _ = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID}, bson.M{"$set": bson.M{"scan_lock_until": time.Now().Add(time.Minute)}})
	expect(request("owner", "POST", path+"/scan", nil), 409)
	expect(request("owner", "DELETE", path, nil), 409)
	_, _ = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID}, bson.M{"$unset": bson.M{"scan_lock_until": ""}})
	_, _ = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": c.ID}, bson.M{"$set": bson.M{"status": "syncing", "scan_lock_until": time.Now().Add(-time.Minute)}})
	w = request("owner", "GET", path, nil)
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"status":"error"`) {
		t.Fatal("expired import lease left the UI stuck syncing")
	}
	expect(request("owner", "POST", path+"/scan", nil), 200)
	expect(request("owner", "PATCH", path+"/credentials", map[string]any{"credentials": creds}), 200)
	expect(request("owner", "DELETE", path, nil), 200)
	for _, collection := range []string{"cloud_connections", "cloud_findings"} {
		count, err := s.db.Collection(collection).CountDocuments(ctx, bson.M{})
		if err != nil || count != 0 {
			t.Fatalf("%s not cleaned", collection)
		}
	}
	// More than one target can use the same service account in a workspace.
	for _, project := range []string{"project-one", "project-two"} {
		body.ProjectID = project
		expect(request("owner", "POST", "/api/v1/cloud/gcp/connections", body), 201)
	}
	// Deletion during an in-flight initial import must not resurrect credentials.
	s.readGCP = func(readCtx context.Context, _ []byte, _ string, _ string, _ bool) ([]cloudgcp.Finding, error) {
		if err := s.purgeWorkspaces(readCtx, []bson.ObjectID{workspace.ID}); err != nil {
			t.Fatal(err)
		}
		return findings, nil
	}
	body.ProjectID = "project-last"
	expect(request("owner", "POST", "/api/v1/cloud/gcp/connections", body), 409)
	for _, collection := range []string{"cloud_connections", "cloud_findings"} {
		count, err := s.db.Collection(collection).CountDocuments(ctx, bson.M{})
		if err != nil || count != 0 {
			t.Fatalf("%s retained after workspace purge", collection)
		}
	}
}
