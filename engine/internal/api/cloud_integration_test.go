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
	"fmt"
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
	if r.URL.Path == "/v1/aws/onboarding" {
		var body struct {
			AccountID string `json:"accountId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		respond(200, securityclient.AWSSetup{AccountID: body.AccountID, RoleARN: "arn:aws:iam::" + body.AccountID + ":role/RuntzTest", PrincipalARN: "arn:aws:iam::123456789013:role/RuntzScanner", ExternalID: "runtz-" + strings.Repeat("a", 64), Template: json.RawMessage(`{"Resources":{}}`)})
		return
	}
	if r.URL.Path == "/v1/aws/validate" {
		var body struct {
			AccountID string `json:"accountId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		respond(200, map[string]any{"valid": true, "accountId": body.AccountID})
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
	if r.URL.Path == "/v1/azure/validate" {
		var body struct {
			SubscriptionID string `json:"subscriptionId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		respond(200, map[string]any{"valid": true, "subscription": map[string]string{"id": body.SubscriptionID}})
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
			AccountID      string `json:"accountId"`
			Name           string `json:"name"`
			Provider       string `json:"provider"`
			ProjectID      string `json:"projectId"`
			SubscriptionID string `json:"subscriptionId"`
			Credentials    struct {
				RoleARN     string   `json:"roleArn"`
				Regions     []string `json:"regions"`
				ClientEmail string   `json:"client_email"`
				TenantID    string   `json:"tenantId"`
				ClientID    string   `json:"clientId"`
			} `json:"credentials"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Provider == "" {
			body.Provider = "gcp"
		}
		if body.Provider == "aws" {
			body.ProjectID = body.AccountID
		}
		if body.Provider == "azure" {
			body.ProjectID = body.SubscriptionID
		}
		for _, c := range f.connections {
			if c.TenantID == tenant && c.Provider == body.Provider && c.ProjectID == body.ProjectID {
				respond(409, map[string]string{"error": "already connected"})
				return
			}
		}
		c := securityclient.Connection{AccountID: body.AccountID, RoleARN: body.Credentials.RoleARN, Regions: body.Credentials.Regions, ID: bson.NewObjectID().Hex(), TenantID: tenant, Provider: body.Provider, Name: body.Name, ProjectID: body.ProjectID, SubscriptionID: body.SubscriptionID, DirectoryTenantID: body.Credentials.TenantID, ClientID: body.Credentials.ClientID, ServiceAccount: body.Credentials.ClientEmail}
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
	if c.Provider == "azure" {
		job.Result.Provider, job.Result.ProjectID, job.Result.SubscriptionID = "azure", "", c.SubscriptionID
		job.Result.Findings = []securityclient.Finding{{ID: "one", RuleID: "RZ-AZURE-003", Title: "Storage anonymous access capability", Severity: "high", SubscriptionID: c.SubscriptionID, Source: "Runtz Azure configuration checks", Evidence: "anonymous access allowed"}, {ID: "two", RuleID: "RZ-AZURE-009", Title: "SQL public access", Severity: "medium", SubscriptionID: c.SubscriptionID}}
		job.Result.Checks = []securityclient.Check{{RuleID: "RZ-AZURE-003", Status: "fail", Title: "Storage anonymous access capability"}, {RuleID: "RZ-AZURE-011", Status: "skipped", Title: "Read permission missing", Evidence: "Microsoft.ContainerService/managedClusters/read"}}
	}
	if c.Provider == "aws" {
		job.Result.Provider = "aws"
		job.Result.ProjectID = ""
		job.Result.AccountID = c.AccountID
		job.Result.Regions = c.Regions
		job.Result.Findings = []securityclient.Finding{{ID: "one", RuleID: "RZ-AWS-007", Title: "SSH rule", Severity: "high", AccountID: c.AccountID, Region: "us-east-1"}, {ID: "two", RuleID: "RZ-AWS-011", Title: "EBS encryption", Severity: "medium", AccountID: c.AccountID, Region: "us-east-1"}}
		job.Result.Checks = []securityclient.Check{{RuleID: "RZ-AWS-007", Status: "fail", Title: "SSH rule", Region: "us-east-1"}, {RuleID: "RZ-AWS-012", Status: "skipped", Title: "Read permission missing", Evidence: "rds:DescribeDBInstances", Region: "us-east-1"}}
	}
	f.jobs[job.ID] = job
	c.LastScanID = job.ID
	f.connections[c.ID] = c
}

func TestCloudConnectionLifecycleAndWorkspaceIsolation(t *testing.T) {
	for _, provider := range []string{"gcp", "azure", "aws"} {
		t.Run(provider, func(t *testing.T) { testCloudConnectionLifecycle(t, provider) })
	}
}

func testCloudConnectionLifecycle(t *testing.T, provider string) {
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
	var secret string
	if provider == "aws" {
		credential, _ = json.Marshal(securityclient.AWSCredentials{RoleARN: "arn:aws:iam::123456789012:role/RuntzTest", ExternalID: "runtz-" + strings.Repeat("a", 64), Regions: []string{"us-east-1", "sa-east-1"}})
	}
	if provider == "azure" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			t.Fatal(err)
		}
		secret = base64.RawURLEncoding.EncodeToString(raw)
		credential, _ = json.Marshal(securityclient.AzureCredentials{TenantID: "22222222-2222-4222-8222-222222222222", ClientID: "33333333-3333-4333-8333-333333333333", ClientSecret: secret})
	}
	request := func(who, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw)).WithContext(ctx)
		r.Header.Set("X-Runtz-Tenant-ID", other.WorkspaceIDs[0].Hex()) // must never be forwarded
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: tokens[who]})
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "PRIVATE KEY") || strings.Contains(w.Body.String(), s.cfg.SecurityAPIKey) || (secret != "" && strings.Contains(w.Body.String(), secret)) {
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
	base := "/api/v1/cloud/" + provider
	if provider == "aws" {
		body.ProjectID = ""
		body.AccountID = "123456789012"
		expect(request("member", "POST", base+"/onboarding", body), 403)
		expect(request("other", "POST", base+"/onboarding", body), 403)
		setup := request("owner", "POST", base+"/onboarding", body)
		expect(setup, 200)
		if !strings.Contains(setup.Body.String(), "runtz-") {
			t.Fatal("AWS onboarding missing external ID")
		}
	}
	if provider == "azure" {
		body.ProjectID, body.SubscriptionID = "", "11111111-1111-4111-8111-111111111111"
	}
	expect(request("member", "POST", base+"/validate", body), 403)
	expect(request("other", "POST", base+"/validate", body), 403)
	expect(request("owner", "POST", base+"/validate", body), 200)
	w := request("owner", "POST", base+"/connections", body)
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
	if provider == "azure" && (stored.SubscriptionID != body.SubscriptionID || stored.ClientID == "" || stored.DirectoryTenantID == "") {
		t.Fatal("Azure scope or display identity missing")
	}
	expect(request("owner", "POST", base+"/connections", body), 409)
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
	permission := "compute.instances.list"
	if provider == "aws" {
		permission = "rds:DescribeDBInstances"
	}
	if provider == "azure" {
		permission = "Microsoft.ContainerService/managedClusters/read"
	}
	if !strings.Contains(w.Body.String(), `"total":1`) || !strings.Contains(w.Body.String(), permission) {
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
	if provider == "azure" || provider == "aws" {
		fixture.mu.Lock()
		remote := fixture.connections[stored.SecurityConnectionID]
		job := fixture.jobs[remote.LastScanID]
		original := job.Result
		wrong := *original
		wrong.SubscriptionID = "99999999-9999-4999-8999-999999999999"
		if provider == "aws" {
			wrong.AccountID = "999999999999"
		}
		job.Result = &wrong
		fixture.jobs[job.ID] = job
		fixture.mu.Unlock()
		for _, suffix := range []string{"/findings", "/checks"} {
			expect(request("owner", "GET", path+suffix, nil), 502)
		}
		fixture.mu.Lock()
		job.Result = original
		fixture.jobs[job.ID] = job
		fixture.mu.Unlock()
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
		expect(request(who, "PATCH", path+"/credentials", map[string]any{"credentials": json.RawMessage(credential)}), status)
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
	rotationStatus := 200
	if provider == "aws" {
		rotationStatus = 400
	}
	expect(request("owner", "PATCH", path+"/credentials", map[string]any{"credentials": json.RawMessage(credential)}), rotationStatus)
	if provider == "azure" {
		var wrongIdentity securityclient.AzureCredentials
		_ = json.Unmarshal(credential, &wrongIdentity)
		wrongIdentity.ClientID = "44444444-4444-4444-8444-444444444444"
		expect(request("owner", "PATCH", path+"/credentials", map[string]any{"credentials": wrongIdentity}), 400)
	}
	// Existing encrypted engine credentials migrate on an owner-initiated scan.
	if provider == "gcp" {
		sealed, err := sealCloudCredential(s.cfg.CloudEncryptionKey, stored, credential)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.cloudConnections.UpdateOne(ctx, bson.M{"_id": stored.ID}, bson.M{"$set": bson.M{"credential": sealed}, "$unset": bson.M{"security_connection_id": ""}})
		if err != nil {
			t.Fatal(err)
		}
		expect(request("owner", "POST", path+"/scan", nil), 202)
	}
	expect(request("owner", "DELETE", path, nil), 200)
	fixture.mu.Lock()
	remaining := len(fixture.connections)
	fixture.mu.Unlock()
	if remaining != 0 {
		t.Fatal("disconnect left credentials in Security")
	}
	setScope := func(n int) {
		if provider == "aws" {
			body.AccountID = fmt.Sprintf("%012d", n)
			var c securityclient.AWSCredentials
			json.Unmarshal(credential, &c)
			c.RoleARN = "arn:aws:iam::" + body.AccountID + ":role/RuntzTest"
			body.Credentials, _ = json.Marshal(c)
		} else if provider == "azure" {
			body.SubscriptionID = fmt.Sprintf("%08d-1111-4111-8111-111111111111", n)
		} else {
			body.ProjectID = fmt.Sprintf("project-%d", n)
		}
	}
	for n := 1; n <= 2; n++ {
		setScope(n)
		expect(request("owner", "POST", base+"/connections", body), 201)
	}
	fixture.mu.Lock()
	fixture.afterCreate = func() {
		if err := s.purgeWorkspaces(ctx, []bson.ObjectID{workspace.ID}); err != nil {
			t.Error(err)
		}
	}
	fixture.mu.Unlock()
	setScope(3)
	expect(request("owner", "POST", base+"/connections", body), 409)
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
