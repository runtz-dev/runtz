package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	stripeAPIBase    = "https://api.stripe.com/v1"
	stripeAPIVersion = "2026-02-25.clover"
)

type createCheckoutRequest struct {
	Plan           string `json:"plan"`
	DeploymentMode string `json:"deploymentMode"`
	Email          string `json:"email"`
	SuccessURL     string `json:"successUrl"`
	CancelURL      string `json:"cancelUrl"`
	InstallationID string `json:"installationId"`
}

type stripeCheckoutSession struct {
	ID              string            `json:"id"`
	URL             string            `json:"url"`
	Mode            string            `json:"mode"`
	Status          string            `json:"status"`
	PaymentStatus   string            `json:"payment_status"`
	Customer        string            `json:"customer"`
	Subscription    string            `json:"subscription"`
	ClientReference string            `json:"client_reference_id"`
	Metadata        map[string]string `json:"metadata"`
	CustomerDetails struct {
		Email string `json:"email"`
	} `json:"customer_details"`
}

type stripeSubscription struct {
	ID string `json:"id"`
	// Present up to API version 2025-02-24.acacia. From 2025-03-31.basil on,
	// Stripe moved the billing period onto each item — see periodEnd().
	CurrentPeriodEnd  int64             `json:"current_period_end"`
	Customer          string            `json:"customer"`
	Status            string            `json:"status"`
	CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
	Metadata          map[string]string `json:"metadata"`
	Items             struct {
		Data []struct {
			CurrentPeriodEnd int64 `json:"current_period_end"`
			Price            struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

// periodEnd is when the subscription renews, read from whichever shape the
// payload uses: the subscription-level field on old API versions, or the
// furthest item period on 2025-03-31.basil and later (our plans have a single
// item, so the two agree).
func (s stripeSubscription) periodEnd() int64 {
	periodEnd := s.CurrentPeriodEnd
	for _, item := range s.Items.Data {
		if item.CurrentPeriodEnd > periodEnd {
			periodEnd = item.CurrentPeriodEnd
		}
	}

	return periodEnd
}

type stripeBillingPortalSession struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type stripeHTTPError struct {
	StatusCode int
	Body       string
}

func (e stripeHTTPError) Error() string {
	return fmt.Sprintf("stripe request failed with status %d: %s", e.StatusCode, e.Body)
}

func (s *Server) handleCreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	var request createCheckoutRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	if s.cfg.DeploymentMode == hostingSelfHosted {
		user, ok := s.optionalUserFromSession(r)
		if !ok || user.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}

		state, err := s.ensureInstanceState(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to prepare installation state")
			return
		}
		request.DeploymentMode = hostingSelfHosted
		request.InstallationID = state.InstallationID
		if request.Email == "" {
			request.Email = user.Email
		}

		session, err := s.createCentralCheckoutSession(r.Context(), request)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"id":  session.ID,
			"url": session.URL,
		})
		return
	}

	if strings.TrimSpace(s.cfg.StripeSecretKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "stripe is not configured")
		return
	}

	plan := normalizePlan(request.Plan)
	if plan == planFree {
		writeError(w, http.StatusBadRequest, "paid plan is required")
		return
	}
	deploymentMode := normalizeHostingMode(request.DeploymentMode)
	priceID, ok := s.stripePriceID(plan, deploymentMode)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "stripe price is not configured for this plan")
		return
	}

	user, hasUser := s.optionalUserFromSession(r)
	email := strings.ToLower(strings.TrimSpace(request.Email))
	if email == "" && hasUser {
		email = strings.ToLower(strings.TrimSpace(user.Email))
	}

	successURL := strings.TrimSpace(request.SuccessURL)
	if successURL == "" {
		successURL = s.cfg.PublicURL + "/home/pricing/success?session_id={CHECKOUT_SESSION_ID}"
	}
	cancelURL := strings.TrimSpace(request.CancelURL)
	if cancelURL == "" {
		cancelURL = s.cfg.PublicURL + "/home/pricing"
	}
	if !s.checkoutReturnURLAllowed(deploymentMode, successURL) || !s.checkoutReturnURLAllowed(deploymentMode, cancelURL) {
		writeError(w, http.StatusBadRequest, "successUrl and cancelUrl must use an allowed origin")
		return
	}

	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("line_items[0][price]", priceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("allow_promotion_codes", "true")
	form.Set("billing_address_collection", "auto")
	form.Set("metadata[plan]", plan)
	form.Set("metadata[deployment_mode]", deploymentMode)
	form.Set("subscription_data[metadata][plan]", plan)
	form.Set("subscription_data[metadata][deployment_mode]", deploymentMode)
	if strings.TrimSpace(request.InstallationID) != "" {
		form.Set("metadata[installation_id]", strings.TrimSpace(request.InstallationID))
		form.Set("subscription_data[metadata][installation_id]", strings.TrimSpace(request.InstallationID))
	}
	if email != "" {
		form.Set("customer_email", email)
	}
	if hasUser {
		form.Set("client_reference_id", user.ID.Hex())
		form.Set("metadata[user_id]", user.ID.Hex())
		form.Set("subscription_data[metadata][user_id]", user.ID.Hex())
	}

	var session stripeCheckoutSession
	if err := s.stripePostForm(r.Context(), "/checkout/sessions", form, &session); err != nil {
		writeError(w, http.StatusBadGateway, "failed to create stripe checkout session")
		return
	}

	now := time.Now().UTC()
	// A completion webhook may arrive before this write. Never reset a
	// confirmed checkout to pending when the create request finishes later.
	record := bson.M{
		"$setOnInsert": bson.M{
			"_id":                        bson.NewObjectID(),
			"created_at":                 now,
			"email":                      email,
			"plan":                       plan,
			"deployment_mode":            deploymentMode,
			"status":                     "checkout_pending",
			"stripe_checkout_session_id": session.ID,
			"stripe_price_id":            priceID,
			"updated_at":                 now,
		},
	}
	if hasUser {
		record["$setOnInsert"].(bson.M)["user_id"] = user.ID
	}
	if strings.TrimSpace(request.InstallationID) != "" {
		record["$setOnInsert"].(bson.M)["installation_id"] = strings.TrimSpace(request.InstallationID)
	}
	_, err := s.billingSubscriptions.UpdateOne(
		r.Context(),
		bson.M{"stripe_checkout_session_id": session.ID},
		record,
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		slog.Error("store pending checkout", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to store checkout session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":  session.ID,
		"url": session.URL,
	})
}

func (s *Server) handleCreateBillingPortalSession(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.cfg.StripeSecretKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "stripe is not configured")
		return
	}

	user, _ := currentUser(r.Context())
	subscription, err := s.findBillingSubscriptionForUser(r.Context(), user)
	if err != nil || strings.TrimSpace(subscription.StripeCustomerID) == "" {
		writeError(w, http.StatusNotFound, "billing account not found")
		return
	}

	var request struct {
		ReturnURL string `json:"returnUrl"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	returnURL := strings.TrimSpace(request.ReturnURL)
	if returnURL == "" {
		returnURL = s.cfg.PublicURL + "/app/settings"
	}
	if !s.externalURLAllowed(returnURL) {
		writeError(w, http.StatusBadRequest, "returnUrl must use an allowed origin")
		return
	}

	form := url.Values{}
	form.Set("customer", subscription.StripeCustomerID)
	form.Set("return_url", returnURL)

	var session stripeBillingPortalSession
	if err := s.stripePostForm(r.Context(), "/billing_portal/sessions", form, &session); err != nil {
		writeError(w, http.StatusBadGateway, "failed to create stripe portal session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": session.URL})
}

func (s *Server) handleBillingStatus(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	response := map[string]any{
		"entitlement": s.currentEntitlement(r.Context(), &user),
	}
	if s.cfg.DeploymentMode == hostingCloud {
		if subscription, err := s.findBillingSubscriptionForUser(r.Context(), user); err == nil {
			response["subscription"] = serializeBillingSubscription(subscription, false)
		}
	} else if state, err := s.getInstanceState(r.Context()); err == nil {
		response["instance"] = serializeInstanceState(state)
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGetCheckoutSessionStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("id"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "checkout session id is required")
		return
	}

	subscription, err := s.ensureCheckoutSessionStored(r.Context(), sessionID)
	if err != nil {
		var stripeErr stripeHTTPError
		switch {
		case errors.Is(err, mongo.ErrNoDocuments), errors.As(err, &stripeErr) && stripeErr.StatusCode == http.StatusNotFound:
			writeError(w, http.StatusNotFound, "checkout session not found")
		default:
			slog.Error("synchronize checkout", "error", err)
			writeError(w, http.StatusBadGateway, "failed to confirm subscription; please try again")
		}
		return
	}

	response := serializeBillingSubscription(subscription, false)
	if subscription.DeploymentMode == hostingSelfHosted && subscriptionStatusActive(subscription.Status) {
		licenseKey, generated, err := s.ensureLicenseKey(r.Context(), subscription)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create license key")
			return
		}
		if generated {
			response["licenseKey"] = licenseKey
		} else {
			response["licenseKeyAvailable"] = false
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.cfg.StripeWebhookSecret) == "" {
		writeError(w, http.StatusServiceUnavailable, "stripe webhook secret is not configured")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read webhook body")
		return
	}
	defer r.Body.Close()

	if !verifyStripeSignature(body, r.Header.Get("Stripe-Signature"), s.cfg.StripeWebhookSecret, 5*time.Minute) {
		writeError(w, http.StatusBadRequest, "invalid stripe signature")
		return
	}

	var event stripeEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid stripe event")
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var session stripeCheckoutSession
		if err := json.Unmarshal(event.Data.Object, &session); err != nil {
			writeError(w, http.StatusBadRequest, "invalid checkout session object")
			return
		}
		if err := s.storeCheckoutSession(r.Context(), session); err != nil {
			slog.Error("synchronize checkout webhook", "event_id", event.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to store checkout session")
			return
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var subscription stripeSubscription
		if err := json.Unmarshal(event.Data.Object, &subscription); err != nil {
			writeError(w, http.StatusBadRequest, "invalid subscription object")
			return
		}
		// Deliveries can arrive out of order. Read Stripe's current state so an
		// older created/updated event cannot undo a payment or cancellation.
		if strings.TrimSpace(subscription.ID) == "" {
			writeError(w, http.StatusBadRequest, "stripe subscription id is required")
			return
		}
		var latest stripeSubscription
		if err := s.stripeGet(r.Context(), "/subscriptions/"+url.PathEscape(subscription.ID), nil, &latest); err != nil {
			slog.Error("refresh subscription webhook", "event_id", event.ID, "error", err)
			writeError(w, http.StatusBadGateway, "failed to retrieve subscription")
			return
		}
		subscription = latest
		if err := s.storeStripeSubscription(r.Context(), subscription, "", "", ""); err != nil {
			slog.Error("synchronize subscription webhook", "event_id", event.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to store subscription")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"received": "true"})
}

func (s *Server) ensureCheckoutSessionStored(ctx context.Context, sessionID string) (BillingSubscription, error) {
	var existing BillingSubscription
	err := s.billingSubscriptions.FindOne(ctx, bson.M{"stripe_checkout_session_id": sessionID}).Decode(&existing)
	if err == nil && subscriptionStatusActive(existing.Status) {
		return existing, nil
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return BillingSubscription{}, err
	}

	if strings.TrimSpace(s.cfg.StripeSecretKey) != "" {
		var session stripeCheckoutSession
		if err := s.stripeGet(ctx, "/checkout/sessions/"+url.PathEscape(sessionID), nil, &session); err != nil {
			return BillingSubscription{}, err
		}
		if err := s.storeCheckoutSession(ctx, session); err != nil {
			return BillingSubscription{}, err
		}
	}

	err = s.billingSubscriptions.FindOne(ctx, bson.M{"stripe_checkout_session_id": sessionID}).Decode(&existing)
	if err != nil {
		return BillingSubscription{}, err
	}
	return existing, nil
}

func (s *Server) storeCheckoutSession(ctx context.Context, session stripeCheckoutSession) error {
	plan := normalizePlan(session.Metadata["plan"])
	deploymentMode := normalizeHostingMode(session.Metadata["deployment_mode"])
	userIDValue := firstNonEmpty(session.Metadata["user_id"], session.ClientReference)
	userID, _ := bson.ObjectIDFromHex(strings.TrimSpace(userIDValue))
	email := strings.ToLower(strings.TrimSpace(session.CustomerDetails.Email))

	if session.Subscription != "" {
		var subscription stripeSubscription
		if err := s.stripeGet(ctx, "/subscriptions/"+url.PathEscape(session.Subscription), nil, &subscription); err != nil {
			return err
		}
		return s.storeStripeSubscription(ctx, subscription, session.ID, email, userIDValue)
	}

	now := time.Now().UTC()
	set := bson.M{
		"plan":                       plan,
		"deployment_mode":            deploymentMode,
		"status":                     firstNonEmpty(session.Status, "checkout_complete"),
		"stripe_checkout_session_id": session.ID,
		"updated_at":                 now,
	}
	if email != "" {
		set["email"] = email
	}
	if session.Customer != "" {
		set["stripe_customer_id"] = session.Customer
	}
	if !userID.IsZero() {
		set["user_id"] = userID
	}
	if installationID := strings.TrimSpace(session.Metadata["installation_id"]); installationID != "" {
		set["installation_id"] = installationID
	}

	_, err := s.billingSubscriptions.UpdateOne(ctx, pendingCheckoutFilter(session.ID), bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"_id":        bson.NewObjectID(),
			"created_at": now,
		},
	}, options.UpdateOne().SetUpsert(true))
	// A concurrent confirmation already linked this checkout. Do not replace
	// its subscription status with an earlier open/expired checkout snapshot.
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	return err
}

func (s *Server) storeStripeSubscription(ctx context.Context, subscription stripeSubscription, checkoutSessionID, email, userIDValue string) error {
	if strings.TrimSpace(subscription.ID) == "" {
		return errors.New("stripe subscription id is required")
	}
	plan, deploymentMode, priceID := s.planFromStripeSubscription(subscription)
	userID, _ := bson.ObjectIDFromHex(strings.TrimSpace(firstNonEmpty(userIDValue, subscription.Metadata["user_id"])))
	installationID := strings.TrimSpace(subscription.Metadata["installation_id"])

	now := time.Now().UTC()
	set := bson.M{
		"plan":                   plan,
		"deployment_mode":        deploymentMode,
		"status":                 subscription.Status,
		"stripe_customer_id":     subscription.Customer,
		"stripe_subscription_id": subscription.ID,
		"stripe_price_id":        priceID,
		"cancel_at_period_end":   subscription.CancelAtPeriodEnd,
		"updated_at":             now,
	}
	if checkoutSessionID != "" {
		set["stripe_checkout_session_id"] = checkoutSessionID
	}
	if strings.TrimSpace(email) != "" {
		set["email"] = strings.ToLower(strings.TrimSpace(email))
	}
	if !userID.IsZero() {
		set["user_id"] = userID
	}
	if installationID != "" {
		set["installation_id"] = installationID
	}
	if periodEnd := subscription.periodEnd(); periodEnd > 0 {
		set["current_period_end"] = time.Unix(periodEnd, 0).UTC()
	}

	if checkoutSessionID != "" {
		// Promote the original checkout in place. A subscription webhook can
		// win this race and create its own row, in which case reconcile below.
		result, err := s.billingSubscriptions.UpdateOne(ctx, pendingCheckoutFilter(checkoutSessionID), bson.M{"$set": set})
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return err
		}
		if err == nil && result.MatchedCount > 0 {
			return nil
		}
	}

	filter := bson.M{"stripe_subscription_id": subscription.ID}
	delete(set, "stripe_checkout_session_id")
	_, err := s.billingSubscriptions.UpdateOne(ctx, filter, bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"_id":        bson.NewObjectID(),
			"created_at": now,
		},
	}, options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		// Another delivery inserted the same subscription concurrently.
		_, err = s.billingSubscriptions.UpdateOne(ctx, filter, bson.M{"$set": set})
	}
	if err != nil || checkoutSessionID == "" {
		return err
	}

	// The canonical subscription now has the confirmed checkout's identity
	// and billing data. Remove only an unlinked placeholder before assigning
	// its unique checkout ID. Retrying after either write is safe.
	if _, err := s.billingSubscriptions.DeleteOne(ctx, pendingCheckoutFilter(checkoutSessionID)); err != nil {
		return err
	}
	_, err = s.billingSubscriptions.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"stripe_checkout_session_id": checkoutSessionID}})
	return err
}

func pendingCheckoutFilter(sessionID string) bson.M {
	return bson.M{
		"stripe_checkout_session_id": sessionID,
		"stripe_subscription_id":     bson.M{"$in": []any{nil, ""}},
	}
}

func (s *Server) createCentralCheckoutSession(ctx context.Context, checkout createCheckoutRequest) (stripeCheckoutSession, error) {
	payload, err := json.Marshal(checkout)
	if err != nil {
		return stripeCheckoutSession{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, centralEngineURL+"/api/v1/billing/checkout", bytes.NewReader(payload))
	if err != nil {
		return stripeCheckoutSession{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return stripeCheckoutSession{}, centralEngineReachabilityError(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return stripeCheckoutSession{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return stripeCheckoutSession{}, fmt.Errorf("central checkout failed with status %d: %s", response.StatusCode, string(body))
	}

	var session stripeCheckoutSession
	if err := json.Unmarshal(body, &session); err != nil {
		return stripeCheckoutSession{}, err
	}
	return session, nil
}

func (s *Server) planFromStripeSubscription(subscription stripeSubscription) (plan, deploymentMode, priceID string) {
	plan = normalizePlan(subscription.Metadata["plan"])
	deploymentMode = normalizeHostingMode(subscription.Metadata["deployment_mode"])
	for _, item := range subscription.Items.Data {
		priceID = item.Price.ID
		if inferredPlan, inferredMode, ok := s.planFromStripePriceID(priceID); ok {
			if plan == planFree {
				plan = inferredPlan
			}
			deploymentMode = inferredMode
		}
	}
	return plan, deploymentMode, priceID
}

func (s *Server) findBillingSubscriptionForUser(ctx context.Context, user User) (BillingSubscription, error) {
	clauses := []bson.M{}
	if !user.ID.IsZero() {
		clauses = append(clauses, bson.M{"user_id": user.ID})
	}
	if user.Email != "" {
		clauses = append(clauses, bson.M{"email": strings.ToLower(strings.TrimSpace(user.Email))})
	}
	if len(clauses) == 0 {
		return BillingSubscription{}, mongo.ErrNoDocuments
	}

	var subscription BillingSubscription
	err := s.billingSubscriptions.FindOne(
		ctx,
		bson.M{"deployment_mode": hostingCloud, "$or": clauses},
		options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}}),
	).Decode(&subscription)
	return subscription, err
}

func (s *Server) ensureLicenseKey(ctx context.Context, subscription BillingSubscription) (string, bool, error) {
	if subscription.LicenseKeyHash != "" {
		return "", false, nil
	}

	licenseKey, prefix, err := generateLicenseKey()
	if err != nil {
		return "", false, err
	}

	now := time.Now().UTC()
	result := s.billingSubscriptions.FindOneAndUpdate(
		ctx,
		bson.M{"_id": subscription.ID, "license_key_hash": bson.M{"$in": []any{"", nil}}},
		bson.M{"$set": bson.M{
			"license_key_hash":   hashSecret(licenseKey),
			"license_key_prefix": prefix,
			"updated_at":         now,
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var updated BillingSubscription
	if err := result.Decode(&updated); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", false, nil
		}
		return "", false, err
	}

	return licenseKey, true, nil
}

func (s *Server) stripePriceID(plan, deploymentMode string) (string, bool) {
	switch normalizeHostingMode(deploymentMode) {
	case hostingCloud:
		switch normalizePlan(plan) {
		case planPro:
			return strings.TrimSpace(s.cfg.StripePriceProCloud), strings.TrimSpace(s.cfg.StripePriceProCloud) != ""
		case planEnterprise:
			return strings.TrimSpace(s.cfg.StripePriceEnterpriseCloud), strings.TrimSpace(s.cfg.StripePriceEnterpriseCloud) != ""
		}
	case hostingSelfHosted:
		switch normalizePlan(plan) {
		case planPro:
			return strings.TrimSpace(s.cfg.StripePriceProSelfHosted), strings.TrimSpace(s.cfg.StripePriceProSelfHosted) != ""
		case planEnterprise:
			return strings.TrimSpace(s.cfg.StripePriceEnterpriseSelfHosted), strings.TrimSpace(s.cfg.StripePriceEnterpriseSelfHosted) != ""
		}
	}
	return "", false
}

func (s *Server) planFromStripePriceID(priceID string) (plan, deploymentMode string, ok bool) {
	priceID = strings.TrimSpace(priceID)
	pairs := []struct {
		priceID        string
		plan           string
		deploymentMode string
	}{
		{s.cfg.StripePriceProCloud, planPro, hostingCloud},
		{s.cfg.StripePriceEnterpriseCloud, planEnterprise, hostingCloud},
		{s.cfg.StripePriceProSelfHosted, planPro, hostingSelfHosted},
		{s.cfg.StripePriceEnterpriseSelfHosted, planEnterprise, hostingSelfHosted},
	}
	for _, pair := range pairs {
		if strings.TrimSpace(pair.priceID) != "" && strings.TrimSpace(pair.priceID) == priceID {
			return pair.plan, pair.deploymentMode, true
		}
	}
	return planFree, hostingSelfHosted, false
}

func (s *Server) stripePostForm(ctx context.Context, path string, form url.Values, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, stripeAPIBase+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.sendStripeRequest(request, target)
}

func (s *Server) stripeGet(ctx context.Context, path string, query url.Values, target any) error {
	endpoint := stripeAPIBase + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	return s.sendStripeRequest(request, target)
}

func (s *Server) sendStripeRequest(request *http.Request, target any) error {
	request.SetBasicAuth(s.cfg.StripeSecretKey, "")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Stripe-Version", stripeAPIVersion)

	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return stripeHTTPError{StatusCode: response.StatusCode, Body: string(body)}
	}

	if target == nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode stripe response: %w", err)
	}
	return nil
}

func verifyStripeSignature(payload []byte, header, secret string, tolerance time.Duration) bool {
	values := parseStripeSignatureHeader(header)
	timestampValue := values["t"]
	signature := values["v1"]
	if timestampValue == "" || signature == "" {
		return false
	}

	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil {
		return false
	}
	signedAt := time.Unix(timestamp, 0)
	if tolerance > 0 && time.Since(signedAt) > tolerance {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestampValue))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func parseStripeSignatureHeader(header string) map[string]string {
	values := map[string]string{}
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && key != "" && value != "" {
			values[key] = value
		}
	}
	return values
}

func (s *Server) optionalUserFromSession(r *http.Request) (User, bool) {
	rawToken := sessionTokenFromRequest(r)
	if rawToken == "" {
		return User{}, false
	}

	user, err := s.userFromSession(r.Context(), rawToken)
	if err != nil {
		return User{}, false
	}
	return user, true
}

func (s *Server) externalURLAllowed(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	origin := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	for _, allowed := range s.allowedExternalOrigins() {
		if strings.EqualFold(strings.TrimRight(allowed, "/"), origin) {
			return true
		}
	}
	return false
}

func (s *Server) checkoutReturnURLAllowed(deploymentMode, value string) bool {
	if normalizeHostingMode(deploymentMode) != hostingSelfHosted {
		return s.externalURLAllowed(value)
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func (s *Server) allowedExternalOrigins() []string {
	origins := append([]string{}, s.cfg.CORSAllowedOrigins...)
	if parsed, err := url.Parse(s.cfg.PublicURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		origins = append(origins, fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host))
	}
	return origins
}

func generateLicenseKey() (rawKey, prefix string, err error) {
	keyID, err := randomHex(8)
	if err != nil {
		return "", "", err
	}
	secret, err := randomHex(32)
	if err != nil {
		return "", "", err
	}

	prefix = "rtz_lic_" + keyID
	return fmt.Sprintf("%s_%s", prefix, secret), prefix, nil
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func serializeBillingSubscription(subscription BillingSubscription, includeInternal bool) map[string]any {
	response := map[string]any{
		"id":                   subscription.ID.Hex(),
		"plan":                 normalizePlan(subscription.Plan),
		"deploymentMode":       normalizeHostingMode(subscription.DeploymentMode),
		"status":               subscription.Status,
		"stripeCustomerId":     subscription.StripeCustomerID,
		"stripeSubscriptionId": subscription.StripeSubscriptionID,
		"licenseKeyPrefix":     subscription.LicenseKeyPrefix,
		"cancelAtPeriodEnd":    subscription.CancelAtPeriodEnd,
	}
	if subscription.Email != "" {
		response["email"] = subscription.Email
	}
	if subscription.CurrentPeriodEnd != nil {
		response["currentPeriodEnd"] = subscription.CurrentPeriodEnd.Format(time.RFC3339)
	}
	if includeInternal {
		response["installationId"] = subscription.InstallationID
	}
	return response
}
