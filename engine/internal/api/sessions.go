package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// sessionCookieName carries the browser session. It is opaque: the value
	// is a random string whose SHA-256 is the only thing stored server-side,
	// so nothing about the user can be read out of it and nothing about it can
	// be forged without guessing 256 bits.
	sessionCookieName = "runtz_session"

	// sessionTTL is how long a browser session stays signed in.
	sessionTTL = 7 * 24 * time.Hour

	// sessionTouchInterval throttles last_used_at writes: bumping it on every
	// single request would turn a read-only page load into a write.
	sessionTouchInterval = 5 * time.Minute
)

// Session is a browser login. The raw token is never stored — only its hash —
// so a database leak yields no usable session, the same reasoning that applies
// to APIKey.KeyHash. A fast hash is the right choice here (and not bcrypt as
// for passwords): the token is 256 bits of entropy, so there is no dictionary
// to defend against, only a search space nobody can walk.
type Session struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	UserID     bson.ObjectID `bson:"user_id"`
	TokenHash  string        `bson:"token_hash"`
	UserAgent  string        `bson:"user_agent,omitempty"`
	CreatedAt  time.Time     `bson:"created_at"`
	LastUsedAt time.Time     `bson:"last_used_at"`
	ExpiresAt  time.Time     `bson:"expires_at"`
}

func hashSessionToken(rawToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawToken)))
	return hex.EncodeToString(sum[:])
}

// issueSession creates a session for user and returns the raw token to hand to
// the browser. The raw value exists only in this return and in the cookie; it
// is never written down.
func (s *Server) issueSession(ctx context.Context, user User, r *http.Request) (string, error) {
	rawToken, err := randomHex(32)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	session := Session{
		ID:         bson.NewObjectID(),
		UserID:     user.ID,
		TokenHash:  hashSessionToken(rawToken),
		UserAgent:  truncate(r.UserAgent(), 200),
		CreatedAt:  now,
		LastUsedAt: now,
		ExpiresAt:  now.Add(sessionTTL),
	}
	if _, err := s.sessions.InsertOne(ctx, session); err != nil {
		return "", err
	}

	return rawToken, nil
}

// userFromSession resolves the user behind a raw session token. The expiry is
// filtered here rather than left to the TTL index, which sweeps in the
// background and is not instantaneous.
func (s *Server) userFromSession(ctx context.Context, rawToken string) (User, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return User{}, errors.New("missing session token")
	}

	var session Session
	err := s.sessions.FindOne(ctx, bson.M{
		"token_hash": hashSessionToken(rawToken),
		"expires_at": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&session)
	if err != nil {
		return User{}, errors.New("invalid session")
	}

	var user User
	if err := s.users.FindOne(ctx, bson.M{"_id": session.UserID}).Decode(&user); err != nil {
		return User{}, errors.New("user not found")
	}

	s.touchSession(ctx, session)
	return user, nil
}

func (s *Server) touchSession(ctx context.Context, session Session) {
	now := time.Now().UTC()
	if now.Sub(session.LastUsedAt) < sessionTouchInterval {
		return
	}

	_, _ = s.sessions.UpdateOne(
		ctx,
		bson.M{"_id": session.ID},
		bson.M{"$set": bson.M{"last_used_at": now}},
	)
}

// revokeSession deletes a single session. Deleting rather than flagging keeps
// the collection self-cleaning, and there is no audit requirement that would
// justify keeping spent rows around.
func (s *Server) revokeSession(ctx context.Context, rawToken string) error {
	_, err := s.sessions.DeleteOne(ctx, bson.M{"token_hash": hashSessionToken(rawToken)})
	return err
}

func sessionTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// sessionCookieSecure marks the cookie Secure whenever the platform is served
// over HTTPS. It cannot be unconditional: a Secure cookie is dropped by the
// browser on plain http, which is exactly how the self-hosted quickstart runs
// on localhost.
func (s *Server) sessionCookieSecure() bool {
	return strings.HasPrefix(strings.ToLower(s.cfg.PublicURL), "https://")
}

// setSessionCookie writes the session cookie. SameSite=Lax is what stands in
// for CSRF tokens here: it withholds the cookie from cross-site POST/PATCH,
// and no GET in this API mutates state. It works because the browser reaches
// the engine same-origin through the frontend proxy, so the cookie is
// first-party and needs neither SameSite=None nor HTTPS in development.
func (s *Server) setSessionCookie(w http.ResponseWriter, rawToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.sessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.sessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// startSession issues the session and sets the cookie. Login handlers call
// this instead of returning a token in the body — the browser never sees the
// credential in JavaScript-readable form.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user User) error {
	rawToken, err := s.issueSession(r.Context(), user, r)
	if err != nil {
		return err
	}
	s.setSessionCookie(w, rawToken)
	return nil
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if rawToken := sessionTokenFromRequest(r); rawToken != "" {
		if err := s.revokeSession(r.Context(), rawToken); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to end session")
			return
		}
	}

	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "signed_out"})
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}
