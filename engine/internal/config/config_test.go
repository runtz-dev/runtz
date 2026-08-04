package config

import "testing"

// The engine used to refuse to start on a missing or placeholder JWT_SECRET /
// RUNTZ_INGEST_TOKEN, and this file tested every way of getting that wrong.
// Both secrets are gone: sessions, API keys and login codes are issued and
// stored by the engine, so an empty environment is now a correct one and there
// is no weak value left for an operator to supply.
func TestValidateAcceptsEmptyConfig(t *testing.T) {
	t.Parallel()

	if err := (Config{}).Validate(); err != nil {
		t.Fatalf("expected an empty config to be valid, got error: %v", err)
	}
}
