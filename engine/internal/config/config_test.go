package config

import (
	"encoding/base64"
	"testing"
)

// The engine used to refuse to start on a missing or placeholder JWT_SECRET /
// RUNTZ_INGEST_TOKEN, and this file tested every way of getting that wrong.
// Both secrets are gone: sessions, API keys and login codes are issued and
// stored by the engine, so an empty environment remains valid. Cloud integration
// encryption is opt-in and is validated separately when configured.
func TestValidateAcceptsEmptyConfig(t *testing.T) {
	t.Parallel()

	if err := (Config{}).Validate(); err != nil {
		t.Fatalf("expected an empty config to be valid, got error: %v", err)
	}
}

func TestValidateCloudEncryptionKey(t *testing.T) {
	for _, key := range []string{"invalid", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		if (Config{CloudEncryptionKey: key}).Validate() == nil {
			t.Fatal("invalid cloud encryption key accepted")
		}
	}
	if err := (Config{CloudEncryptionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))}).Validate(); err != nil {
		t.Fatal(err)
	}
}
