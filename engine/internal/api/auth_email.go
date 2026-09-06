package api

import (
	"bytes"
	"context"
	"crypto/rand"
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
	"golang.org/x/crypto/bcrypt"
)

const (
	emailLoginCodeTTL      = 10 * time.Minute
	emailLoginResendWindow = time.Minute
	// maxEmailLoginAttempts wrong guesses kill the code (see the "attempts"
	// filter in handleVerifyEmailLogin) and, on the same guess that reaches
	// the limit, lock the email out of requesting or verifying any code for
	// emailLoginLockoutTTL — see lockLogin.
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

	lockedUntil, err := s.loginLocked(r.Context(), emailLockoutKey(email))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare login email")
		return
	}
	if !lockedUntil.IsZero() {
		writeLockoutError(w, lockedUntil, "codes")
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

	codeHash, err := hashEmailLoginCode(email, code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare login email")
		return
	}

	loginCode := emailLoginCode{
		ID:        bson.NewObjectID(),
		Email:     email,
		CodeHash:  codeHash,
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

	lockedUntil, err := s.loginLocked(r.Context(), emailLockoutKey(email))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify login code")
		return
	}
	if !lockedUntil.IsZero() {
		writeLockoutError(w, lockedUntil, "codes")
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

	if !emailLoginCodeMatches(loginCode.CodeHash, email, code) {
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
			if lockErr := s.lockLogin(r.Context(), emailLockoutKey(email), now, emailLoginLockoutTTL); lockErr != nil {
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
	if err := s.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
	if s.cfg.DeploymentMode != hostingCloud && len(user.WorkspaceIDs) == 0 {
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

// hashEmailLoginCode hashes a login code for storage. bcrypt — and not the
// fast SHA-256 used for session and API key tokens — because this credential is
// six digits: a leaked database of fast hashes would be reversed instantly by
// walking all 10^6, while bcrypt makes that cost hours against a code that
// lives ten minutes and dies after maxEmailLoginAttempts guesses. The email is
// mixed in so a hash cannot be replayed against a different address.
func hashEmailLoginCode(email, code string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(email+":"+code), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func emailLoginCodeMatches(storedHash, email, code string) bool {
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(email+":"+code)) == nil
}

// loginCodeEmailHTML is the transactional email sent for passwordless
// sign-in. It's a self-contained, inline-styled HTML string (no external
// assets, no web fonts) so it renders consistently across email clients,
// styled to match the runtz.dev / app brand's dark surface — canvas
// #050912, ink #eaf4ff, accent #6db5ff — with JetBrains Mono for the
// wordmark and the code itself.
const loginCodeEmailHTML = `<div style="display:none;max-height:0;overflow:hidden;opacity:0;">Your runtz sign-in code — expires in 10 minutes.</div>
<div style="background-color:#050912;padding:40px 16px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Inter,Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:440px;margin:0 auto;">
    <tr>
      <td style="padding-bottom:24px;text-align:center;">
        <span style="font-family:ui-monospace,'JetBrains Mono',SFMono-Regular,Menlo,monospace;font-weight:700;font-size:20px;letter-spacing:-0.05em;color:#eaf4ff;">runtz<span style="display:inline-block;width:9px;height:16px;background-color:#6db5ff;margin-left:2px;vertical-align:middle;"></span></span>
      </td>
    </tr>
    <tr>
      <td style="background-color:#0d1420;border:1px solid #223149;border-radius:12px;padding:40px 32px;text-align:center;">
        <p style="margin:0 0 24px;font-size:15px;line-height:1.5;color:#eaf4ff;">Use the code below to sign in to runtz platform:</p>
        <div style="display:inline-block;background-color:#111b2b;border:1px solid #223149;border-radius:8px;padding:16px 24px;">
          <span style="font-family:ui-monospace,'JetBrains Mono',SFMono-Regular,Menlo,monospace;font-size:28px;font-weight:700;letter-spacing:6px;color:#6db5ff;">%s</span>
        </div>
        <p style="margin:24px 0 0;font-size:13px;color:#8ba3c2;">Expires in 10 minutes. If you didn't request this, you can ignore this email.</p>
      </td>
    </tr>
    <tr>
      <td style="padding-top:24px;text-align:center;">
        <p style="margin:0;font-size:12px;color:#8ba3c2;">runtz.dev</p>
      </td>
    </tr>
  </table>
</div>`

func (s *Server) sendLoginCode(ctx context.Context, email, code string) error {
	payload := map[string]any{
		"from":    s.cfg.ResendFromEmail,
		"to":      []string{email},
		"subject": "Your runtz access code",
		"html":    fmt.Sprintf(loginCodeEmailHTML, html.EscapeString(code)),
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
