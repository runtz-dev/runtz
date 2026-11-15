package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCLICloudIngestLifecycleAndIsolation(t *testing.T) {
	s, ctx := billingTestServer(t)
	owner, workspaces, err := s.findOrCreateEmailUser(ctx, "cli-cloud@example.com")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := s.findOrCreateEmailUser(ctx, "other-cli-cloud@example.com")
	if err != nil {
		t.Fatal(err)
	}
	workspace := workspaces[0]
	token, prefix, err := generateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.apiKeys.InsertOne(ctx, APIKey{ID: bson.NewObjectID(), WorkspaceID: workspace.ID, CreatedBy: owner.ID, KeyHash: hashAPIKey(token), Prefix: prefix, Scopes: []string{"ingest:write"}}); err != nil {
		t.Fatal(err)
	}
	session, err := s.issueSession(ctx, owner, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	otherSession, err := s.issueSession(ctx, other, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, body any, credential string, apiKey bool) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw)).WithContext(ctx)
		if apiKey {
			r.Header.Set("Authorization", "Bearer "+credential)
		} else {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: credential})
		}
		r.Header.Set("X-Runtz-Tenant-ID", other.WorkspaceIDs[0].Hex())
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	expect := func(t *testing.T, w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("HTTP %d, want %d: %s", w.Code, status, w.Body.String())
		}
	}
	for _, provider := range []string{"gcp", "aws", "azure"} {
		t.Run(provider, func(t *testing.T) {
			result := cliCloudFixture(provider)
			payload := map[string]any{"name": "CLI production", "result": result}
			expect(t, call("POST", "/api/v1/ingest/cloud", payload, "", true), 401)
			response := call("POST", "/api/v1/ingest/cloud", payload, token, true)
			expect(t, response, 201)
			var out struct {
				ID string `json:"id"`
			}
			decodeBody(t, response, &out)
			path := "/api/v1/cloud/connections/" + out.ID
			expect(t, call("GET", path, nil, otherSession, false), 404)
			expect(t, call("GET", path, nil, session, false), 200)
			expect(t, call("GET", path+"/findings", nil, session, false), 200)
			checks := call("GET", path+"/checks", nil, session, false)
			expect(t, checks, 200)
			if !bytes.Contains(checks.Body.Bytes(), []byte(`"skipped":1`)) {
				t.Fatal("lost incomplete coverage")
			}
			expect(t, call("POST", path+"/scan", nil, session, false), 409)
			expect(t, call("PATCH", path+"/credentials", nil, session, false), 409)
			expect(t, call("POST", "/api/v1/ingest/cloud", payload, token, true), 200)
			id, _ := bson.ObjectIDFromHex(out.ID)
			count, err := s.scans.CountDocuments(ctx, bson.M{"workspace_id": workspace.ID, "cloud_connection_id": id})
			if err != nil || count != 1 {
				t.Fatal("retry created duplicate scan usage")
			}
			result.FinishedAt = result.FinishedAt.Add(-time.Second)
			payload["result"] = result
			expect(t, call("POST", "/api/v1/ingest/cloud", payload, token, true), 409)
			expect(t, call("DELETE", path, nil, otherSession, false), 404)
			expect(t, call("DELETE", path, nil, session, false), 200)
			count, err = s.scans.CountDocuments(ctx, bson.M{"cloud_connection_id": id, "cloud_result": bson.M{"$exists": true}})
			if err != nil || count != 0 {
				t.Fatal("cloud results survived removal")
			}
			// Usage history remains after result removal, matching normal scan accounting.
			count, err = s.scans.CountDocuments(context.Background(), bson.M{"cloud_connection_id": id})
			if err != nil || count != 1 {
				t.Fatal("removal modified usage ledger")
			}
		})
	}
}
