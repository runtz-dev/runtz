// Paid-plan licensing. Self-hosted Pro/Enterprise is gated by Ed25519 license
// certificates issued by the central engine (https://engine.runtz.dev) and
// verified against the public key compiled into every binary
// (config.LicenseVerificationKey). Removing, weakening or bypassing this
// verification to unlock paid features without an active subscription breaches
// the license (BUSL-1.1); see NOTICE.
package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	centralEngineURL      = "https://engine.runtz.dev"
	licenseCertificateTTL = 7 * 24 * time.Hour
	licenseHeartbeatEvery = 12 * time.Hour
)

func centralEngineReachabilityError(err error) error {
	return fmt.Errorf(
		"To activate or validate Pro/Enterprise, this installation must be able to connect to %s. Try again when the URL is reachable. Technical details: %v",
		centralEngineURL,
		err,
	)
}

type licenseValidationRequest struct {
	LicenseKey     string `json:"licenseKey"`
	InstallationID string `json:"installationId"`
}

type checkoutLicenseActivationRequest struct {
	SessionID      string `json:"sessionId"`
	InstallationID string `json:"installationId"`
}

type licenseValidationResponse struct {
	License   LicensePayload `json:"license"`
	Payload   string         `json:"payload"`
	Signature string         `json:"signature"`
}

func (s *Server) startLicenseHeartbeat(ctx context.Context) {
	if s.cfg.DeploymentMode != hostingSelfHosted {
		return
	}

	ticker := time.NewTicker(licenseHeartbeatEvery)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.refreshStoredSelfHostedLicense(ctx)
			}
		}
	}()
}

func (s *Server) handleActivateSelfHostedLicense(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingSelfHosted {
		writeError(w, http.StatusNotFound, "license activation is only available in self-hosted mode")
		return
	}

	var request struct {
		LicenseKey string `json:"licenseKey"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	licenseKey := strings.TrimSpace(request.LicenseKey)
	if licenseKey == "" {
		writeError(w, http.StatusBadRequest, "licenseKey is required")
		return
	}

	state, err := s.ensureInstanceState(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare installation state")
		return
	}

	updated, err := s.validateAndStoreSelfHostedLicense(r.Context(), licenseKey, state.InstallationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entitlement": s.selfHostedEntitlement(r.Context()),
		"instance":    serializeInstanceState(updated),
	})
}

func (s *Server) handleRefreshSelfHostedLicense(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingSelfHosted {
		writeError(w, http.StatusNotFound, "license refresh is only available in self-hosted mode")
		return
	}

	if err := s.refreshStoredSelfHostedLicense(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	state, _ := s.getInstanceState(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"entitlement": s.selfHostedEntitlement(r.Context()),
		"instance":    serializeInstanceState(state),
	})
}

func (s *Server) handleActivateSelfHostedCheckout(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingSelfHosted {
		writeError(w, http.StatusNotFound, "checkout activation is only available in self-hosted mode")
		return
	}

	var request struct {
		SessionID string `json:"sessionId"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	state, err := s.ensureInstanceState(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare installation state")
		return
	}

	validation, err := s.requestCheckoutLicenseActivation(r.Context(), sessionID, state.InstallationID)
	if err != nil {
		_ = s.storeLicenseValidationError(r.Context(), err.Error())
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	updated, err := s.storeValidatedLicense(r.Context(), "", sessionID, state.InstallationID, validation)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entitlement": s.selfHostedEntitlement(r.Context()),
		"instance":    serializeInstanceState(updated),
	})
}

func (s *Server) handleActivateCheckoutLicense(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingCloud {
		writeError(w, http.StatusNotFound, "checkout license activation is only available on the central engine")
		return
	}
	if strings.TrimSpace(s.cfg.LicensePrivateKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "license signing key is not configured")
		return
	}

	var request checkoutLicenseActivationRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	sessionID := strings.TrimSpace(request.SessionID)
	installationID := strings.TrimSpace(request.InstallationID)
	if sessionID == "" || installationID == "" {
		writeError(w, http.StatusBadRequest, "sessionId and installationId are required")
		return
	}

	subscription, err := s.ensureCheckoutSessionStored(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "checkout session not found")
		return
	}
	if subscription.DeploymentMode != hostingSelfHosted {
		writeError(w, http.StatusBadRequest, "checkout session is not for self-hosted")
		return
	}
	if !subscriptionStatusActive(subscription.Status) {
		writeError(w, http.StatusPaymentRequired, "subscription is not active")
		return
	}
	if subscription.InstallationID != "" && subtle.ConstantTimeCompare([]byte(subscription.InstallationID), []byte(installationID)) != 1 {
		writeError(w, http.StatusConflict, "license is already activated on another installation")
		return
	}

	validation, err := s.issueSignedLicense(r.Context(), subscription, installationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, validation)
}

func (s *Server) handleValidateLicense(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != hostingCloud {
		writeError(w, http.StatusNotFound, "license validation is only available on the central engine")
		return
	}
	if strings.TrimSpace(s.cfg.LicensePrivateKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "license signing key is not configured")
		return
	}

	var request licenseValidationRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	licenseKey := strings.TrimSpace(request.LicenseKey)
	installationID := strings.TrimSpace(request.InstallationID)
	if licenseKey == "" || installationID == "" {
		writeError(w, http.StatusBadRequest, "licenseKey and installationId are required")
		return
	}

	var subscription BillingSubscription
	err := s.billingSubscriptions.FindOne(r.Context(), bson.M{
		"deployment_mode":  hostingSelfHosted,
		"license_key_hash": hashSecret(licenseKey),
	}).Decode(&subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeError(w, http.StatusUnauthorized, "invalid license key")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read license")
		return
	}
	if !subscriptionStatusActive(subscription.Status) {
		writeError(w, http.StatusPaymentRequired, "subscription is not active")
		return
	}
	if subscription.InstallationID != "" && subtle.ConstantTimeCompare([]byte(subscription.InstallationID), []byte(installationID)) != 1 {
		writeError(w, http.StatusConflict, "license is already activated on another installation")
		return
	}

	validation, err := s.issueSignedLicense(r.Context(), subscription, installationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, validation)
}

// verifyLicenseSignature checks an Ed25519 license certificate against the
// compiled-in verification key (config.LicenseVerificationKey). It fails closed:
// a missing, malformed or non-matching key is a verification failure, never a
// reason to skip verification. This is the control that stops a self-hosted
// installation from activating Pro/Enterprise by pointing the central-engine URL
// at a look-alike server or by editing stored state. Do not reintroduce a
// "verify only when a key is configured" branch — see NOTICE.
func (s *Server) verifyLicenseSignature(payload, signature []byte) error {
	publicKey, err := parseEd25519PublicKey(s.cfg.LicensePublicKey)
	if err != nil {
		return errors.New("license verification key is unavailable")
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return errors.New("license signature verification failed")
	}
	return nil
}

// verifiedStoredLicense re-checks the license certificate persisted in the
// instance state on every read, so tampering with the stored license_payload
// (for example editing the plan directly in MongoDB) is rejected. The authentic
// payload is the one recovered from the signed bytes, not the convenience copy
// stored alongside it.
func (s *Server) verifiedStoredLicense(state InstanceState) (LicensePayload, bool) {
	rawPayload := strings.TrimSpace(state.LicensePayloadRaw)
	rawSignature := strings.TrimSpace(state.LicenseSignature)
	if rawPayload == "" || rawSignature == "" {
		return LicensePayload{}, false
	}
	payloadBytes, err := base64.StdEncoding.DecodeString(rawPayload)
	if err != nil {
		return LicensePayload{}, false
	}
	signature, err := base64.StdEncoding.DecodeString(rawSignature)
	if err != nil {
		return LicensePayload{}, false
	}
	if err := s.verifyLicenseSignature(payloadBytes, signature); err != nil {
		return LicensePayload{}, false
	}
	var payload LicensePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return LicensePayload{}, false
	}
	if payload.InstallationID != state.InstallationID {
		return LicensePayload{}, false
	}
	return payload, true
}

func (s *Server) validateAndStoreSelfHostedLicense(ctx context.Context, licenseKey, installationID string) (InstanceState, error) {
	response, err := s.requestLicenseValidation(ctx, licenseKey, installationID)
	if err != nil {
		_ = s.storeLicenseValidationError(ctx, err.Error())
		return InstanceState{}, err
	}

	return s.storeValidatedLicense(ctx, licenseKey, "", installationID, response)
}

func (s *Server) storeValidatedLicense(ctx context.Context, licenseKey, checkoutSessionID, installationID string, response licenseValidationResponse) (InstanceState, error) {
	payloadBytes, err := base64.StdEncoding.DecodeString(response.Payload)
	if err != nil {
		_ = s.storeLicenseValidationError(ctx, "invalid license payload")
		return InstanceState{}, errors.New("central engine returned an invalid license payload")
	}
	signature, err := base64.StdEncoding.DecodeString(response.Signature)
	if err != nil {
		_ = s.storeLicenseValidationError(ctx, "invalid license signature")
		return InstanceState{}, errors.New("central engine returned an invalid license signature")
	}
	if err := s.verifyLicenseSignature(payloadBytes, signature); err != nil {
		_ = s.storeLicenseValidationError(ctx, err.Error())
		return InstanceState{}, err
	}

	var payload LicensePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		_ = s.storeLicenseValidationError(ctx, "invalid license payload json")
		return InstanceState{}, errors.New("central engine returned invalid license json")
	}
	if payload.InstallationID != installationID {
		_ = s.storeLicenseValidationError(ctx, "license installation mismatch")
		return InstanceState{}, errors.New("license installation mismatch")
	}

	now := time.Now().UTC()
	set := bson.M{
		"installation_id":       installationID,
		"license_key_prefix":    payload.LicenseKeyPrefix,
		"license_payload":       payload,
		"license_payload_raw":   response.Payload,
		"license_signature":     response.Signature,
		"last_validated_at":     now,
		"last_validation_error": "",
		"updated_at":            now,
	}
	if strings.TrimSpace(licenseKey) != "" {
		set["license_key"] = licenseKey
	}
	if strings.TrimSpace(checkoutSessionID) != "" {
		set["checkout_session_id"] = checkoutSessionID
	}

	result := s.instanceState.FindOneAndUpdate(
		ctx,
		bson.M{"key": instanceStateKey},
		bson.M{
			"$set": set,
			"$setOnInsert": bson.M{
				"_id":        bson.NewObjectID(),
				"key":        instanceStateKey,
				"created_at": now,
			},
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	)

	var state InstanceState
	if err := result.Decode(&state); err != nil {
		return InstanceState{}, err
	}
	return state, nil
}

func (s *Server) requestLicenseValidation(ctx context.Context, licenseKey, installationID string) (licenseValidationResponse, error) {
	payload, err := json.Marshal(licenseValidationRequest{
		LicenseKey:     licenseKey,
		InstallationID: installationID,
	})
	if err != nil {
		return licenseValidationResponse{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, centralEngineURL+"/api/v1/licenses/validate", bytes.NewReader(payload))
	if err != nil {
		return licenseValidationResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return licenseValidationResponse{}, centralEngineReachabilityError(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return licenseValidationResponse{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var apiError struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &apiError)
		if apiError.Error != "" {
			return licenseValidationResponse{}, errors.New(apiError.Error)
		}
		return licenseValidationResponse{}, fmt.Errorf("license validation failed with status %d", response.StatusCode)
	}

	var validation licenseValidationResponse
	if err := json.Unmarshal(body, &validation); err != nil {
		return licenseValidationResponse{}, err
	}
	return validation, nil
}

func (s *Server) requestCheckoutLicenseActivation(ctx context.Context, sessionID, installationID string) (licenseValidationResponse, error) {
	payload, err := json.Marshal(checkoutLicenseActivationRequest{
		SessionID:      sessionID,
		InstallationID: installationID,
	})
	if err != nil {
		return licenseValidationResponse{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, centralEngineURL+"/api/v1/licenses/activate-checkout", bytes.NewReader(payload))
	if err != nil {
		return licenseValidationResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return licenseValidationResponse{}, centralEngineReachabilityError(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return licenseValidationResponse{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var apiError struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &apiError)
		if apiError.Error != "" {
			return licenseValidationResponse{}, errors.New(apiError.Error)
		}
		return licenseValidationResponse{}, fmt.Errorf("checkout license activation failed with status %d", response.StatusCode)
	}

	var validation licenseValidationResponse
	if err := json.Unmarshal(body, &validation); err != nil {
		return licenseValidationResponse{}, err
	}
	return validation, nil
}

func (s *Server) issueSignedLicense(ctx context.Context, subscription BillingSubscription, installationID string) (licenseValidationResponse, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(licenseCertificateTTL)
	currentPeriodEnd := ""
	if subscription.CurrentPeriodEnd != nil {
		currentPeriodEnd = subscription.CurrentPeriodEnd.Format(time.RFC3339)
		graceEnd := subscription.CurrentPeriodEnd.Add(licenseCertificateTTL)
		if graceEnd.Before(expiresAt) {
			expiresAt = graceEnd
		}
	}

	payload := LicensePayload{
		LicenseKeyPrefix: subscription.LicenseKeyPrefix,
		InstallationID:   installationID,
		Plan:             normalizePlan(subscription.Plan),
		DeploymentMode:   hostingSelfHosted,
		Status:           subscription.Status,
		Features:         featuresForPlan(subscription.Plan, hostingSelfHosted),
		IssuedAt:         now,
		ExpiresAt:        expiresAt,
		CurrentPeriodEnd: currentPeriodEnd,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return licenseValidationResponse{}, errors.New("failed to encode license")
	}

	privateKey, err := parseEd25519PrivateKey(s.cfg.LicensePrivateKey)
	if err != nil {
		return licenseValidationResponse{}, errors.New("license signing key is invalid")
	}
	signature := ed25519.Sign(privateKey, payloadBytes)

	_, err = s.billingSubscriptions.UpdateOne(ctx, bson.M{"_id": subscription.ID}, bson.M{
		"$set": bson.M{
			"installation_id":   installationID,
			"last_heartbeat_at": now,
			"updated_at":        now,
		},
	})
	if err != nil {
		return licenseValidationResponse{}, errors.New("failed to update license heartbeat")
	}

	return licenseValidationResponse{
		License:   payload,
		Payload:   base64.StdEncoding.EncodeToString(payloadBytes),
		Signature: base64.StdEncoding.EncodeToString(signature),
	}, nil
}

func (s *Server) refreshStoredSelfHostedLicense(ctx context.Context) error {
	state, err := s.getInstanceState(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(state.LicenseKey) != "" {
		_, err = s.validateAndStoreSelfHostedLicense(ctx, state.LicenseKey, state.InstallationID)
		return err
	}
	if strings.TrimSpace(state.CheckoutSessionID) == "" {
		return errors.New("no self-hosted license has been activated")
	}
	validation, err := s.requestCheckoutLicenseActivation(ctx, state.CheckoutSessionID, state.InstallationID)
	if err != nil {
		_ = s.storeLicenseValidationError(ctx, err.Error())
		return err
	}
	_, err = s.storeValidatedLicense(ctx, "", state.CheckoutSessionID, state.InstallationID, validation)
	return err
}

func (s *Server) ensureInstanceState(ctx context.Context) (InstanceState, error) {
	state, err := s.getInstanceState(ctx)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return InstanceState{}, err
	}

	installationID, err := generateInstallationID()
	if err != nil {
		return InstanceState{}, err
	}
	now := time.Now().UTC()
	state = InstanceState{
		ID:             bson.NewObjectID(),
		Key:            instanceStateKey,
		InstallationID: installationID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_, err = s.instanceState.InsertOne(ctx, state)
	return state, err
}

func (s *Server) getInstanceState(ctx context.Context) (InstanceState, error) {
	var state InstanceState
	err := s.instanceState.FindOne(ctx, bson.M{"key": instanceStateKey}).Decode(&state)
	return state, err
}

func (s *Server) storeLicenseValidationError(ctx context.Context, message string) error {
	now := time.Now().UTC()
	_, err := s.instanceState.UpdateOne(ctx, bson.M{"key": instanceStateKey}, bson.M{
		"$set": bson.M{
			"last_validation_error": message,
			"updated_at":            now,
		},
	})
	return err
}

func generateInstallationID() (string, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return "rti_" + value, nil
}

func parseEd25519PrivateKey(value string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, fmt.Errorf("ed25519 private key must be %d or %d bytes", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

func parseEd25519PublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

func serializeInstanceState(state InstanceState) map[string]any {
	response := map[string]any{
		"installationId":   state.InstallationID,
		"licenseKeyPrefix": state.LicenseKeyPrefix,
	}
	if state.LastValidatedAt != nil {
		response["lastValidatedAt"] = state.LastValidatedAt.Format(time.RFC3339)
	}
	if state.LastValidationError != "" {
		response["lastValidationError"] = state.LastValidationError
	}
	if !state.LicensePayload.ExpiresAt.IsZero() {
		response["license"] = state.LicensePayload
	}
	return response
}
