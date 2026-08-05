package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/runtz-dev/runtz/engine/internal/config"
)

// signedInstanceState builds an InstanceState carrying a license certificate
// signed by priv, as storeValidatedLicense would persist it.
func signedInstanceState(t *testing.T, priv ed25519.PrivateKey, installationID, plan string) InstanceState {
	t.Helper()
	payload := LicensePayload{
		InstallationID: installationID,
		Plan:           plan,
		Status:         "active",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	sig := ed25519.Sign(priv, raw)
	return InstanceState{
		InstallationID:    installationID,
		LicensePayload:    payload,
		LicensePayloadRaw: base64.StdEncoding.EncodeToString(raw),
		LicenseSignature:  base64.StdEncoding.EncodeToString(sig),
	}
}

func TestVerifiedStoredLicense(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	server := &Server{cfg: config.Config{LicensePublicKey: base64.StdEncoding.EncodeToString(pub)}}

	// A correctly signed certificate verifies and returns the signed plan.
	state := signedInstanceState(t, priv, "rti_abc", planEnterprise)
	payload, ok := server.verifiedStoredLicense(state)
	if !ok || payload.Plan != planEnterprise {
		t.Fatalf("valid license: ok=%v plan=%q, want ok=true plan=%q", ok, payload.Plan, planEnterprise)
	}

	// Tampering with the convenience struct (not the signed bytes) is ignored:
	// the authoritative plan comes from the verified payload.
	tampered := state
	tampered.LicensePayload.Plan = planEnterprise
	tampered.LicensePayloadRaw = signedInstanceState(t, priv, "rti_abc", planPro).LicensePayloadRaw
	tampered.LicenseSignature = signedInstanceState(t, priv, "rti_abc", planPro).LicenseSignature
	if payload, ok := server.verifiedStoredLicense(tampered); !ok || payload.Plan != planPro {
		t.Fatalf("struct tamper: ok=%v plan=%q, want ok=true plan=%q", ok, payload.Plan, planPro)
	}

	// A different key must not verify (look-alike central engine / self-signed).
	otherPub, _, _ := ed25519.GenerateKey(nil)
	wrongKey := &Server{cfg: config.Config{LicensePublicKey: base64.StdEncoding.EncodeToString(otherPub)}}
	if _, ok := wrongKey.verifiedStoredLicense(state); ok {
		t.Fatal("license signed by another key was accepted")
	}

	// Fail closed when no verification key is compiled in.
	empty := &Server{cfg: config.Config{LicensePublicKey: ""}}
	if _, ok := empty.verifiedStoredLicense(state); ok {
		t.Fatal("verification passed with an empty key (should fail closed)")
	}

	// A certificate whose signed installation id does not match this instance's
	// id is rejected (a license copied from another installation).
	mism := signedInstanceState(t, priv, "rti_abc", planPro)
	mism.InstallationID = "rti_different"
	if _, ok := server.verifiedStoredLicense(mism); ok {
		t.Fatal("installation mismatch was accepted")
	}

	// Missing certificate bytes → unlicensed.
	if _, ok := server.verifiedStoredLicense(InstanceState{InstallationID: "rti_abc"}); ok {
		t.Fatal("missing raw/signature was accepted")
	}
}

func TestParseEd25519Keys(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	parsedPrivate, err := parseEd25519PrivateKey(base64.StdEncoding.EncodeToString(privateKey))
	if err != nil {
		t.Fatalf("parseEd25519PrivateKey() error = %v", err)
	}
	parsedPublic, err := parseEd25519PublicKey(base64.StdEncoding.EncodeToString(publicKey))
	if err != nil {
		t.Fatalf("parseEd25519PublicKey() error = %v", err)
	}

	message := []byte("license-payload")
	signature := ed25519.Sign(parsedPrivate, message)
	if !ed25519.Verify(parsedPublic, message, signature) {
		t.Fatal("parsed keys failed signature verification")
	}
}

func TestCentralEngineReachabilityErrorIsActionable(t *testing.T) {
	err := centralEngineReachabilityError(errors.New("network unreachable"))
	message := err.Error()
	for _, want := range []string{
		"https://engine.runtz.dev",
		"must be able to connect",
		"network unreachable",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message %q does not contain %q", message, want)
		}
	}
}
