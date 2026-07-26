package config

import (
	"strings"
	"testing"
)

func TestValidateAcceptsStrongSecrets(t *testing.T) {
	cfg := Config{
		JWTSecret:   strings.Repeat("a", minJWTSecretLength),
		IngestToken: strings.Repeat("b", minIngestTokenLength),
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateRejectsWeakSecrets(t *testing.T) {
	strongJWT := strings.Repeat("a", minJWTSecretLength)
	strongIngest := strings.Repeat("b", minIngestTokenLength)

	cases := []struct {
		name        string
		jwtSecret   string
		ingestToken string
		wantSubstr  string
	}{
		{"empty jwt secret", "", strongIngest, "JWT_SECRET"},
		{"placeholder jwt secret", "change-me-in-production", strongIngest, "JWT_SECRET"},
		{"short jwt secret", "too-short", strongIngest, "JWT_SECRET"},
		{"empty ingest token", strongJWT, "", "RUNTZ_INGEST_TOKEN"},
		{"placeholder ingest token", strongJWT, "dev-ingest-token", "RUNTZ_INGEST_TOKEN"},
		{"short ingest token", strongJWT, "short", "RUNTZ_INGEST_TOKEN"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{JWTSecret: tc.jwtSecret, IngestToken: tc.ingestToken}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error mentioning %q, got: %v", tc.wantSubstr, err)
			}
		})
	}
}
