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

// TestSelfHostedInviteFlow exercises the whole path this session added:
// create a user with just a username (no password), get an invite link back,
// accept it to set a password and land signed in, and confirm a viewer can't
// create API keys while an admin still can.
func TestSelfHostedInviteFlow(t *testing.T) {
	uri := os.Getenv("RUNTZ_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set RUNTZ_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	s, err := New(ctx, config.Config{DeploymentMode: hostingSelfHosted, MongoURI: uri, MongoDatabase: "runtz_test_" + bson.NewObjectID().Hex()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	defer s.db.Drop(context.Background())

	request := func(token, method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: HTTP %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}

	now := time.Now().UTC()
	workspace := Workspace{ID: bson.NewObjectID(), Name: "default", Slug: "default", CreatedAt: now, UpdatedAt: now}
	if _, err := s.workspaces.InsertOne(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	admin := User{ID: bson.NewObjectID(), Username: "admin", Role: "admin", WorkspaceIDs: []bson.ObjectID{workspace.ID}, CreatedAt: now, UpdatedAt: now}
	if _, err := s.users.InsertOne(ctx, admin); err != nil {
		t.Fatal(err)
	}
	adminToken, err := s.issueSession(ctx, admin, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}

	// Username-only creation: no password field is even accepted anymore.
	createBody := `{"username":"new-viewer","workspaceIds":["` + workspace.ID.Hex() + `"]}`
	created := request(adminToken, http.MethodPost, "/api/v1/users", createBody, http.StatusCreated)
	var createResponse struct {
		User struct {
			ID          string `json:"id"`
			Role        string `json:"role"`
			PasswordSet bool   `json:"passwordSet"`
		} `json:"user"`
		InviteLink string `json:"inviteLink"`
	}
	decodeBody(t, created, &createResponse)
	if createResponse.User.Role != "viewer" {
		t.Fatalf("default role = %q, want viewer", createResponse.User.Role)
	}
	if createResponse.User.PasswordSet {
		t.Fatal("a freshly invited user should not have a password set yet")
	}
	if !strings.Contains(createResponse.InviteLink, "/invite/") {
		t.Fatalf("unexpected invite link: %q", createResponse.InviteLink)
	}
	token := createResponse.InviteLink[strings.LastIndex(createResponse.InviteLink, "/invite/")+len("/invite/"):]

	// The invite page looks the invite up before showing the form.
	invitePreview := request("", http.MethodGet, "/api/v1/invites/"+token, "", http.StatusOK)
	var previewResponse struct {
		Username string `json:"username"`
	}
	decodeBody(t, invitePreview, &previewResponse)
	if previewResponse.Username != "new-viewer" {
		t.Fatalf("invite preview username = %q, want new-viewer", previewResponse.Username)
	}

	// A bad token must not work.
	request("", http.MethodGet, "/api/v1/invites/not-a-real-token", "", http.StatusNotFound)

	// Accepting sets the password and signs the invitee in.
	accept := request("", http.MethodPost, "/api/v1/invites/"+token+"/accept", `{"password":"correct horse battery staple"}`, http.StatusOK)
	var acceptResponse struct {
		User struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"user"`
	}
	decodeBody(t, accept, &acceptResponse)
	var setCookie string
	for _, cookie := range accept.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			setCookie = cookie.Value
		}
	}
	if setCookie == "" {
		t.Fatal("accepting an invite should start a session")
	}

	// The invite is single-use: accepting again with the same token fails.
	request("", http.MethodPost, "/api/v1/invites/"+token+"/accept", `{"password":"another password entirely"}`, http.StatusNotFound)

	// The new user can sign in with the password they just chose, and reads
	// as role "viewer".
	viewerLogin := request("", http.MethodPost, "/api/v1/auth/login", `{"username":"new-viewer","password":"correct horse battery staple"}`, http.StatusOK)
	var viewerCookie string
	for _, cookie := range viewerLogin.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			viewerCookie = cookie.Value
		}
	}

	// Viewers cannot create API keys on self-hosted; admins can.
	keyBody := `{"name":"cli","workspaceId":"` + workspace.ID.Hex() + `"}`
	request(viewerCookie, http.MethodPost, "/api/v1/api-keys", keyBody, http.StatusForbidden)
	request(adminToken, http.MethodPost, "/api/v1/api-keys", keyBody, http.StatusCreated)
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response body: %v (%s)", err, w.Body.String())
	}
}
