package securityclient

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const subscription = "11111111-1111-4111-8111-111111111111"

func TestAzureValidationChecksExactSubscription(t *testing.T) {
	for _, returned := range []string{subscription, "22222222-2222-4222-8222-222222222222"} {
		t.Run(returned, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/azure/validate" || r.Header.Get("X-Runtz-Tenant-ID") != tenant {
					t.Error("wrong provider route or workspace")
				}
				var body map[string]json.RawMessage
				_ = json.NewDecoder(r.Body).Decode(&body)
				if string(body["subscriptionId"]) != `"`+subscription+`"` || body["projectId"] != nil {
					t.Error("wrong Azure request scope")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "subscription": map[string]string{"id": returned}})
			}))
			defer server.Close()
			err := New(server.URL, serviceKey).ValidateAzure(context.Background(), tenant, subscription, nil)
			if returned == subscription && err != nil {
				t.Fatal(err)
			}
			if returned != subscription && !IsStatus(err, 502) {
				t.Fatal("mismatched subscription accepted")
			}
		})
	}
}

func TestAzureRecoveryKeepsProviderAndTenant(t *testing.T) {
	rotations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceKey || r.Header.Get("X-Runtz-Tenant-ID") != tenant {
			t.Error("missing service authentication or workspace")
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/connections":
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["projectId"] != nil || string(body["provider"]) != `"azure"` || string(body["subscriptionId"]) != `"`+subscription+`"` {
				t.Error("Azure create contract mismatch")
			}
			w.WriteHeader(409)
		case "GET /v1/connections":
			_ = json.NewEncoder(w).Encode(map[string]any{"connections": []Connection{{ID: "cccccccccccccccccccccccc", TenantID: tenant, Provider: "gcp", ProjectID: subscription}, {ID: reference, TenantID: tenant, Provider: "azure", ProjectID: subscription, SubscriptionID: subscription}}})
		case "PATCH /v1/connections/" + reference + "/credentials":
			rotations++
			_, _ = w.Write([]byte(`{"status":"updated"}`))
		case "POST /v1/connections/" + reference + "/scans":
			_ = json.NewEncoder(w).Encode(map[string]any{"scan": Job{ID: "dddddddddddddddddddddddd", TenantID: tenant, ConnectionID: reference, State: "queued"}})
		default:
			t.Error("unexpected request", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	out, err := New(server.URL, serviceKey).CreateAzure(context.Background(), tenant, "Azure", subscription, nil)
	if err != nil || out.Connection.ID != reference || rotations != 1 {
		t.Fatal("Azure recovery failed", err)
	}
}

func TestAzureCredentialValidation(t *testing.T) {
	c := AzureCredentials{TenantID: subscription, ClientID: "22222222-2222-4222-8222-222222222222", ClientSecret: rand.Text()}
	raw, _ := json.Marshal(c)
	if _, err := ParseAzureCredentials(raw); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{nil, []byte(`{"tenantId":"bad"}`), append(append([]byte(nil), raw...), []byte(` {}`)...)} {
		if _, err := ParseAzureCredentials(raw); err == nil {
			t.Fatal("invalid credential accepted")
		}
	}
}
