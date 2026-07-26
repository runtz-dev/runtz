package api

import (
	"testing"

	"github.com/runtz-dev/runtz/engine/internal/config"
)

func TestGitHubRedirectURIAllowed(t *testing.T) {
	server := &Server{
		cfg: config.Config{
			CORSAllowedOrigins: []string{"https://runtz.dev", "http://localhost:3000"},
		},
	}

	tests := []struct {
		name        string
		redirectURI string
		allowed     bool
	}{
		{name: "production callback", redirectURI: "https://runtz.dev/login/github/callback", allowed: true},
		{name: "local callback", redirectURI: "http://localhost:3000/login/github/callback", allowed: true},
		{name: "untrusted origin", redirectURI: "https://example.com/login/github/callback", allowed: false},
		{name: "wrong path", redirectURI: "https://runtz.dev/login", allowed: false},
		{name: "callback with query", redirectURI: "https://runtz.dev/login/github/callback?next=/app", allowed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := server.githubRedirectURIAllowed(tt.redirectURI); got != tt.allowed {
				t.Fatalf("githubRedirectURIAllowed(%q) = %v, want %v", tt.redirectURI, got, tt.allowed)
			}
		})
	}
}
