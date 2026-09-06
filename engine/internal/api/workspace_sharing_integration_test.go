package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCloudWorkspaceSharing(t *testing.T) {
	uri := os.Getenv("RUNTZ_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set RUNTZ_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s, err := New(ctx, config.Config{DeploymentMode: hostingCloud, MongoURI: uri, MongoDatabase: "runtz_test_" + bson.NewObjectID().Hex()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	defer s.db.Drop(context.Background())
	newUser := func(email string) (User, Workspace, string) {
		t.Helper()
		user, workspaces, err := s.findOrCreateEmailUser(ctx, email)
		if err != nil || len(workspaces) != 1 {
			t.Fatalf("signup: %v, %v", workspaces, err)
		}
		token, err := s.issueSession(ctx, user, httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		return user, workspaces[0], token
	}
	owner, workspace, ownerToken := newUser("sharing-owner@gmail.com")
	member, personal, memberToken := newUser("sharing-member@gmail.com")
	_, _, strangerToken := newUser("sharing-stranger@gmail.com")
	endpoint := "/api/v1/workspaces/" + workspace.ID.Hex() + "/members"
	request := func(token, method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: HTTP %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	memberBody := `{"email":" SHARING-MEMBER@gmail.com "}`
	request("", http.MethodGet, endpoint, "", http.StatusUnauthorized)
	request(ownerToken, http.MethodPost, endpoint, memberBody, http.StatusPaymentRequired)
	request(strangerToken, http.MethodGet, endpoint, "", http.StatusNotFound)
	request(strangerToken, http.MethodPost, endpoint, memberBody, http.StatusNotFound)
	request(strangerToken, http.MethodDelete, endpoint+"/"+member.ID.Hex(), "", http.StatusNotFound)
	subscription := BillingSubscription{ID: bson.NewObjectID(), UserID: owner.ID, Plan: planPro, DeploymentMode: hostingCloud, Status: "active"}
	if _, err := s.billingSubscriptions.InsertOne(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	request(ownerToken, http.MethodPost, endpoint, `{"email":"invalid"}`, http.StatusBadRequest)
	request(ownerToken, http.MethodPost, endpoint, `{"email":"unknown@gmail.com"}`, http.StatusNotFound)
	request(memberToken, http.MethodGet, endpoint, "", http.StatusNotFound)
	request(memberToken, http.MethodGet, "/api/v1/api-keys?workspaceId="+workspace.ID.Hex(), "", http.StatusForbidden)
	request(ownerToken, http.MethodPost, endpoint, memberBody, http.StatusOK)
	request(ownerToken, http.MethodPost, endpoint, memberBody, http.StatusOK)
	list := request(ownerToken, http.MethodGet, endpoint, "", http.StatusOK)
	var response struct {
		Members []workspaceMember `json:"members"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Members) != 2 {
		t.Fatalf("duplicate membership: %s", list.Body.String())
	}
	for _, item := range response.Members {
		if item.ID == owner.ID.Hex() && item.Role != "owner" {
			t.Fatal("owner role missing")
		}
	}
	if strings.Contains(list.Body.String(), "workspaceIds") || strings.Contains(list.Body.String(), "password") {
		t.Fatal("member response exposes private fields")
	}
	visible := request(memberToken, http.MethodGet, "/api/v1/workspaces", "", http.StatusOK)
	if !strings.Contains(visible.Body.String(), workspace.ID.Hex()) || !strings.Contains(visible.Body.String(), personal.ID.Hex()) {
		t.Fatal("member must see both shared and personal workspaces")
	}
	request(memberToken, http.MethodGet, "/api/v1/api-keys?workspaceId="+workspace.ID.Hex(), "", http.StatusOK)
	memberList := request(memberToken, http.MethodGet, endpoint, "", http.StatusOK)
	if memberList.Body.String() != list.Body.String() {
		t.Fatal("members must see the same roster as the owner")
	}
	request(memberToken, http.MethodPost, endpoint, memberBody, http.StatusNotFound)
	request(memberToken, http.MethodDelete, endpoint+"/"+owner.ID.Hex(), "", http.StatusNotFound)
	request(ownerToken, http.MethodDelete, endpoint+"/"+owner.ID.Hex(), "", http.StatusBadRequest)
	request(ownerToken, http.MethodDelete, endpoint+"/invalid", "", http.StatusBadRequest)
	keyResponse := request(memberToken, http.MethodPost, "/api/v1/api-keys", `{"name":"shared key","workspaceId":"`+workspace.ID.Hex()+`"}`, http.StatusCreated)
	var key struct {
		Token string `json:"key"`
	}
	if err := json.Unmarshal(keyResponse.Body.Bytes(), &key); err != nil {
		t.Fatal(err)
	}
	// The member's key must stop authenticating after removal.
	if key.Token == "" {
		t.Fatalf("key not returned: %s", keyResponse.Body.String())
	}
	if _, _, err := s.workspaceFromAPIKey(ctx, key.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.billingSubscriptions.UpdateOne(ctx, bson.M{"_id": subscription.ID}, bson.M{"$set": bson.M{"status": "canceled"}}); err != nil {
		t.Fatal(err)
	}
	request(ownerToken, http.MethodGet, endpoint, "", http.StatusOK)
	request(ownerToken, http.MethodDelete, endpoint+"/"+member.ID.Hex(), "", http.StatusOK)
	request(memberToken, http.MethodGet, endpoint, "", http.StatusNotFound)
	request(memberToken, http.MethodGet, "/api/v1/api-keys?workspaceId="+workspace.ID.Hex(), "", http.StatusForbidden)
	if _, _, err := s.workspaceFromAPIKey(ctx, key.Token); err == nil {
		t.Fatal("removed member key still works")
	}
	visible = request(memberToken, http.MethodGet, "/api/v1/workspaces", "", http.StatusOK)
	if strings.Contains(visible.Body.String(), workspace.ID.Hex()) || !strings.Contains(visible.Body.String(), personal.ID.Hex()) {
		t.Fatal("removal must affect only the shared workspace")
	}
	request(ownerToken, http.MethodPost, endpoint, memberBody, http.StatusPaymentRequired)
	if _, err := s.billingSubscriptions.UpdateOne(ctx, bson.M{"_id": subscription.ID}, bson.M{"$set": bson.M{"status": "active"}}); err != nil {
		t.Fatal(err)
	}
	// Fill the account's remaining seats; Pro must reject one more distinct user.
	for i := 0; i < 49; i++ {
		_, err := s.users.InsertOne(ctx, User{ID: bson.NewObjectID(), Username: fmt.Sprintf("seat-%d", i), Email: fmt.Sprintf("seat-%d@gmail.com", i), WorkspaceIDs: []bson.ObjectID{workspace.ID}})
		if err != nil {
			t.Fatal(err)
		}
	}
	request(ownerToken, http.MethodPost, endpoint, memberBody, http.StatusPaymentRequired)
	second := Workspace{ID: bson.NewObjectID(), Name: "second", Slug: "sharing-second", CreatedBy: owner.ID}
	if _, err := s.workspaces.InsertOne(ctx, second); err != nil {
		t.Fatal(err)
	}
	request(ownerToken, http.MethodPost, "/api/v1/workspaces/"+second.ID.Hex()+"/members", `{"email":"seat-0@gmail.com"}`, http.StatusOK)
	// Enterprise removes the Pro seat cap.
	if _, err := s.billingSubscriptions.UpdateOne(ctx, bson.M{"_id": subscription.ID}, bson.M{"$set": bson.M{"plan": planEnterprise}}); err != nil {
		t.Fatal(err)
	}
	request(ownerToken, http.MethodPost, endpoint, memberBody, http.StatusOK)
	s.cfg.DeploymentMode = hostingSelfHosted
	request(ownerToken, http.MethodGet, endpoint, "", http.StatusNotFound)
}
