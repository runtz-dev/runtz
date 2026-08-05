package api

import "testing"

func TestWorkspaceDefaultsForEmail(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		username      string
		workspaceName string
		workspaceKind string
	}{
		{
			name:          "personal gmail",
			email:         "Alex.Costa@gmail.com",
			username:      "alex-costa",
			workspaceName: "personal",
			workspaceKind: "personal",
		},
		{
			name:          "company domain",
			email:         "alex@acme.io",
			username:      "alex",
			workspaceName: "acme",
			workspaceKind: "company",
		},
		{
			name:          "company compound suffix",
			email:         "sec@company.com.br",
			username:      "sec",
			workspaceName: "company",
			workspaceKind: "company",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username, workspaceName, workspaceKind := workspaceDefaultsForEmail(tt.email)
			if username != tt.username {
				t.Fatalf("username = %q, want %q", username, tt.username)
			}
			if workspaceName != tt.workspaceName {
				t.Fatalf("workspaceName = %q, want %q", workspaceName, tt.workspaceName)
			}
			if workspaceKind != tt.workspaceKind {
				t.Fatalf("workspaceKind = %q, want %q", workspaceKind, tt.workspaceKind)
			}
		})
	}
}
