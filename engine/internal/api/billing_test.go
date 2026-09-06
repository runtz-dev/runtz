package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBillingSubscriptionAccountIdentity(t *testing.T) {
	user := User{ID: bson.NewObjectID(), Email: "owner@example.com"}
	for _, tc := range []struct {
		name    string
		sub     BillingSubscription
		matches bool
	}{
		{"linked_account", BillingSubscription{UserID: user.ID, Email: "billing@example.com"}, true},
		{"different_id_same_email", BillingSubscription{UserID: bson.NewObjectID(), Email: user.Email}, false},
		{"unlinked_matching_email", BillingSubscription{Email: " OWNER@example.com "}, true},
		{"unlinked_other_email", BillingSubscription{Email: "other@example.com"}, false},
		{"missing_identity", BillingSubscription{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := billingSubscriptionBelongsToUser(tc.sub, user); got != tc.matches {
				t.Fatalf("account match = %v, want %v", got, tc.matches)
			}
		})
	}
}

func TestVerifyStripeSignature(t *testing.T) {
	payload := []byte(`{"id":"evt_test"}`)
	secret := "whsec_test"
	timestamp := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, payload)))
	signature := hex.EncodeToString(mac.Sum(nil))

	header := fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
	if !verifyStripeSignature(payload, header, secret, 5*time.Minute) {
		t.Fatal("verifyStripeSignature rejected a valid signature")
	}

	if verifyStripeSignature(payload, header, "wrong-secret", 5*time.Minute) {
		t.Fatal("verifyStripeSignature accepted an invalid signature")
	}
}

func TestStripeSubscriptionPeriodEnd(t *testing.T) {
	// Payload shape up to 2025-02-24.acacia.
	var legacy stripeSubscription
	if err := json.Unmarshal([]byte(`{"id":"sub_1","current_period_end":1790000000}`), &legacy); err != nil {
		t.Fatalf("decode legacy subscription: %v", err)
	}
	if legacy.periodEnd() != 1790000000 {
		t.Fatalf("legacy period end = %d", legacy.periodEnd())
	}

	// Payload shape from 2025-03-31.basil on: the period lives on the items.
	var basil stripeSubscription
	if err := json.Unmarshal([]byte(`{"id":"sub_2","items":{"data":[{"current_period_end":1790000001,"price":{"id":"price_1"}}]}}`), &basil); err != nil {
		t.Fatalf("decode basil subscription: %v", err)
	}
	if basil.periodEnd() != 1790000001 {
		t.Fatalf("item period end = %d", basil.periodEnd())
	}

	var empty stripeSubscription
	if empty.periodEnd() != 0 {
		t.Fatal("an empty subscription must not report a renewal date")
	}
}

func TestFeaturesForPlan(t *testing.T) {
	// Google sign-in is available on every plan/mode; GitHub stays cloud-only.
	freeSelfHosted := featuresForPlan(planFree, hostingSelfHosted)
	if !featureEnabled(freeSelfHosted, featureGoogleAuth) {
		t.Fatal("self-hosted free should include google auth")
	}
	if featureEnabled(freeSelfHosted, featureGitHubAuth) {
		t.Fatal("self-hosted free should not include github auth")
	}

	proSelfHosted := featuresForPlan(planPro, hostingSelfHosted)
	if !featureEnabled(proSelfHosted, featureGoogleAuth) || !featureEnabled(proSelfHosted, featureAIAlertAgent) {
		t.Fatal("self-hosted pro should include google auth and ai alert agent")
	}
	if featureEnabled(proSelfHosted, featureGitHubAuth) {
		t.Fatal("self-hosted pro should not include github auth")
	}

	freeCloud := featuresForPlan(planFree, hostingCloud)
	if !featureEnabled(freeCloud, featureGoogleAuth) || !featureEnabled(freeCloud, featureGitHubAuth) {
		t.Fatal("cloud free should include both google and github auth")
	}
}

func TestWorkspaceAndUserLimitForPlan(t *testing.T) {
	if got := workspaceLimitForPlan(planFree); got != freeWorkspaceLimit {
		t.Fatalf("free workspace limit = %d, want %d", got, freeWorkspaceLimit)
	}
	if got := workspaceLimitForPlan(planPro); got != proWorkspaceLimit {
		t.Fatalf("pro workspace limit = %d, want %d", got, proWorkspaceLimit)
	}
	if got := workspaceLimitForPlan(planEnterprise); got != unlimitedLimit {
		t.Fatalf("enterprise workspace limit = %d, want unlimited", got)
	}

	if got := userLimitForPlan(planFree, hostingCloud); got != freeUserLimitCloud {
		t.Fatalf("cloud free user limit = %d, want %d", got, freeUserLimitCloud)
	}
	if got := userLimitForPlan(planFree, hostingSelfHosted); got != freeUserLimitSelfHosted {
		t.Fatalf("self-hosted free user limit = %d, want %d", got, freeUserLimitSelfHosted)
	}
	if got := userLimitForPlan(planPro, hostingCloud); got != proUserLimit {
		t.Fatalf("cloud pro user limit = %d, want %d", got, proUserLimit)
	}
	if got := userLimitForPlan(planPro, hostingSelfHosted); got != proUserLimit {
		t.Fatalf("self-hosted pro user limit = %d, want %d", got, proUserLimit)
	}
	if got := userLimitForPlan(planEnterprise, hostingCloud); got != unlimitedLimit {
		t.Fatalf("enterprise user limit = %d, want unlimited", got)
	}
}
