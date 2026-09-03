package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/config"
	"github.com/runtz-dev/runtz/engine/internal/telemetry"
	"github.com/runtz-dev/runtz/engine/internal/version"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	cfg                  config.Config
	client               *mongo.Client
	db                   *mongo.Database
	users                *mongo.Collection
	workspaces           *mongo.Collection
	apiKeys              *mongo.Collection
	scans                *mongo.Collection
	emailCodes           *mongo.Collection
	loginLockouts        *mongo.Collection
	sessions             *mongo.Collection
	billingSubscriptions *mongo.Collection
	instanceState        *mongo.Collection

	playgroundMu        sync.Mutex
	playgroundReadyDay  string
	playgroundWorkspace Workspace
}

type contextKey string

const currentUserKey contextKey = "currentUser"

func New(ctx context.Context, cfg config.Config) (*Server, error) {
	client, err := mongo.Connect(
		options.Client().
			ApplyURI(cfg.MongoURI).
			SetMonitor(telemetry.NewMongoMonitor()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	db := client.Database(cfg.MongoDatabase)
	server := &Server{
		cfg:                  cfg,
		client:               client,
		db:                   db,
		users:                db.Collection("users"),
		workspaces:           db.Collection("workspaces"),
		apiKeys:              db.Collection("api_keys"),
		scans:                db.Collection("scans"),
		emailCodes:           db.Collection("email_login_codes"),
		loginLockouts:        db.Collection("login_lockouts"),
		sessions:             db.Collection("sessions"),
		billingSubscriptions: db.Collection("billing_subscriptions"),
		instanceState:        db.Collection("instance_state"),
	}

	if err := server.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	if err := server.backfillVulnerabilityFixSummaries(ctx); err != nil {
		return nil, err
	}

	server.startLicenseHeartbeat(ctx)

	return server, nil
}

func (s *Server) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/setup/status", s.handleSetupStatus)
	mux.HandleFunc("POST /api/v1/setup", s.handleSetup)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/google", s.handleGoogleLogin)
	mux.HandleFunc("POST /api/v1/auth/github", s.handleGitHubLogin)
	mux.HandleFunc("POST /api/v1/auth/email/request", s.handleRequestEmailLogin)
	mux.HandleFunc("POST /api/v1/auth/email/verify", s.handleVerifyEmailLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.Handle("GET /api/v1/me", s.auth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/v1/me/deletion-impact", s.auth(http.HandlerFunc(s.handleAccountDeletionImpact)))
	mux.Handle("DELETE /api/v1/me", s.auth(http.HandlerFunc(s.handleDeleteAccount)))
	mux.Handle("PATCH /api/v1/me/onboarding", s.auth(http.HandlerFunc(s.handleCompleteOnboarding)))
	mux.Handle("PATCH /api/v1/me/password", s.auth(http.HandlerFunc(s.handleChangePassword)))
	mux.Handle("GET /api/v1/usage", s.auth(http.HandlerFunc(s.handleUsage)))
	mux.Handle("GET /api/v1/workspaces", s.auth(http.HandlerFunc(s.handleListWorkspaces)))
	mux.Handle("POST /api/v1/workspaces", s.adminOnly(http.HandlerFunc(s.handleCreateWorkspace)))
	mux.Handle("GET /api/v1/workspaces/{id}/deletion-impact", s.auth(http.HandlerFunc(s.handleWorkspaceDeletionImpact)))
	mux.Handle("DELETE /api/v1/workspaces/{id}", s.auth(http.HandlerFunc(s.handleDeleteWorkspace)))
	mux.Handle("GET /api/v1/api-keys", s.auth(http.HandlerFunc(s.handleListAPIKeys)))
	mux.Handle("POST /api/v1/api-keys", s.auth(http.HandlerFunc(s.handleCreateAPIKey)))
	mux.Handle("PATCH /api/v1/api-keys/{id}", s.auth(http.HandlerFunc(s.handleUpdateAPIKey)))
	mux.Handle("DELETE /api/v1/api-keys/{id}", s.auth(http.HandlerFunc(s.handleDeleteAPIKey)))
	mux.Handle("PATCH /api/v1/api-keys/{id}/revoke", s.auth(http.HandlerFunc(s.handleRevokeAPIKey)))
	mux.HandleFunc("POST /api/v1/billing/checkout", s.handleCreateCheckoutSession)
	mux.Handle("POST /api/v1/billing/portal", s.auth(http.HandlerFunc(s.handleCreateBillingPortalSession)))
	mux.Handle("GET /api/v1/billing/status", s.auth(http.HandlerFunc(s.handleBillingStatus)))
	mux.HandleFunc("GET /api/v1/billing/checkout-session/{id}", s.handleGetCheckoutSessionStatus)
	mux.HandleFunc("POST /api/v1/billing/webhook", s.handleStripeWebhook)
	mux.Handle("POST /api/v1/license/activate", s.adminOnly(http.HandlerFunc(s.handleActivateSelfHostedLicense)))
	mux.Handle("POST /api/v1/license/checkout/activate", s.adminOnly(http.HandlerFunc(s.handleActivateSelfHostedCheckout)))
	mux.Handle("POST /api/v1/license/refresh", s.adminOnly(http.HandlerFunc(s.handleRefreshSelfHostedLicense)))
	mux.HandleFunc("POST /api/v1/licenses/activate-checkout", s.handleActivateCheckoutLicense)
	mux.HandleFunc("POST /api/v1/licenses/validate", s.handleValidateLicense)
	mux.Handle("GET /api/v1/users", s.adminOnly(http.HandlerFunc(s.handleListUsers)))
	mux.Handle("POST /api/v1/users", s.adminOnly(http.HandlerFunc(s.handleCreateUser)))
	mux.Handle("PATCH /api/v1/users/{id}", s.adminOnly(http.HandlerFunc(s.handleUpdateUser)))
	mux.Handle("POST /api/v1/users/{id}/invite", s.adminOnly(http.HandlerFunc(s.handleCreateInvite)))
	mux.HandleFunc("GET /api/v1/keys/verify", s.handleVerifyKey)
	mux.HandleFunc("POST /api/v1/ingest/sca", s.handleIngestSCA)
	mux.HandleFunc("POST /api/v1/ingest/sast", s.handleIngestSAST)
	mux.HandleFunc("POST /api/v1/ingest/host", s.handleIngestHost)
	mux.HandleFunc("POST /api/v1/ingest/container", s.handleIngestContainer)
	mux.HandleFunc("POST /api/v1/ingest/k8s", s.handleIngestK8s)
	mux.HandleFunc("POST /api/v1/ingest/kubernetes", s.handleIngestK8s)
	mux.HandleFunc("GET /api/v1/playground/scans", s.handleListAllPlaygroundScans)
	mux.HandleFunc("GET /api/v1/playground/scans/{type}", s.handleListPlaygroundScans)
	mux.Handle("GET /api/v1/scans/sca", s.auth(http.HandlerFunc(s.handleListSCAScans)))
	mux.Handle("GET /api/v1/scans/sca/{id}", s.auth(http.HandlerFunc(s.handleGetSCAScan)))
	mux.Handle("GET /api/v1/scans/sast", s.auth(http.HandlerFunc(s.handleListSASTScans)))
	mux.Handle("GET /api/v1/scans/sast/{id}", s.auth(http.HandlerFunc(s.handleGetSASTScan)))
	mux.Handle("GET /api/v1/scans/host", s.auth(http.HandlerFunc(s.handleListHostScans)))
	mux.Handle("GET /api/v1/scans/host/{id}", s.auth(http.HandlerFunc(s.handleGetHostScan)))
	mux.Handle("GET /api/v1/scans/container", s.auth(http.HandlerFunc(s.handleListContainerScans)))
	mux.Handle("GET /api/v1/scans/container/{id}", s.auth(http.HandlerFunc(s.handleGetContainerScan)))
	mux.Handle("GET /api/v1/scans/k8s", s.auth(http.HandlerFunc(s.handleListK8sScans)))
	mux.Handle("GET /api/v1/scans/k8s/{id}", s.auth(http.HandlerFunc(s.handleGetK8sScan)))

	// otelhttp goes outermost so a span exists before anything else runs,
	// including the CORS preflight short-circuit. withRoutePattern sits
	// underneath it to give that span a route-shaped name.
	return otelhttp.NewHandler(
		s.withCORS(withRoutePattern(mux)),
		"runtz-engine",
		otelhttp.WithFilter(func(r *http.Request) bool {
			// The kubelet hits /health on both probes every few seconds.
			// Tracing that says nothing and drowns out real traffic.
			return r.URL.Path != "/health"
		}),
	)
}

// withRoutePattern renames the request span after the ServeMux pattern that
// will handle it — "GET /api/v1/scans/sca/{id}" rather than the raw path — so
// traces group by endpoint instead of exploding into one name per scan id.
//
// It has to resolve the route itself: Go's ServeMux only fills Request.Pattern
// on the request it passes to the matched handler, which the middleware
// wrapping the mux never sees. Handler runs the same trie lookup ServeHTTP is
// about to run, so this costs one extra match per request and no allocation.
func withRoutePattern(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			span := trace.SpanFromContext(r.Context())
			span.SetName(pattern)
			span.SetAttributes(semconv.HTTPRoute(pattern))

			// The labeler feeds otelhttp's duration histogram, which would
			// otherwise carry no route at all.
			if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
				labeler.Add(semconv.HTTPRoute(pattern))
			}
		}

		mux.ServeHTTP(w, r)
	})
}

func (s *Server) ensureIndexes(ctx context.Context) error {
	_, err := s.users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "google_subject", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "github_subject", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	})
	if err != nil {
		return fmt.Errorf("create users indexes: %w", err)
	}

	_, err = s.workspaces.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("create workspace indexes: %w", err)
	}

	_, err = s.apiKeys.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "prefix", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "key_hash", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "workspace_id", Value: 1}, {Key: "created_at", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("create api key indexes: %w", err)
	}

	_, err = s.scans.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "type", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "workspace_id", Value: 1}, {Key: "type", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "workspace_id", Value: 1}, {Key: "source", Value: 1}, {Key: "type", Value: 1}, {Key: "created_at", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("create scan indexes: %w", err)
	}

	_, err = s.emailCodes.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{Keys: bson.D{{Key: "email", Value: 1}, {Key: "created_at", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("create email login code indexes: %w", err)
	}

	_, err = s.sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "token_hash", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{
			// Mongo reaps expired sessions on its own, so nothing has to sweep
			// the collection and a signed-out week-old row cannot linger.
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	})
	if err != nil {
		return fmt.Errorf("create session indexes: %w", err)
	}

	_, err = s.loginLockouts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "locked_until", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	})
	if err != nil {
		return fmt.Errorf("create login lockout indexes: %w", err)
	}

	_, err = s.billingSubscriptions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "stripe_subscription_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "stripe_checkout_session_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "license_key_hash", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{Keys: bson.D{{Key: "email", Value: 1}, {Key: "updated_at", Value: -1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "updated_at", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("create billing subscription indexes: %w", err)
	}

	_, err = s.instanceState.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "installation_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("create instance state indexes: %w", err)
	}

	return nil
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Runtz-API-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range s.cfg.CORSAllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken := sessionTokenFromRequest(r)
		if rawToken == "" {
			writeError(w, http.StatusUnauthorized, "missing session")
			return
		}

		user, err := s.userFromSession(r.Context(), rawToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid session")
			return
		}

		ctx := context.WithValue(r.Context(), currentUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) adminOnly(next http.Handler) http.Handler {
	return s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentUser(r.Context())
		if !ok || user.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}

		next.ServeHTTP(w, r)
	}))
}

func currentUser(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(currentUserKey).(User)
	return user, ok
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"name":    "runtz",
		"version": version.Version,
	})
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.users.CountDocuments(r.Context(), bson.M{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read setup status")
		return
	}

	entitlement := s.currentEntitlement(r.Context(), nil)
	googleEnabled := featureEnabled(entitlement.Features, featureGoogleAuth)
	githubEnabled := featureEnabled(entitlement.Features, featureGitHubAuth)
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":     count > 0,
		"deploymentMode": s.cfg.DeploymentMode,
		"entitlement":    entitlement,
		"auth": map[string]any{
			"email":          strings.TrimSpace(s.cfg.ResendAPIKey) != "",
			"google":         googleEnabled && strings.TrimSpace(s.cfg.GoogleClientID) != "",
			"github":         githubEnabled && strings.TrimSpace(s.cfg.GitHubClientID) != "" && strings.TrimSpace(s.cfg.GitHubClientSecret) != "",
			"githubClientId": s.cfg.GitHubClientID,
			"googleClientId": s.cfg.GoogleClientID,
		},
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode == "cloud" {
		writeError(w, http.StatusNotFound, "setup is not available in cloud mode")
		return
	}

	var request struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		WorkspaceName string `json:"workspaceName"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	request.Username = strings.TrimSpace(request.Username)
	request.WorkspaceName = strings.TrimSpace(request.WorkspaceName)
	if request.Username == "" || len(request.Password) < 8 || request.WorkspaceName == "" {
		writeError(w, http.StatusBadRequest, "username, workspaceName and password with at least 8 characters are required")
		return
	}

	count, err := s.users.CountDocuments(r.Context(), bson.M{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify setup state")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "runtz is already configured")
		return
	}

	now := time.Now().UTC()
	workspace := Workspace{
		ID:        bson.NewObjectID(),
		Name:      request.WorkspaceName,
		Slug:      slugify(request.WorkspaceName),
		Kind:      "manual",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.workspaces.InsertOne(r.Context(), workspace); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workspace")
		return
	}

	passwordHash, err := hashPassword(request.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure password")
		return
	}

	user := User{
		ID:                    bson.NewObjectID(),
		Username:              request.Username,
		AuthProvider:          "password",
		PasswordHash:          passwordHash,
		Role:                  "admin",
		WorkspaceIDs:          []bson.ObjectID{workspace.ID},
		RequirePasswordChange: false,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if _, err := s.users.InsertOne(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin user")
		return
	}

	if err := s.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start session")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user":      serializeUser(user),
		"workspace": serializeWorkspace(workspace),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode == "cloud" {
		writeError(w, http.StatusNotFound, "password login is not available in cloud mode")
		return
	}

	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	username := strings.TrimSpace(request.Username)
	lockoutKey := passwordLockoutKey(username)

	lockedUntil, err := s.loginLocked(r.Context(), lockoutKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify credentials")
		return
	}
	if !lockedUntil.IsZero() {
		writeLockoutError(w, lockedUntil, "passwords")
		return
	}

	// The lookup miss and the password mismatch share one branch and one
	// message so the response does not reveal whether the username exists.
	var user User
	err = s.users.FindOne(r.Context(), bson.M{"username": username}).Decode(&user)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)) != nil {
		if lockErr := s.registerFailedLogin(
			r.Context(), lockoutKey, time.Now().UTC(), maxPasswordLoginAttempts, passwordLoginLockoutTTL,
		); lockErr != nil {
			slog.Warn("failed to record login failure", "error", lockErr)
		}
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if err := s.clearLoginFailures(r.Context(), lockoutKey); err != nil {
		slog.Warn("failed to clear login failures", "error", err)
	}

	if err := s.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": serializeUser(user),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	workspaces, err := s.getVisibleWorkspaces(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspaces")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":           serializeUser(user),
		"workspaces":     serializeWorkspaces(workspaces),
		"deploymentMode": s.cfg.DeploymentMode,
		"entitlement":    s.currentEntitlement(r.Context(), &user),
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode == "cloud" {
		writeError(w, http.StatusNotFound, "passwords are not available in cloud mode")
		return
	}

	user, _ := currentUser(r.Context())
	var request struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if len(request.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must have at least 8 characters")
		return
	}
	if !user.RequirePasswordChange && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.CurrentPassword)) != nil {
		writeError(w, http.StatusUnauthorized, "current password is invalid")
		return
	}

	passwordHash, err := hashPassword(request.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure password")
		return
	}

	_, err = s.users.UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"password_hash":           passwordHash,
			"require_password_change": false,
			"updated_at":              time.Now().UTC(),
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	workspaces, err := s.getVisibleWorkspaces(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspaces")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspaces": serializeWorkspaces(workspaces)})
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	plan := s.currentEntitlement(r.Context(), &user).Plan
	if limit := workspaceLimitForPlan(plan); limit >= 0 {
		count, err := s.workspaceCountForOwner(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check workspace limit")
			return
		}
		if count >= limit {
			writeError(w, http.StatusPaymentRequired, fmt.Sprintf(
				"workspace limit reached for your plan (%s); upgrade to add more workspaces",
				formatUsageLimit(limit),
			))
			return
		}
	}

	var request struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "workspace name is required")
		return
	}

	now := time.Now().UTC()
	workspace := Workspace{
		ID:        bson.NewObjectID(),
		Name:      name,
		Slug:      slugify(name),
		Kind:      "manual",
		CreatedBy: user.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.workspaces.InsertOne(r.Context(), workspace); err != nil {
		writeError(w, http.StatusConflict, "workspace slug already exists")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"workspace": serializeWorkspace(workspace)})
}

func (s *Server) handleCompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r.Context())
	now := time.Now().UTC()
	_, err := s.users.UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"onboarding_completed": true,
			"updated_at":           now,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete onboarding")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "onboarding_completed"})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	cursor, err := s.users.Find(r.Context(), bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	defer closeCursor(r.Context(), cursor)

	users := make([]User, 0)
	if err := cursor.All(r.Context(), &users); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decode users")
		return
	}

	response := make([]publicUser, 0, len(users))
	for _, user := range users {
		response = append(response, serializeUser(user))
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": response})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	plan := s.currentEntitlement(r.Context(), nil).Plan
	if limit := userLimitForPlan(plan, s.cfg.DeploymentMode); limit >= 0 {
		count, err := s.users.CountDocuments(r.Context(), bson.M{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check user limit")
			return
		}
		if count >= limit {
			writeError(w, http.StatusPaymentRequired, fmt.Sprintf(
				"user limit reached for your plan (%s); upgrade to add more users",
				formatUsageLimit(limit),
			))
			return
		}
	}

	var request struct {
		Username              string   `json:"username"`
		Password              string   `json:"password"`
		Role                  string   `json:"role"`
		WorkspaceIDs          []string `json:"workspaceIds"`
		RequirePasswordChange bool     `json:"requirePasswordChange"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	username := strings.TrimSpace(request.Username)
	if username == "" || len(request.Password) < 8 {
		writeError(w, http.StatusBadRequest, "username and password with at least 8 characters are required")
		return
	}

	role := normalizeRole(request.Role)
	workspaceIDs, err := s.parseWorkspaceIDs(r.Context(), request.WorkspaceIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	passwordHash, err := hashPassword(request.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure password")
		return
	}

	now := time.Now().UTC()
	user := User{
		ID:                    bson.NewObjectID(),
		Username:              username,
		AuthProvider:          "password",
		PasswordHash:          passwordHash,
		Role:                  role,
		WorkspaceIDs:          workspaceIDs,
		RequirePasswordChange: request.RequirePasswordChange,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if _, err := s.users.InsertOne(r.Context(), user); err != nil {
		writeError(w, http.StatusConflict, "username already exists")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"user": serializeUser(user)})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var request struct {
		Password              *string  `json:"password"`
		Role                  *string  `json:"role"`
		WorkspaceIDs          []string `json:"workspaceIds"`
		RequirePasswordChange *bool    `json:"requirePasswordChange"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	set := bson.M{"updated_at": time.Now().UTC()}
	if request.Role != nil {
		set["role"] = normalizeRole(*request.Role)
	}
	if request.RequirePasswordChange != nil {
		set["require_password_change"] = *request.RequirePasswordChange
	}
	if request.Password != nil && strings.TrimSpace(*request.Password) != "" {
		if len(*request.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must have at least 8 characters")
			return
		}
		passwordHash, err := hashPassword(*request.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to secure password")
			return
		}
		set["password_hash"] = passwordHash
	}
	if request.WorkspaceIDs != nil {
		workspaceIDs, err := s.parseWorkspaceIDs(r.Context(), request.WorkspaceIDs)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		set["workspace_ids"] = workspaceIDs
	}

	result := s.users.FindOneAndUpdate(
		r.Context(),
		bson.M{"_id": userID},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var updated User
	if err := result.Decode(&updated); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": serializeUser(updated)})
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if _, err := bson.ObjectIDFromHex(userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	token, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate invite token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"inviteLink": fmt.Sprintf("http://localhost:3000/invite/%s?token=%s", userID, token),
	})
}

func (s *Server) handleIngestSCA(w http.ResponseWriter, r *http.Request) {
	workspace, ok := s.authenticateIngest(w, r)
	if !ok {
		return
	}

	var request struct {
		// workspaceId/workspace are accepted and ignored: the workspace comes
		// from the API key. They stay in the struct because decodeJSON rejects
		// unknown fields, and older CLI versions still send them.
		WorkspaceID     string          `json:"workspaceId"`
		Workspace       string          `json:"workspace"`
		ProjectName     string          `json:"projectName"`
		Source          string          `json:"source"`
		TargetFile      string          `json:"targetFile"`
		ScannerVersion  string          `json:"scannerVersion"`
		Dependencies    []Dependency    `json:"dependencies"`
		Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	if strings.TrimSpace(request.ProjectName) == "" {
		writeError(w, http.StatusBadRequest, "projectName is required")
		return
	}

	now := time.Now().UTC()
	scan := Scan{
		ID:              bson.NewObjectID(),
		Type:            "sca",
		WorkspaceID:     workspace.ID,
		WorkspaceName:   workspace.Name,
		ProjectName:     request.ProjectName,
		Source:          request.Source,
		TargetFile:      request.TargetFile,
		Status:          "completed",
		ScannerVersion:  request.ScannerVersion,
		Summary:         buildSummary(request.Dependencies, request.Vulnerabilities),
		Dependencies:    orEmpty(request.Dependencies),
		Vulnerabilities: orEmpty(request.Vulnerabilities),
		CreatedAt:       now,
	}

	if _, err := s.scans.InsertOne(r.Context(), scan); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store scan")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      scan.ID.Hex(),
		"status":  "stored",
		"summary": scan.Summary,
	})
}

func (s *Server) handleIngestSAST(w http.ResponseWriter, r *http.Request) {
	s.handleIngestFindingsScan(w, r, "sast")
}

func (s *Server) handleIngestK8s(w http.ResponseWriter, r *http.Request) {
	s.handleIngestFindingsScan(w, r, "k8s")
}

func (s *Server) handleIngestFindingsScan(w http.ResponseWriter, r *http.Request, scanType string) {
	workspace, ok := s.authenticateIngest(w, r)
	if !ok {
		return
	}

	var request struct {
		// workspaceId/workspace are accepted and ignored: the workspace comes
		// from the API key. They stay in the struct because decodeJSON rejects
		// unknown fields, and older CLI versions still send them.
		WorkspaceID      string    `json:"workspaceId"`
		Workspace        string    `json:"workspace"`
		ProjectName      string    `json:"projectName"`
		TargetName       string    `json:"targetName"`
		Source           string    `json:"source"`
		ScannerVersion   string    `json:"scannerVersion"`
		FilesScanned     int       `json:"filesScanned"`
		ResourcesScanned int       `json:"resourcesScanned"`
		Findings         []Finding `json:"findings"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	targetName := strings.TrimSpace(firstNonEmpty(request.TargetName, request.ProjectName))
	if targetName == "" {
		writeError(w, http.StatusBadRequest, "targetName is required")
		return
	}
	projectName := strings.TrimSpace(request.ProjectName)
	if projectName == "" {
		projectName = targetName
	}

	now := time.Now().UTC()
	totalScanned := request.FilesScanned
	if scanType == "k8s" {
		totalScanned = request.ResourcesScanned
	}
	scan := Scan{
		ID:               bson.NewObjectID(),
		Type:             scanType,
		WorkspaceID:      workspace.ID,
		WorkspaceName:    workspace.Name,
		ProjectName:      projectName,
		TargetName:       targetName,
		Source:           request.Source,
		Status:           "completed",
		ScannerVersion:   request.ScannerVersion,
		FilesScanned:     request.FilesScanned,
		ResourcesScanned: request.ResourcesScanned,
		Summary:          buildFindingSummary(totalScanned, request.Findings),
		Findings:         request.Findings,
		CreatedAt:        now,
	}

	if _, err := s.scans.InsertOne(r.Context(), scan); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store scan")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      scan.ID.Hex(),
		"status":  "stored",
		"summary": scan.Summary,
	})
}

func (s *Server) handleIngestHost(w http.ResponseWriter, r *http.Request) {
	s.handleIngestPackageScan(w, r, "host")
}

func (s *Server) handleIngestContainer(w http.ResponseWriter, r *http.Request) {
	s.handleIngestPackageScan(w, r, "container")
}

func (s *Server) handleIngestPackageScan(w http.ResponseWriter, r *http.Request, scanType string) {
	workspace, ok := s.authenticateIngest(w, r)
	if !ok {
		return
	}

	var request struct {
		// workspaceId/workspace are accepted and ignored: the workspace comes
		// from the API key. They stay in the struct because decodeJSON rejects
		// unknown fields, and older CLI versions still send them.
		WorkspaceID     string          `json:"workspaceId"`
		Workspace       string          `json:"workspace"`
		TargetName      string          `json:"targetName"`
		Hostname        string          `json:"hostname"`
		ImageName       string          `json:"imageName"`
		ImageRef        string          `json:"imageRef"`
		ImageDigest     string          `json:"imageDigest"`
		Source          string          `json:"source"`
		OSID            string          `json:"osId"`
		OSName          string          `json:"osName"`
		OSVersion       string          `json:"osVersion"`
		OSCodename      string          `json:"osCodename"`
		PackageManager  string          `json:"packageManager"`
		ScannerVersion  string          `json:"scannerVersion"`
		Packages        []Package       `json:"packages"`
		Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	targetName := strings.TrimSpace(request.TargetName)
	if targetName == "" && scanType == "host" {
		targetName = strings.TrimSpace(request.Hostname)
	}
	if targetName == "" && scanType == "container" {
		targetName = strings.TrimSpace(firstNonEmpty(request.ImageName, request.ImageRef))
	}
	if targetName == "" {
		writeError(w, http.StatusBadRequest, "targetName is required")
		return
	}

	now := time.Now().UTC()
	scan := Scan{
		ID:              bson.NewObjectID(),
		Type:            scanType,
		WorkspaceID:     workspace.ID,
		WorkspaceName:   workspace.Name,
		ProjectName:     targetName,
		TargetName:      targetName,
		Hostname:        request.Hostname,
		ImageName:       request.ImageName,
		ImageRef:        request.ImageRef,
		ImageDigest:     request.ImageDigest,
		Source:          request.Source,
		Status:          "completed",
		OSID:            request.OSID,
		OSName:          request.OSName,
		OSVersion:       request.OSVersion,
		OSCodename:      request.OSCodename,
		PackageManager:  request.PackageManager,
		ScannerVersion:  request.ScannerVersion,
		Summary:         buildPackageSummary(request.Packages, request.Vulnerabilities),
		Packages:        request.Packages,
		Vulnerabilities: orEmpty(request.Vulnerabilities),
		CreatedAt:       now,
	}

	if _, err := s.scans.InsertOne(r.Context(), scan); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store scan")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      scan.ID.Hex(),
		"status":  "stored",
		"summary": scan.Summary,
	})
}

func (s *Server) handleListSCAScans(w http.ResponseWriter, r *http.Request) {
	s.handleListScansByType(w, r, "sca")
}

func (s *Server) handleListSASTScans(w http.ResponseWriter, r *http.Request) {
	s.handleListScansByType(w, r, "sast")
}

func (s *Server) handleListHostScans(w http.ResponseWriter, r *http.Request) {
	s.handleListScansByType(w, r, "host")
}

func (s *Server) handleListContainerScans(w http.ResponseWriter, r *http.Request) {
	s.handleListScansByType(w, r, "container")
}

func (s *Server) handleListK8sScans(w http.ResponseWriter, r *http.Request) {
	s.handleListScansByType(w, r, "k8s")
}

func (s *Server) handleListScansByType(w http.ResponseWriter, r *http.Request, scanType string) {
	user, _ := currentUser(r.Context())
	filter := bson.M{"type": scanType}
	if workspaceIDParam := r.URL.Query().Get("workspaceId"); workspaceIDParam != "" {
		workspaceID, err := bson.ObjectIDFromHex(workspaceIDParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid workspace id")
			return
		}
		if !s.userCanAccessWorkspace(user, workspaceID) {
			writeError(w, http.StatusForbidden, "workspace access required")
			return
		}
		filter["workspace_id"] = workspaceID
	} else if !s.globalDataScope(user) {
		filter["workspace_id"] = bson.M{"$in": user.WorkspaceIDs}
	}

	// List views only need scan metadata and the precomputed summary. Excluding
	// the result arrays is especially important for host scans, where a single
	// inventory may contain thousands of packages and CVEs. The full document
	// remains available from GET /api/v1/scans/{type}/{id}.
	cursor, err := s.scans.Find(
		r.Context(),
		filter,
		options.Find().
			SetProjection(bson.M{
				"dependencies":    0,
				"packages":        0,
				"findings":        0,
				"vulnerabilities": 0,
			}).
			SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetLimit(50),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list scans")
		return
	}
	defer closeCursor(r.Context(), cursor)

	scans := make([]Scan, 0)
	if err := cursor.All(r.Context(), &scans); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decode scans")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"scans": scans})
}

func (s *Server) handleGetSCAScan(w http.ResponseWriter, r *http.Request) {
	s.handleGetScanByType(w, r, "sca")
}

func (s *Server) handleGetSASTScan(w http.ResponseWriter, r *http.Request) {
	s.handleGetScanByType(w, r, "sast")
}

func (s *Server) handleGetHostScan(w http.ResponseWriter, r *http.Request) {
	s.handleGetScanByType(w, r, "host")
}

func (s *Server) handleGetContainerScan(w http.ResponseWriter, r *http.Request) {
	s.handleGetScanByType(w, r, "container")
}

func (s *Server) handleGetK8sScan(w http.ResponseWriter, r *http.Request) {
	s.handleGetScanByType(w, r, "k8s")
}

func (s *Server) handleGetScanByType(w http.ResponseWriter, r *http.Request, scanType string) {
	user, _ := currentUser(r.Context())
	scanID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	findOptions := options.FindOne()
	if r.URL.Query().Get("view") == "results" {
		projection := bson.M{
			"dependencies": 0,
			"packages":     0,
		}
		if scanType == "sast" || scanType == "k8s" {
			projection["vulnerabilities"] = 0
		} else {
			projection["findings"] = 0
		}
		findOptions.SetProjection(projection)
	}

	var scan Scan
	if err := s.scans.FindOne(
		r.Context(),
		bson.M{"_id": scanID, "type": scanType},
		findOptions,
	).Decode(&scan); err != nil {
		writeError(w, http.StatusNotFound, "scan not found")
		return
	}
	if !s.userCanAccessWorkspace(user, scan.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workspace access required")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"scan": scan})
}

func (s *Server) getVisibleWorkspaces(ctx context.Context, user User) ([]Workspace, error) {
	filter := bson.M{}
	if !s.globalDataScope(user) {
		filter["_id"] = bson.M{"$in": user.WorkspaceIDs}
	}

	cursor, err := s.workspaces.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cursor)

	var workspaces []Workspace
	if err := cursor.All(ctx, &workspaces); err != nil {
		return nil, err
	}

	return workspaces, nil
}

func (s *Server) parseWorkspaceIDs(ctx context.Context, ids []string) ([]bson.ObjectID, error) {
	if len(ids) == 0 {
		workspaces, err := s.getAllWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		workspaceIDs := make([]bson.ObjectID, 0, len(workspaces))
		for _, workspace := range workspaces {
			workspaceIDs = append(workspaceIDs, workspace.ID)
		}
		return workspaceIDs, nil
	}

	workspaceIDs := make([]bson.ObjectID, 0, len(ids))
	for _, rawID := range ids {
		workspaceID, err := bson.ObjectIDFromHex(rawID)
		if err != nil {
			return nil, fmt.Errorf("invalid workspace id: %s", rawID)
		}
		workspaceIDs = append(workspaceIDs, workspaceID)
	}

	return workspaceIDs, nil
}

// workspaceCountForOwner reports how many workspaces already count against
// ownerID's workspace limit. Self-hosted is a single instance (one org, no
// per-user ownership), so every workspace counts; cloud scopes to the
// workspaces this account created, matching accountLimitCounts in usage.go.
func (s *Server) workspaceCountForOwner(ctx context.Context, ownerID bson.ObjectID) (int64, error) {
	filter := bson.M{}
	if s.cfg.DeploymentMode != hostingSelfHosted {
		filter["created_by"] = ownerID
	}
	return s.workspaces.CountDocuments(ctx, filter)
}

func (s *Server) getAllWorkspaces(ctx context.Context) ([]Workspace, error) {
	cursor, err := s.workspaces.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cursor)

	var workspaces []Workspace
	if err := cursor.All(ctx, &workspaces); err != nil {
		return nil, err
	}

	return workspaces, nil
}

func buildSummary(dependencies []Dependency, vulnerabilities []Vulnerability) ScanSummary {
	summary := ScanSummary{
		TotalDependencies: len(dependencies),
		Vulnerabilities:   len(vulnerabilities),
		FixStatusComputed: true,
	}

	for _, vulnerability := range vulnerabilities {
		addVulnerabilityToSummary(&summary, vulnerability)
	}

	return summary
}

func buildPackageSummary(packages []Package, vulnerabilities []Vulnerability) ScanSummary {
	summary := ScanSummary{
		TotalDependencies: len(packages),
		Vulnerabilities:   len(vulnerabilities),
		FixStatusComputed: true,
	}

	for _, vulnerability := range vulnerabilities {
		addVulnerabilityToSummary(&summary, vulnerability)
	}

	return summary
}

func addVulnerabilityToSummary(summary *ScanSummary, vulnerability Vulnerability) {
	counts := &summary.WithoutFix
	if strings.TrimSpace(vulnerability.FirstPatchedVersion) != "" {
		counts = &summary.WithFix
	}

	counts.Vulnerabilities++
	switch strings.ToLower(strings.TrimSpace(vulnerability.Severity)) {
	case "critical":
		summary.Critical++
		counts.Critical++
	case "high":
		summary.High++
		counts.High++
	case "medium":
		summary.Medium++
		counts.Medium++
	case "low":
		summary.Low++
		counts.Low++
	default:
		summary.Unknown++
		counts.Unknown++
	}
}

func buildFindingSummary(totalScanned int, findings []Finding) ScanSummary {
	summary := ScanSummary{
		TotalDependencies: totalScanned,
		Vulnerabilities:   len(findings),
	}

	for _, finding := range findings {
		switch strings.ToLower(finding.Severity) {
		case "critical":
			summary.Critical++
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		case "low":
			summary.Low++
		default:
			summary.Unknown++
		}
	}

	return summary
}

// orEmpty turns a nil slice into an empty one so it is stored — and later
// served — as [] instead of null. Scanners legitimately send no results at
// all: macOS host scans never carry vulnerabilities because OSV has no feed
// for Homebrew, and Go marshals a nil slice as null. Clients then have to
// guard every read, and the one that forgets crashes on .length.
func orEmpty[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func normalizeRole(role string) string {
	if strings.EqualFold(role, "admin") {
		return "admin"
	}

	return "member"
}

func serializeWorkspaces(workspaces []Workspace) []publicWorkspace {
	response := make([]publicWorkspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		response = append(response, serializeWorkspace(workspace))
	}

	return response
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "workspace"
	}

	return value
}

func closeCursor(ctx context.Context, cursor *mongo.Cursor) {
	if err := cursor.Close(ctx); err != nil {
		slog.Warn("failed to close mongo cursor", "error", err)
	}
}

func randomToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
