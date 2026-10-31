package securityclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const tenant = "aaaaaaaaaaaaaaaaaaaaaaaa"
const reference = "bbbbbbbbbbbbbbbbbbbbbbbb"
const serviceKey = "test-service-key-with-at-least-32-characters"

func TestTrustedTenantAndProjectValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceKey || r.Header.Get("X-Runtz-Tenant-ID") != tenant {
			t.Error("service credentials or trusted tenant missing")
		}
		var body struct {
			ProjectID string `json:"projectId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "project": map[string]string{"id": body.ProjectID}})
	}))
	defer server.Close()
	client := New(server.URL, serviceKey)
	if err := client.Validate(context.Background(), tenant, "test-project", json.RawMessage(`{"type":"service_account"}`)); err != nil {
		t.Fatal(err)
	}
	if err := client.Validate(context.Background(), "../other", "test-project", nil); !IsStatus(err, 400) {
		t.Fatal("invalid tenant was forwarded", err)
	}
}
func TestRedirectsAndRawErrorsDoNotLeakCredentials(t *testing.T) {
	hit := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		t.Error("redirect forwarded to another server")
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer server.Close()
	err := New(server.URL, serviceKey).Validate(context.Background(), tenant, "test-project", nil)
	if err == nil || hit || strings.Contains(err.Error(), serviceKey) {
		t.Fatal("unsafe redirect handling", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("private-raw-response"))
	}))
	defer bad.Close()
	err = New(bad.URL, serviceKey).Validate(context.Background(), tenant, "test-project", nil)
	if err == nil || strings.Contains(err.Error(), "private-raw-response") {
		t.Fatal("raw response exposed", err)
	}
}
func TestMetadataRejectsAnotherWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"connection": Connection{ID: reference, TenantID: "cccccccccccccccccccccccc", ProjectID: "test-project", Provider: "gcp"}})
	}))
	defer server.Close()
	if _, err := New(server.URL, serviceKey).Connection(context.Background(), tenant, reference); !IsStatus(err, 502) {
		t.Fatal("cross-tenant response accepted", err)
	}
}
func TestServiceConfiguration(t *testing.T) {
	for _, endpoint := range []string{"ftp://example.org", "https://user:password@example.org", "https://example.org/path", "https://example.org?key=secret", "https://example.org#fragment"} {
		if ValidateConfig(endpoint, serviceKey) == nil {
			t.Fatal("unsafe service origin accepted", endpoint)
		}
	}
	if ValidateConfig("", "") != nil || ValidateConfig("http://runtz-security:8080", serviceKey) != nil {
		t.Fatal("valid optional/internal configuration rejected")
	}
	if ValidateConfig("https://security.example.com", "") == nil {
		t.Fatal("missing service key accepted")
	}
}
