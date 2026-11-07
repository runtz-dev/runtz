package securityclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAWSValidationRejectsDifferentAccount(t *testing.T) {
	for _, account := range []string{"123456789012", "999999999999"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/aws/validate" || r.Header.Get("X-Runtz-Tenant-ID") != tenant {
				t.Error("incorrect AWS route or tenant")
			}
			json.NewEncoder(w).Encode(map[string]any{"valid": true, "accountId": account})
		}))
		err := New(server.URL, serviceKey).ValidateAWS(context.Background(), tenant, "123456789012", nil)
		server.Close()
		if (err == nil) != (account == "123456789012") {
			t.Fatal("account boundary not enforced", err)
		}
	}
}
func TestAWSRecoveryChecksImmutableRoleAndRegions(t *testing.T) {
	credential, _ := json.Marshal(AWSCredentials{RoleARN: "arn:aws:iam::123456789012:role/RuntzTest", ExternalID: "runtz-" + strings.Repeat("a", 64), Regions: []string{"us-east-1"}})
	for _, role := range []string{"RuntzTest", "OtherRole"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method + " " + r.URL.Path {
			case "POST /v1/connections":
				w.WriteHeader(409)
			case "GET /v1/connections":
				json.NewEncoder(w).Encode(map[string]any{"connections": []Connection{{ID: reference, TenantID: tenant, Provider: "aws", ProjectID: "123456789012", AccountID: "123456789012", RoleARN: "arn:aws:iam::123456789012:role/" + role, Regions: []string{"us-east-1"}}}})
			case "POST /v1/connections/" + reference + "/scans":
				json.NewEncoder(w).Encode(map[string]any{"scan": Job{ID: "dddddddddddddddddddddddd", TenantID: tenant, ConnectionID: reference}})
			default:
				t.Error("unexpected mutation", r.Method, r.URL.Path)
				w.WriteHeader(500)
			}
		}))
		_, err := New(server.URL, serviceKey).CreateAWS(context.Background(), tenant, "AWS", "123456789012", credential)
		server.Close()
		if (err == nil) != (role == "RuntzTest") {
			t.Fatal("AWS recovery scope mismatch", err)
		}
	}
}
