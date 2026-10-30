package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/runtz-dev/runtz/engine/internal/cloudgcp"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCloudCredentialEncryptionAndTenantBinding(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	c := cloudConnection{ID: bson.NewObjectID(), WorkspaceID: bson.NewObjectID(), Provider: "gcp"}
	raw := []byte(`{"private_key":"test-secret"}`)
	sealed, err := sealCloudCredential(key, c, raw)
	if err != nil {
		t.Fatal(err)
	}
	c.Credential = sealed
	if bytes.Contains([]byte(sealed), raw) {
		t.Fatal("credential stored in plaintext")
	}
	opened, err := openCloudCredential(key, c)
	if err != nil || !bytes.Equal(opened, raw) {
		t.Fatal("credential did not decrypt")
	}
	public, _ := json.Marshal(c)
	if bytes.Contains(public, []byte("test-secret")) || bytes.Contains(public, []byte(sealed)) {
		t.Fatal("credential exposed in JSON")
	}
	other := c
	other.WorkspaceID = bson.NewObjectID()
	if _, err := openCloudCredential(key, other); err == nil {
		t.Fatal("credential can be moved between tenants")
	}
	other = c
	other.ID = bson.NewObjectID()
	if _, err := openCloudCredential(key, other); err == nil {
		t.Fatal("credential can be moved between connections")
	}
	if _, err := openCloudCredential(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)), c); err == nil {
		t.Fatal("wrong key accepted")
	}
	c.Credential = "v1:bad"
	if _, err := openCloudCredential(key, c); err == nil {
		t.Fatal("corrupted ciphertext accepted")
	}
	if _, err := sealCloudCredential("", c, raw); err == nil {
		t.Fatal("missing encryption key accepted")
	}
}

func TestCloudCountsExcludeInactiveAndMutedFindings(t *testing.T) {
	counts := countCloudFindings([]cloudgcp.Finding{
		{State: "ACTIVE", Severity: "critical"}, {State: "ACTIVE", Severity: "high"},
		{State: "ACTIVE", Severity: "medium"}, {State: "ACTIVE", Severity: "low"}, {State: "ACTIVE", Severity: "unknown"},
		{State: "INACTIVE", Severity: "critical"}, {State: "ACTIVE", Mute: "MUTED", Severity: "high"},
	})
	if counts.Total != 5 || counts.Critical != 1 || counts.High != 1 || counts.Medium != 1 || counts.Low != 1 || counts.Unknown != 1 {
		t.Fatalf("wrong counts: %+v", counts)
	}
}
