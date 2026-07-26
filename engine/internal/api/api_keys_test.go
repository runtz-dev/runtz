package api

import "testing"

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
