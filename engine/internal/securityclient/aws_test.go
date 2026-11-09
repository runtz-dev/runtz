package securityclient

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
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

func TestAWSAccessKeyRecoveryValidatesScopeBeforeRotation(t *testing.T) {
	id, secret := make([]byte, 8), make([]byte, 30)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	creds := AWSCredentials{AuthMethod: AWSAuthAccessKey, AccessKeyID: "AKIA" + strings.ToUpper(hex.EncodeToString(id)), SecretAccessKey: base64.StdEncoding.EncodeToString(secret), Regions: []string{"us-east-1"}}
	raw, _ := json.Marshal(creds)
	for _, test := range []struct {
		name, method, role string
		regions            []string
		valid              bool
	}{
		{"same scope", AWSAuthAccessKey, "", []string{"us-east-1"}, true},
		{"different regions", AWSAuthAccessKey, "", []string{"sa-east-1"}, false},
		{"different authentication", AWSAuthRole, "arn:aws:iam::123456789012:role/RuntzTest", []string{"us-east-1"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			rotated, enqueued := false, false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "POST /v1/connections":
					w.WriteHeader(409)
				case "GET /v1/connections":
					json.NewEncoder(w).Encode(map[string]any{"connections": []Connection{{ID: reference, TenantID: tenant, Provider: "aws", ProjectID: "123456789012", AccountID: "123456789012", AuthMethod: test.method, RoleARN: test.role, Regions: test.regions}}})
				case "PATCH /v1/connections/" + reference + "/credentials":
					rotated = true
					var body struct {
						Credentials json.RawMessage `json:"credentials"`
					}
					json.NewDecoder(r.Body).Decode(&body)
					received, err := ParseAWSCredentials(body.Credentials)
					if err != nil || received.AccessKeyID != creds.AccessKeyID || received.SecretAccessKey != creds.SecretAccessKey {
						t.Error("rotation did not forward replacement keys")
					}
					json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
				case "POST /v1/connections/" + reference + "/scans":
					enqueued = true
					json.NewEncoder(w).Encode(map[string]any{"scan": Job{ID: "dddddddddddddddddddddddd", TenantID: tenant, ConnectionID: reference}})
				default:
					t.Error("unexpected request", r.Method, r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			_, err := New(server.URL, serviceKey).CreateAWS(context.Background(), tenant, "AWS", "123456789012", raw)
			if (err == nil) != test.valid || rotated != test.valid || enqueued != test.valid {
				t.Fatal("recovery changed an incompatible connection or skipped validation", err)
			}
		})
	}
}
