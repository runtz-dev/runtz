package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
