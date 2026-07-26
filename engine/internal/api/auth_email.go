package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	emailLoginCodeTTL      = 10 * time.Minute
	emailLoginResendWindow = time.Minute
	// maxEmailLoginAttempts wrong guesses kill the code (see the "attempts"
	// filter in handleVerifyEmailLogin) and, on the same guess that reaches
	// the limit, lock the email out of requesting or verifying any code for
	// emailLoginLockoutTTL — see lockEmailLogin.
	maxEmailLoginAttempts = 10
	emailLoginLockoutTTL  = time.Hour
)

type emailLoginCode struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Email     string        `bson:"email"`
	CodeHash  string        `bson:"code_hash"`
	Attempts  int           `bson:"attempts"`
	CreatedAt time.Time     `bson:"created_at"`
	ExpiresAt time.Time     `bson:"expires_at"`
	UsedAt    *time.Time    `bson:"used_at,omitempty"`
}

// emailLoginLockout is deliberately its own collection, separate from
// emailLoginCode: requesting a fresh code deletes prior unused codes and
// their attempt counts (see handleRequestEmailLogin), so a per-code counter
// alone cannot stop someone from just asking for a new code every ten
// guesses. The lockout row survives that and blocks both endpoints for
// emailLoginLockoutTTL, keyed by email rather than by any one code.
type emailLoginLockout struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Email       string        `bson:"email"`
	LockedUntil time.Time     `bson:"locked_until"`
}

func (s *Server) handleRequestEmailLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != "cloud" {
		writeError(w, http.StatusNotFound, "email login is only available in cloud mode")
		return
	}
	if strings.TrimSpace(s.cfg.ResendAPIKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "email login is not configured")
		return
	}

	var request struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return
	}

	lockedUntil, err := s.emailLoginLocked(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare login email")
		return
	}
	if !lockedUntil.IsZero() {
		writeEmailLockoutError(w, lockedUntil)
		return
	}

	now := time.Now().UTC()
	recentCount, err := s.emailCodes.CountDocuments(r.Context(), bson.M{
		"email":      email,
		"created_at": bson.M{"$gt": now.Add(-emailLoginResendWindow)},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare login email")
		return
	}
	if recentCount > 0 {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent"})
		return
	}

	code, err := generateEmailLoginCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate login code")
		return
	}

	_, _ = s.emailCodes.DeleteMany(r.Context(), bson.M{
		"email":   email,
		"used_at": bson.M{"$exists": false},
	})

	loginCode := emailLoginCode{
		ID:        bson.NewObjectID(),
		Email:     email,
		CodeHash:  s.hashEmailLoginCode(email, code),
		CreatedAt: now,
		ExpiresAt: now.Add(emailLoginCodeTTL),
	}
	if _, err := s.emailCodes.InsertOne(r.Context(), loginCode); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare login email")
		return
	}

	if err := s.sendLoginCode(r.Context(), email, code); err != nil {
		_, _ = s.emailCodes.DeleteOne(r.Context(), bson.M{"_id": loginCode.ID})
		slog.Warn("failed to send login email", "error", err)
		writeError(w, http.StatusBadGateway, "failed to send login email")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent"})
}

func (s *Server) handleVerifyEmailLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DeploymentMode != "cloud" {
		writeError(w, http.StatusNotFound, "email login is only available in cloud mode")
		return
	}

	var request struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return
	}

	lockedUntil, err := s.emailLoginLocked(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify login code")
		return
	}
	if !lockedUntil.IsZero() {
		writeEmailLockoutError(w, lockedUntil)
		return
	}

	code := strings.TrimSpace(request.Code)
	if len(code) != 6 {
		writeError(w, http.StatusUnauthorized, "invalid or expired login code")
		return
	}

	now := time.Now().UTC()
	var loginCode emailLoginCode
	err = s.emailCodes.FindOne(
		r.Context(),
		bson.M{
			"email":      email,
			"used_at":    bson.M{"$exists": false},
			"expires_at": bson.M{"$gt": now},
			"attempts":   bson.M{"$lt": maxEmailLoginAttempts},
		},
		options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	).Decode(&loginCode)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired login code")
		return
	}

	if loginCode.CodeHash != s.hashEmailLoginCode(email, code) {
		// FindOneAndUpdate (not UpdateOne) so the post-increment count comes
		// back atomically: two concurrent wrong guesses must not both read a
		// stale count and both miss crossing the lockout threshold.
		var afterAttempt emailLoginCode
		incErr := s.emailCodes.FindOneAndUpdate(
			r.Context(),
			bson.M{"_id": loginCode.ID},
			bson.M{"$inc": bson.M{"attempts": 1}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&afterAttempt)
		if incErr == nil && afterAttempt.Attempts >= maxEmailLoginAttempts {
			if lockErr := s.lockEmailLogin(r.Context(), email, now); lockErr != nil {
				slog.Warn("failed to lock email login after repeated wrong codes", "error", lockErr)
			}
		}
		writeError(w, http.StatusUnauthorized, "invalid or expired login code")
		return
	}

	result, err := s.emailCodes.UpdateOne(r.Context(), bson.M{
		"_id":     loginCode.ID,
		"used_at": bson.M{"$exists": false},
	}, bson.M{
		"$set": bson.M{"used_at": now},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to consume login code")
		return
	}
	if result.ModifiedCount != 1 {
		writeError(w, http.StatusUnauthorized, "invalid or expired login code")
		return
	}

	user, workspaces, err := s.findOrCreateEmailUser(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign in with email")
		return
	}
	token, err := s.issueToken(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"user":       serializeUser(user),
		"workspaces": serializeWorkspaces(workspaces),
	})
}

func (s *Server) findOrCreateEmailUser(ctx context.Context, email string) (User, []Workspace, error) {
	now := time.Now().UTC()
	var user User
	err := s.users.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, nil, err
	}

	username, workspaceName, workspaceKind := workspaceDefaultsForEmail(email)
	if errors.Is(err, mongo.ErrNoDocuments) {
		username, err = s.uniqueUsername(ctx, username)
		if err != nil {
			return User{}, nil, err
		}

		workspace, err := s.createInitialWorkspace(ctx, googleProfile{
			Workspace: workspaceName,
			Kind:      workspaceKind,
		}, bson.ObjectID{})
		if err != nil {
			return User{}, nil, err
		}

		user = User{
			ID:                  bson.NewObjectID(),
			Username:            username,
			Email:               email,
			DisplayName:         username,
			AuthProvider:        "email",
			Role:                "member",
			WorkspaceIDs:        []bson.ObjectID{workspace.ID},
			OnboardingCompleted: false,
			LastLoginAt:         &now,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if _, err := s.users.InsertOne(ctx, user); err != nil {
			return User{}, nil, err
		}

		_, _ = s.workspaces.UpdateOne(ctx, bson.M{"_id": workspace.ID}, bson.M{
			"$set": bson.M{"created_by": user.ID, "updated_at": now},
		})
		workspace.CreatedBy = user.ID
		return user, []Workspace{workspace}, nil
	}

	set := bson.M{
		"email":         email,
		"last_login_at": now,
		"updated_at":    now,
	}
	if user.Username == "" {
		set["username"] = username
	}
	if user.DisplayName == "" {
		set["display_name"] = username
	}
	if user.AuthProvider == "" {
		set["auth_provider"] = "email"
	}
	if len(user.WorkspaceIDs) == 0 {
		workspace, err := s.createInitialWorkspace(ctx, googleProfile{
			Workspace: workspaceName,
			Kind:      workspaceKind,
		}, user.ID)
		if err != nil {
			return User{}, nil, err
		}
		set["workspace_ids"] = []bson.ObjectID{workspace.ID}
	}

	result := s.users.FindOneAndUpdate(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if err := result.Decode(&user); err != nil {
		return User{}, nil, err
	}

	workspaces, err := s.getVisibleWorkspaces(ctx, user)
	if err != nil {
		return User{}, nil, err
	}

	return user, workspaces, nil
}

// emailLoginLocked returns the lockout's expiry if email is currently locked
// out of the login-code endpoints, or the zero Time if it is not. It checks
// the expiry itself rather than trusting that a matching document means an
// active lockout, since the TTL index that removes expired lockouts runs on
// a background sweep and is not instantaneous.
func (s *Server) emailLoginLocked(ctx context.Context, email string) (time.Time, error) {
	var lockout emailLoginLockout
	err := s.emailLockouts.FindOne(ctx, bson.M{"email": email}).Decode(&lockout)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	if !time.Now().UTC().Before(lockout.LockedUntil) {
		return time.Time{}, nil
	}
	return lockout.LockedUntil, nil
}

// lockEmailLogin locks email out of the login-code endpoints until
// now+emailLoginLockoutTTL. Upserted (rather than inserted) so a repeat
// offender within the same lockout window simply extends it instead of
// erroring on the unique email index.
func (s *Server) lockEmailLogin(ctx context.Context, email string, now time.Time) error {
	_, err := s.emailLockouts.UpdateOne(
		ctx,
		bson.M{"email": email},
		bson.M{
			"$set": bson.M{"locked_until": now.Add(emailLoginLockoutTTL)},
			"$setOnInsert": bson.M{
				"_id":   bson.NewObjectID(),
				"email": email,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

// emailLockoutRetryMinutes formats the remaining lockout duration for the
// error message. Rounded to the nearest minute, and never below one, so the
// message never claims "try again in 0 minutes" while still locked.
func emailLockoutRetryMinutes(lockedUntil, now time.Time) int {
	minutes := int(lockedUntil.Sub(now).Round(time.Minute).Minutes())
	if minutes < 1 {
		return 1
	}
	return minutes
}

func writeEmailLockoutError(w http.ResponseWriter, lockedUntil time.Time) {
	minutes := emailLockoutRetryMinutes(lockedUntil, time.Now().UTC())
	writeError(w, http.StatusTooManyRequests, fmt.Sprintf(
		"too many incorrect codes; try again in %d minute(s)", minutes,
	))
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return "", errors.New("invalid email")
	}

	return value, nil
}

func generateEmailLoginCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", value.Int64()), nil
}

func (s *Server) hashEmailLoginCode(email, code string) string {
	hasher := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	_, _ = hasher.Write([]byte(email + ":" + code))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (s *Server) sendLoginCode(ctx context.Context, email, code string) error {
	payload := map[string]any{
		"from":    s.cfg.ResendFromEmail,
		"to":      []string{email},
		"subject": "Seu código de acesso da Runtz",
		"html": fmt.Sprintf(
			`<p>Use o código abaixo para acessar a Runtz:</p><p style="font-size: 24px; font-weight: 700; letter-spacing: 4px">%s</p><p>Ele expira em 10 minutos.</p>`,
			html.EscapeString(code),
		),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+s.cfg.ResendAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("resend returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}

	return nil
}
