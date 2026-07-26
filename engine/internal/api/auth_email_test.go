package api

import (
	"testing"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/config"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "normalizes case and spaces", input: "  User@Example.COM ", want: "user@example.com"},
		{name: "rejects missing domain", input: "user", wantErr: true},
		{name: "rejects display name", input: "User <user@example.com>", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeEmail(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeEmail() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateEmailLoginCode(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		code, err := generateEmailLoginCode()
		if err != nil {
			t.Fatalf("generateEmailLoginCode() error = %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("generateEmailLoginCode() length = %d, want 6", len(code))
		}
		for _, character := range code {
			if character < '0' || character > '9' {
				t.Fatalf("generateEmailLoginCode() = %q, want digits only", code)
			}
		}
	}
}

func TestEmailLockoutRetryMinutes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		lockedUntil time.Time
		want        int
	}{
		{name: "just under an hour rounds to 60", lockedUntil: now.Add(59*time.Minute + 40*time.Second), want: 60},
		{name: "exactly thirty minutes", lockedUntil: now.Add(30 * time.Minute), want: 30},
		{name: "rounds to the nearest minute", lockedUntil: now.Add(90 * time.Second), want: 2},
		{name: "sub-minute remainder floors to 1, never 0", lockedUntil: now.Add(20 * time.Second), want: 1},
		{name: "already expired still reports at least 1", lockedUntil: now.Add(-time.Minute), want: 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := emailLockoutRetryMinutes(tt.lockedUntil, now); got != tt.want {
				t.Fatalf("emailLockoutRetryMinutes() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHashEmailLoginCodeUsesEmailAndSecret(t *testing.T) {
	t.Parallel()

	server := &Server{cfg: config.Config{JWTSecret: "first-secret"}}
	hash := server.hashEmailLoginCode("user@example.com", "123456")
	if hash == server.hashEmailLoginCode("other@example.com", "123456") {
		t.Fatal("expected email to affect hash")
	}

	otherServer := &Server{cfg: config.Config{JWTSecret: "second-secret"}}
	if hash == otherServer.hashEmailLoginCode("user@example.com", "123456") {
		t.Fatal("expected JWT secret to affect hash")
	}
}
