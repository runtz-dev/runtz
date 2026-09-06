package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func billingTestServer(t *testing.T) (*Server, context.Context) {
	t.Helper()
	uri := os.Getenv("RUNTZ_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set RUNTZ_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	s, err := New(ctx, config.Config{DeploymentMode: hostingCloud, MongoURI: uri, MongoDatabase: "runtz_test_" + bson.NewObjectID().Hex(), StripeSecretKey: "sk_test_fixture"})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer cancel()
		defer s.Close(context.Background())
		if err := s.db.Drop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return s, ctx
}

type billingTestTransport func(*http.Request) (*http.Response, error)

func (f billingTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockBillingStripe(t *testing.T, session stripeCheckoutSession, sub stripeSubscription, failPath string) {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = billingTestTransport(func(r *http.Request) (*http.Response, error) {
		status := http.StatusOK
		var payload any
		switch r.URL.Path {
		case "/v1/checkout/sessions/" + session.ID:
			payload = session
		case "/v1/subscriptions/" + sub.ID:
			payload = sub
		default:
			t.Errorf("unexpected Stripe request: %s", r.URL.Path)
			status = http.StatusNotFound
		}
		if r.URL.Path == failPath {
			status = http.StatusServiceUnavailable
		}
		body, _ := json.Marshal(payload)
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
}

func billingFixtures() (User, stripeCheckoutSession, stripeSubscription) {
	user := User{ID: bson.NewObjectID(), Email: "billing@example.com"}
	metadata := map[string]string{"plan": planPro, "deployment_mode": hostingCloud, "user_id": user.ID.Hex()}
	session := stripeCheckoutSession{ID: "cs_test_completed", Subscription: "sub_test_paid", Customer: "cus_test", Status: "complete", PaymentStatus: "paid", Metadata: metadata}
	session.CustomerDetails.Email = user.Email
	sub := stripeSubscription{ID: session.Subscription, Customer: session.Customer, Status: "active", Metadata: metadata, CurrentPeriodEnd: time.Now().Add(30 * 24 * time.Hour).Unix()}
	return user, session, sub
}

func insertPendingCheckout(t *testing.T, s *Server, ctx context.Context, user User, session stripeCheckoutSession) bson.ObjectID {
	t.Helper()
	id := bson.NewObjectID()
	_, err := s.billingSubscriptions.InsertOne(ctx, bson.M{"_id": id, "stripe_checkout_session_id": session.ID, "user_id": user.ID, "email": user.Email, "plan": planPro, "deployment_mode": hostingCloud, "status": "checkout_pending"})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestBillingCheckoutReconciliation(t *testing.T) {
	for _, order := range []string{"checkout_first", "subscription_first", "concurrent"} {
		t.Run(order, func(t *testing.T) {
			s, ctx := billingTestServer(t)
			user, session, sub := billingFixtures()
			mockBillingStripe(t, session, sub, "")
			pendingID := insertPendingCheckout(t, s, ctx, user, session)
			saveSubscription := func() error { return s.storeStripeSubscription(ctx, sub, "", "", "") }
			saveCheckout := func() error { return s.storeCheckoutSession(ctx, session) }
			if order == "subscription_first" {
				if err := saveSubscription(); err != nil {
					t.Fatal(err)
				}
			}
			if order == "concurrent" {
				var wg sync.WaitGroup
				for i := 0; i < 12; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						var err error
						if i%2 == 0 {
							err = saveSubscription()
						} else {
							err = saveCheckout()
						}
						if err != nil {
							t.Error(err)
						}
					}(i)
				}
				wg.Wait()
			}
			for i := 0; i < 3; i++ {
				if err := saveCheckout(); err != nil {
					t.Fatal(err)
				}
				if err := saveSubscription(); err != nil {
					t.Fatal(err)
				}
			}
			stored, err := s.ensureCheckoutSessionStored(ctx, session.ID)
			if err != nil || stored.Status != "active" || stored.UserID != user.ID || stored.Email != user.Email {
				t.Fatalf("confirmed checkout: %+v, %v", stored, err)
			}
			if order == "checkout_first" && stored.ID != pendingID {
				t.Fatal("checkout was not promoted in place")
			}
			count, err := s.billingSubscriptions.CountDocuments(ctx, bson.M{})
			if err != nil || count != 1 {
				t.Fatalf("billing rows = %d, err=%v", count, err)
			}
			if ent := s.cloudEntitlement(ctx, &user); ent.Plan != planPro || ent.Status != "active" {
				t.Fatalf("entitlement = %+v", ent)
			}
			// An old checkout response must not overwrite the active subscription.
			stale := session
			stale.Subscription, stale.Status = "", "open"
			if err := s.storeCheckoutSession(ctx, stale); err != nil {
				t.Fatal(err)
			}
			if ent := s.cloudEntitlement(ctx, &user); ent.Plan != planPro {
				t.Fatal("stale checkout downgraded user")
			}
			// Normal subscription cancellation must still revoke entitlement.
			sub.Status = "canceled"
			if err := s.storeStripeSubscription(ctx, sub, "", "", ""); err != nil {
				t.Fatal(err)
			}
			if ent := s.cloudEntitlement(ctx, &user); ent.Plan != planFree {
				t.Fatal("canceled subscription retained Pro")
			}
		})
	}
}

func TestBillingCheckoutSyncFailures(t *testing.T) {
	for _, failure := range []string{"checkout_api", "subscription_api", "database"} {
		t.Run(failure, func(t *testing.T) {
			s, ctx := billingTestServer(t)
			user, session, sub := billingFixtures()
			insertPendingCheckout(t, s, ctx, user, session)
			failPath := ""
			if failure == "checkout_api" {
				failPath = "/v1/checkout/sessions/" + session.ID
			}
			if failure == "subscription_api" {
				failPath = "/v1/subscriptions/" + sub.ID
			}
			mockBillingStripe(t, session, sub, failPath)
			if failure == "database" {
				err := s.db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "billing_subscriptions"}, {Key: "validator", Value: bson.M{"status": bson.M{"$ne": "active"}}}}).Err()
				if err != nil {
					t.Fatal(err)
				}
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/billing/checkout-session/"+session.ID, nil).WithContext(ctx))
			if w.Code != http.StatusBadGateway {
				t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
			}
			if ent := s.cloudEntitlement(ctx, &user); ent.Plan != planFree {
				t.Fatal("failed synchronization granted Pro")
			}
		})
	}
}

func TestBillingOpenCheckoutsAndSubscriptionIdentity(t *testing.T) {
	s, ctx := billingTestServer(t)
	user, session, sub := billingFixtures()
	mockBillingStripe(t, session, sub, "")
	for _, id := range []string{"cs_open_one", "cs_open_two"} {
		if err := s.storeCheckoutSession(ctx, stripeCheckoutSession{ID: id, Status: "open"}); err != nil {
			t.Fatal(err)
		}
	}
	// A missing session user_id must not mask the subscription metadata with
	// a zero ObjectID string (000000000000000000000000).
	session.Metadata = map[string]string{"plan": planPro, "deployment_mode": hostingCloud}
	if err := s.storeCheckoutSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	stored, err := s.ensureCheckoutSessionStored(ctx, session.ID)
	if err != nil || stored.UserID != user.ID {
		t.Fatalf("identity lost: %+v, %v", stored, err)
	}
}

func TestBillingCompletionBeforeCheckoutCreateReturns(t *testing.T) {
	s, ctx := billingTestServer(t)
	user, session, sub := billingFixtures()
	s.cfg.PublicURL = "https://billing.example.com"
	s.cfg.StripePriceProCloud = "price_test_pro"
	original := http.DefaultTransport
	http.DefaultTransport = billingTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/checkout/sessions" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		// Simulate a webhook finishing while the original API request is open.
		if err := s.storeStripeSubscription(ctx, sub, session.ID, user.Email, user.ID.Hex()); err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(session)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"plan":"pro","deploymentMode":"cloud"}`)).WithContext(ctx))
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	stored, err := s.ensureCheckoutSessionStored(ctx, session.ID)
	if err != nil || stored.Status != "active" || stored.UserID != user.ID {
		t.Fatalf("create request overwrote confirmed subscription: %+v, %v", stored, err)
	}
}

func TestBillingWebhookUsesCurrentSubscription(t *testing.T) {
	for _, currentStatus := range []string{"active", "canceled"} {
		t.Run(currentStatus, func(t *testing.T) {
			s, ctx := billingTestServer(t)
			user, session, sub := billingFixtures()
			s.cfg.StripeWebhookSecret = "whsec_fixture"
			stale := sub
			stale.Status = "incomplete"
			sub.Status = currentStatus
			mockBillingStripe(t, session, sub, "")
			body, _ := json.Marshal(map[string]any{"id": "evt_fixture", "type": "customer.subscription.created", "data": map[string]any{"object": stale}})
			timestamp := time.Now().Unix()
			mac := hmac.New(sha256.New, []byte(s.cfg.StripeWebhookSecret))
			fmt.Fprintf(mac, "%d.%s", timestamp, body)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/billing/webhook", strings.NewReader(string(body))).WithContext(ctx)
			r.Header.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil))))
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
			}
			stored, err := s.findBillingSubscriptionForUser(ctx, user)
			if err != nil || stored.Status != currentStatus {
				t.Fatalf("stale event won: %+v, %v", stored, err)
			}
		})
	}
}

func TestBillingCheckoutAccountMatch(t *testing.T) {
	s, ctx := billingTestServer(t)
	owner, session, sub := billingFixtures()
	sub.Metadata["plan"] = planEnterprise
	if err := s.storeStripeSubscription(ctx, sub, session.ID, owner.Email, owner.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	other := User{ID: bson.NewObjectID(), Email: "other@example.com"}
	for _, user := range []User{owner, other} {
		user.Username = user.ID.Hex()
		if _, err := s.users.InsertOne(ctx, user); err != nil {
			t.Fatal(err)
		}
	}
	// The other account is already paid: that alone must never count as
	// activation of the owner's Enterprise checkout.
	otherSub := stripeSubscription{ID: "sub_other_pro", Status: "active", Metadata: map[string]string{"plan": planPro, "deployment_mode": hostingCloud, "user_id": other.ID.Hex()}}
	if err := s.storeStripeSubscription(ctx, otherSub, "", other.Email, other.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	if ent := s.cloudEntitlement(ctx, &other); ent.Plan != planPro {
		t.Fatalf("other account plan = %s", ent.Plan)
	}
	for _, tc := range []struct {
		name    string
		user    *User
		matches bool
	}{
		{"owner", &owner, true},
		{"different_paid_account", &other, false},
		{"anonymous", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/billing/checkout-session/"+session.ID, nil).WithContext(ctx)
			if tc.user != nil {
				token, err := s.issueSession(ctx, *tc.user, r)
				if err != nil {
					t.Fatal(err)
				}
				r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
			}
			var response struct {
				Plan           string `json:"plan"`
				AccountMatches *bool  `json:"accountMatches"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Plan != planEnterprise || response.AccountMatches == nil || *response.AccountMatches != tc.matches {
				t.Fatalf("unexpected checkout response: %s", w.Body.String())
			}
		})
	}
}
