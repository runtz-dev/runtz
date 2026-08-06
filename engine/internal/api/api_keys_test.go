package api

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestGenerateAPIKeyFormatAndPrefix(t *testing.T) {
	rawKey, prefix, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey() error = %v", err)
	}
	if rawKey == "" || prefix == "" {
		t.Fatal("generateAPIKey() returned empty key or prefix")
	}

	extractedPrefix, ok := extractAPIKeyPrefix(rawKey)
	if !ok {
		t.Fatalf("extractAPIKeyPrefix(%q) returned false", rawKey)
	}
	if extractedPrefix != prefix {
		t.Fatalf("prefix = %q, want %q", extractedPrefix, prefix)
	}
}

func TestAPIKeyAllowsScope(t *testing.T) {
	if !apiKeyAllowsScope(APIKey{Scopes: []string{"ingest:write"}}, "ingest:write") {
		t.Fatal("apiKeyAllowsScope denied an explicit scope")
	}
	if !apiKeyAllowsScope(APIKey{Scopes: []string{"*"}}, "ingest:write") {
		t.Fatal("apiKeyAllowsScope denied wildcard scope")
	}
	if apiKeyAllowsScope(APIKey{Scopes: []string{"scan:read"}}, "ingest:write") {
		t.Fatal("apiKeyAllowsScope allowed an unrelated scope")
	}
}

func TestVerifyKeyResponseShape(t *testing.T) {
	workspaceID := bson.NewObjectID()
	expiresAt := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	response := verifyKeyResponse(
		Workspace{ID: workspaceID, Name: "Acme"},
		APIKey{Name: "CLI key", Prefix: "rtz_live_4fa33139", ExpiresAt: &expiresAt},
	)

	workspace, ok := response["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("workspace missing from response: %v", response)
	}
	if workspace["id"] != workspaceID.Hex() || workspace["name"] != "Acme" {
		t.Fatalf("workspace = %v", workspace)
	}

	key, ok := response["apiKey"].(map[string]any)
	if !ok {
		t.Fatalf("apiKey missing from response: %v", response)
	}
	if key["name"] != "CLI key" || key["prefix"] != "rtz_live_4fa33139" {
		t.Fatalf("apiKey = %v", key)
	}
	if key["expiresAt"] != "2026-11-04T12:00:00Z" {
		t.Fatalf("expiresAt = %v, want RFC3339", key["expiresAt"])
	}

	// The token holder must not learn internal ids beyond the workspace id.
	for _, secret := range []string{"createdBy", "scopes", "id", "keyHash"} {
		if _, leaked := key[secret]; leaked {
			t.Fatalf("verify response leaks apiKey.%s", secret)
		}
	}

	// A key without expiry omits the field instead of sending a zero time.
	response = verifyKeyResponse(Workspace{ID: workspaceID, Name: "Acme"}, APIKey{Name: "CLI key"})
	key = response["apiKey"].(map[string]any)
	if _, present := key["expiresAt"]; present {
		t.Fatal("expiresAt present for a key without expiry")
	}
}

func TestExpiresAtFromDays(t *testing.T) {
	if got := expiresAtFromDays(time.Now(), 0); got != nil {
		t.Fatalf("expected nil expiry for 0 days, got %v", got)
	}
	if got := expiresAtFromDays(time.Now(), -1); got != nil {
		t.Fatalf("expected nil expiry for negative days, got %v", got)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := expiresAtFromDays(from, 90)
	if got == nil {
		t.Fatal("expected a non-nil expiry for 90 days")
	}
	want := from.AddDate(0, 0, 90)
	if !got.Equal(want) {
		t.Fatalf("expiresAtFromDays() = %v, want %v", got, want)
	}
}
