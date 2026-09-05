package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runtz-dev/runtz/engine/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestDestructiveAccountEndpointsAreCloudOnly(t *testing.T) {
	t.Parallel()

	server := &Server{cfg: config.Config{DeploymentMode: hostingSelfHosted}}
	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{name: "workspace impact", method: http.MethodGet, path: "/api/v1/workspaces/id/deletion-impact", handler: server.handleWorkspaceDeletionImpact},
		{name: "workspace deletion", method: http.MethodDelete, path: "/api/v1/workspaces/id", handler: server.handleDeleteWorkspace},
		{name: "account impact", method: http.MethodGet, path: "/api/v1/me/deletion-impact", handler: server.handleAccountDeletionImpact},
		{name: "account deletion", method: http.MethodDelete, path: "/api/v1/me", handler: server.handleDeleteAccount},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			response := httptest.NewRecorder()
			testCase.handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}

func TestWorkspaceDeletionConfirmationRequiresExactName(t *testing.T) {
	t.Parallel()

	if !workspaceDeletionConfirmed("  Production  ", "Production") {
		t.Fatal("trimmed exact workspace name should confirm deletion")
	}
	if workspaceDeletionConfirmed("production", "Production") {
		t.Fatal("workspace confirmation must preserve case")
	}
	if workspaceDeletionConfirmed("Production-2", "Production") {
		t.Fatal("a different workspace name confirmed deletion")
	}
}

func TestAccountDeletionConfirmationUsesNormalizedEmail(t *testing.T) {
	t.Parallel()

	user := User{
		ID:       bson.NewObjectID(),
		Username: "alex",
		Email:    " Alex@Example.COM ",
	}
	if got := accountDeletionConfirmationValue(user); got != "alex@example.com" {
		t.Fatalf("confirmation value = %q, want normalized email", got)
	}
	if !accountDeletionConfirmed(" ALEX@example.com ", user) {
		t.Fatal("email confirmation should be case-insensitive and trimmed")
	}
	if accountDeletionConfirmed("alex", user) {
		t.Fatal("username confirmed deletion when the account has an email")
	}
}

func TestAccountDeletionConfirmationFallsBackToUsername(t *testing.T) {
	t.Parallel()

	user := User{Username: "local-admin"}
	if got := accountDeletionConfirmationValue(user); got != "local-admin" {
		t.Fatalf("confirmation value = %q, want username", got)
	}
	if !accountDeletionConfirmed("LOCAL-ADMIN", user) {
		t.Fatal("username fallback should be case-insensitive")
	}
}
