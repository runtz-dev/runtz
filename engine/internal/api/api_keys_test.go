package api

import (
	"testing"
	"time"
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
