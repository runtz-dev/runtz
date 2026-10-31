package cloudgcp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"
)

func TestCredentialsRejectUntrustedEndpointsAndTypes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	c := Credentials{Type: "service_account", ProjectID: "test-project", ClientEmail: "test@test-project.iam.gserviceaccount.com", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), PrivateKeyID: "test-id", TokenURI: tokenURL}
	raw, _ := json.Marshal(c)
	if _, err := ParseCredentials(raw); err != nil {
		t.Fatal(err)
	}
	c.TokenURI = "https://attacker.example/token"
	raw, _ = json.Marshal(c)
	if _, err := ParseCredentials(raw); err == nil {
		t.Fatal("accepted an untrusted credential endpoint")
	}
	if _, err := ParseCredentials([]byte(`{"type":"external_account","credential_source":{"url":"http://localhost"}}`)); err == nil {
		t.Fatal("accepted external credentials")
	}
	for _, project := range []string{"../../secret", "123456", "test/project", "test-project?query=1"} {
		if ValidProject(project) {
			t.Fatalf("accepted project %s", project)
		}
	}
}
