package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

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
	freeSelfHosted := featuresForPlan(planFree, hostingSelfHosted)
	if featureEnabled(freeSelfHosted, featureGoogleGitHubAuth) {
		t.Fatal("self-hosted free should not include google/github auth")
	}

	proSelfHosted := featuresForPlan(planPro, hostingSelfHosted)
	if !featureEnabled(proSelfHosted, featureGoogleGitHubAuth) || !featureEnabled(proSelfHosted, featureAIAlertAgent) {
		t.Fatal("self-hosted pro should include oauth and ai alert agent")
	}
	if featureEnabled(proSelfHosted, featureMultipleWorkspaces) {
		t.Fatal("self-hosted pro should not include multiple workspaces")
	}

	enterprise := featuresForPlan(planEnterprise, hostingCloud)
	if !featureEnabled(enterprise, featureMultipleWorkspaces) {
		t.Fatal("enterprise should include multiple workspaces")
	}
}
