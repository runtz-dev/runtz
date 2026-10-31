package api

import (
	"bytes"
	"context"
	"encoding/json"
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

// Explicit opt-in; uses an isolated local engine database and a disposable
// Security tenant. Only the provided GCP project's configuration is read.
func TestLiveSecurityGCPIntegration(t *testing.T) {
	uri, endpoint, serviceKey, path := os.Getenv("RUNTZ_TEST_MONGO_URI"), os.Getenv("RUNTZ_TEST_SECURITY_URL"), os.Getenv("RUNTZ_TEST_SECURITY_API_KEY"), os.Getenv("RUNTZ_TEST_GCP_KEY_FILE")
	if uri == "" || endpoint == "" || serviceKey == "" || path == "" {
		t.Skip("set disposable Mongo URI, Security URL/key and an authorized GCP key file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("could not read authorized GCP key")
	}
	defer clear(raw)
	credential, err := cloudgcp.ParseCredentials(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	s, err := New(ctx, config.Config{DeploymentMode: hostingCloud, MongoURI: uri, MongoDatabase: "runtz_security_live_" + bson.NewObjectID().Hex(), SecurityAPIURL: endpoint, SecurityAPIKey: serviceKey})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	defer s.db.Drop(context.Background())
	owner, workspaces, err := s.findOrCreateEmailUser(ctx, "cloud-integration@example.test")
	if err != nil {
		t.Fatal(err)
	}
	workspace := workspaces[0]
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := s.purgeWorkspaces(cleanup, []bson.ObjectID{workspace.ID}); err != nil {
			t.Error("live test tenant cleanup failed")
		}
	}()
	token, err := s.issueSession(ctx, owner, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		payload, _ := json.Marshal(body)
		defer clear(payload)
		r := httptest.NewRequest(method, path, bytes.NewReader(payload)).WithContext(ctx)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "PRIVATE KEY") || strings.Contains(w.Body.String(), serviceKey) {
			t.Fatal("credential leaked in live response")
		}
		return w
	}
	body := cloudConnectRequest{WorkspaceID: workspace.ID.Hex(), ProjectID: credential.ProjectID, Name: "Integration verification", Credentials: json.RawMessage(raw)}
	if w := request("POST", "/api/v1/cloud/gcp/validate", body); w.Code != 200 {
		t.Fatalf("live project validation: HTTP %d %s", w.Code, w.Body.String())
	}
	w := request("POST", "/api/v1/cloud/gcp/connections", body)
	if w.Code != 201 {
		t.Fatalf("live onboarding: HTTP %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Connection cloudConnection `json:"connection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	var stored cloudConnection
	if err := s.cloudConnections.FindOne(ctx, bson.M{"_id": created.Connection.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Credential != "" || stored.SecurityConnectionID == "" {
		t.Fatal("engine duplicated credential instead of storing its reference")
	}
	connectionPath := "/api/v1/cloud/connections/" + created.Connection.ID.Hex()
	for {
		if ctx.Err() != nil {
			t.Fatal("live scan did not finish within the verification timeout")
		}
		w = request("GET", connectionPath+"/findings", nil)
		if w.Code != 200 {
			t.Fatalf("live findings: HTTP %d %s", w.Code, w.Body.String())
		}
		var response struct {
			ResultAvailable bool `json:"resultAvailable"`
			Total           int  `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.ResultAvailable {
			w = request("GET", connectionPath+"/checks", nil)
			if w.Code != 200 {
				t.Fatalf("live coverage: HTTP %d", w.Code)
			}
			var coverage struct {
				Total int `json:"total"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &coverage); err != nil || coverage.Total < 1 {
				t.Fatal("missing live check coverage")
			}
			t.Logf("Engine -> Security -> GCP completed: %d findings, %d resource checks; credential stored only in Security", response.Total, coverage.Total)
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("live verification timed out")
		case <-time.After(time.Second):
		}
	}
}
