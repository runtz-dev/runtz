package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Run against a disposable MongoDB with RUNTZ_TEST_MONGO_URI set. Each test
// creates and drops its own randomly named database.
func TestCloudWorkspaceLifecycle(t *testing.T) {
	uri := os.Getenv("RUNTZ_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set RUNTZ_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	s, err := New(ctx, config.Config{DeploymentMode: hostingCloud, MongoURI: uri, MongoDatabase: "runtz_test_" + bson.NewObjectID().Hex()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	defer s.db.Drop(context.Background())

	user, workspaces, err := s.findOrCreateEmailUser(ctx, "workspace-test@gmail.com")
	if err != nil || len(workspaces) != 1 {
		t.Fatalf("signup: workspaces=%v, err=%v", workspaces, err)
	}
	original := workspaces[0]
	token, err := s.issueSession(ctx, user, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	expectStatus := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("HTTP %d, want %d: %s", w.Code, status, w.Body.String())
		}
	}
	expectStatus(request(http.MethodPost, "/api/v1/workspaces", `{"name":"second"}`), http.StatusPaymentRequired)
	if _, err := s.scans.InsertOne(ctx, bson.M{"workspace_id": original.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.apiKeys.InsertOne(ctx, bson.M{"workspace_id": original.ID, "key_hash": "test-key"}); err != nil {
		t.Fatal(err)
	}

	impactResponse := request(http.MethodGet, "/api/v1/workspaces/"+original.ID.Hex()+"/deletion-impact", "")
	expectStatus(impactResponse, http.StatusOK)
	var impact workspaceDeletionImpact
	if err := json.Unmarshal(impactResponse.Body.Bytes(), &impact); err != nil {
		t.Fatal(err)
	}
	if impact.ReplacementWorkspaceWillBeCreated || impact.ScanCount != 1 || impact.APIKeyCount != 1 {
		t.Fatalf("unexpected impact: %+v", impact)
	}
	expectStatus(request(http.MethodDelete, "/api/v1/workspaces/"+original.ID.Hex(), `{"confirmation":"wrong"}`), http.StatusBadRequest)
	deleted := request(http.MethodDelete, "/api/v1/workspaces/"+original.ID.Hex(), `{"confirmation":"personal"}`)
	expectStatus(deleted, http.StatusOK)
	if strings.Contains(deleted.Body.String(), "replacementWorkspace") {
		t.Fatal("deletion returned a replacement")
	}
	for _, collection := range []string{"workspaces", "scans", "api_keys"} {
		count, err := s.db.Collection(collection).CountDocuments(ctx, bson.M{})
		if err != nil || count != 0 {
			t.Fatalf("%s after deletion: count=%d, err=%v", collection, count, err)
		}
	}

	logins := []struct {
		name  string
		login func() (User, []Workspace, error)
	}{
		{"email", func() (User, []Workspace, error) { return s.findOrCreateEmailUser(ctx, user.Email) }},
		{"google", func() (User, []Workspace, error) {
			return s.findOrCreateGoogleUser(ctx, googleProfile{Subject: "google-workspace-test", Email: user.Email, Username: user.Username, Workspace: "personal", Kind: "personal"})
		}},
		{"github", func() (User, []Workspace, error) {
			return s.findOrCreateGitHubUser(ctx, githubProfile{Subject: "github-workspace-test", Email: user.Email})
		}},
	}
	for _, login := range logins {
		loggedIn, visible, err := login.login()
		if err != nil || len(visible) != 0 || len(loggedIn.WorkspaceIDs) != 0 {
			t.Fatalf("%s recreated workspace: visible=%v, err=%v", login.name, visible, err)
		}
	}
	empty := request(http.MethodGet, "/api/v1/workspaces", "")
	expectStatus(empty, http.StatusOK)
	if !strings.Contains(empty.Body.String(), `"workspaces":[]`) {
		t.Fatalf("expected empty list: %s", empty.Body.String())
	}

	// Another account may use the same display name; slugs must remain unique.
	other, _, err := s.findOrCreateEmailUser(ctx, "other-workspace-test@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", " My custom workspace "} {
		body, _ := json.Marshal(map[string]string{"name": name})
		created := request(http.MethodPost, "/api/v1/workspaces", string(body))
		expectStatus(created, http.StatusCreated)
		var response struct {
			Workspace struct{ ID, Name, Slug, CreatedBy string }
		}
		if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		expectedName := strings.TrimSpace(name)
		if expectedName == "" {
			expectedName = "personal"
		}
		if response.Workspace.Name != expectedName || response.Workspace.CreatedBy != user.ID.Hex() {
			t.Fatalf("wrong workspace: %+v", response)
		}
		var refreshed User
		if err := s.users.FindOne(ctx, bson.M{"_id": user.ID}).Decode(&refreshed); err != nil {
			t.Fatal(err)
		}
		visible, err := s.getVisibleWorkspaces(ctx, refreshed)
		if err != nil || len(visible) != 1 || visible[0].Name != expectedName {
			t.Fatalf("created workspace not visible: %v, %v", visible, err)
		}
		expectStatus(request(http.MethodPost, "/api/v1/workspaces", `{"name":"over limit"}`), http.StatusPaymentRequired)
		confirmation, _ := json.Marshal(map[string]string{"confirmation": expectedName})
		expectStatus(request(http.MethodDelete, "/api/v1/workspaces/"+response.Workspace.ID, string(confirmation)), http.StatusOK)
	}
	visible, err := s.getVisibleWorkspaces(ctx, other)
	if err != nil || len(visible) != 1 {
		t.Fatalf("other user's workspace affected: %v, %v", visible, err)
	}

	// Self-hosted members still cannot create workspaces.
	s.cfg.DeploymentMode = hostingSelfHosted
	expectStatus(request(http.MethodPost, "/api/v1/workspaces", `{"name":"forbidden"}`), http.StatusForbidden)
}
