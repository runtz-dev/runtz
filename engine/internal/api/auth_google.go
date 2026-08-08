package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/api/idtoken"
)

var personalEmailDomains = map[string]bool{
	"aol.com":        true,
	"gmail.com":      true,
	"googlemail.com": true,
	"hotmail.com":    true,
	"icloud.com":     true,
	"live.com":       true,
	"mac.com":        true,
	"me.com":         true,
	"msn.com":        true,
	"outlook.com":    true,
	"proton.me":      true,
	"protonmail.com": true,
	"yahoo.com":      true,
	"yahoo.com.br":   true,
}

type googleProfile struct {
	Subject   string
	Email     string
	Name      string
	Picture   string
	Username  string
	Workspace string
	Kind      string
}

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.cfg.GoogleClientID) == "" {
		writeError(w, http.StatusServiceUnavailable, "google login is not configured")
		return
	}
	if s.cfg.DeploymentMode == hostingSelfHosted && !s.hasFeature(r.Context(), nil, featureGoogleAuth) {
		writeError(w, http.StatusPaymentRequired, "google login is not available for this license")
		return
	}

	var request struct {
		Credential string `json:"credential"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	profile, err := s.validateGoogleCredential(r, strings.TrimSpace(request.Credential))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	user, workspaces, err := s.findOrCreateGoogleUser(r.Context(), profile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign in with google")
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

func (s *Server) validateGoogleCredential(r *http.Request, credential string) (googleProfile, error) {
	if credential == "" {
		return googleProfile{}, errors.New("missing google credential")
	}

	payload, err := idtoken.Validate(r.Context(), credential, s.cfg.GoogleClientID)
	if err != nil {
		return googleProfile{}, errors.New("invalid google credential")
	}

	email := claimString(payload.Claims, "email")
	if email == "" || !claimBool(payload.Claims, "email_verified") {
		return googleProfile{}, errors.New("google email is not verified")
	}

	subject := strings.TrimSpace(payload.Subject)
	if subject == "" {
		return googleProfile{}, errors.New("google subject is missing")
	}

	username, workspaceName, workspaceKind := workspaceDefaultsForEmail(email)
	return googleProfile{
		Subject:   subject,
		Email:     strings.ToLower(strings.TrimSpace(email)),
		Name:      claimString(payload.Claims, "name"),
		Picture:   claimString(payload.Claims, "picture"),
		Username:  username,
		Workspace: workspaceName,
		Kind:      workspaceKind,
	}, nil
}

func (s *Server) findOrCreateGoogleUser(ctx context.Context, profile googleProfile) (User, []Workspace, error) {
	now := time.Now().UTC()
	filter := bson.M{"google_subject": profile.Subject}

	var user User
	err := s.users.FindOne(ctx, filter).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = s.users.FindOne(ctx, bson.M{"email": profile.Email}).Decode(&user)
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = s.users.FindOne(ctx, bson.M{"username": profile.Email}).Decode(&user)
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, nil, err
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		username, err := s.uniqueUsername(ctx, profile.Username)
		if err != nil {
			return User{}, nil, err
		}

		workspace, err := s.createInitialWorkspace(ctx, profile, bson.ObjectID{})
		if err != nil {
			return User{}, nil, err
		}

		user = User{
			ID:                    bson.NewObjectID(),
			Username:              username,
			Email:                 profile.Email,
			DisplayName:           profile.Name,
			AvatarURL:             profile.Picture,
			AuthProvider:          "google",
			GoogleSubject:         profile.Subject,
			Role:                  "member",
			WorkspaceIDs:          []bson.ObjectID{workspace.ID},
			RequirePasswordChange: false,
			LastLoginAt:           &now,
			CreatedAt:             now,
			UpdatedAt:             now,
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
		"email":                   profile.Email,
		"display_name":            profile.Name,
		"avatar_url":              profile.Picture,
		"auth_provider":           "google",
		"google_subject":          profile.Subject,
		"require_password_change": false,
		"last_login_at":           now,
		"updated_at":              now,
	}
	if user.Role == "" {
		set["role"] = "member"
	}
	if user.Username == "" {
		set["username"] = profile.Username
	}

	if len(user.WorkspaceIDs) == 0 {
		workspace, err := s.createInitialWorkspace(ctx, profile, user.ID)
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

func (s *Server) createInitialWorkspace(ctx context.Context, profile googleProfile, createdBy bson.ObjectID) (Workspace, error) {
	now := time.Now().UTC()
	slug, err := s.uniqueWorkspaceSlug(ctx, slugify(profile.Workspace))
	if err != nil {
		return Workspace{}, err
	}

	workspace := Workspace{
		ID:        bson.NewObjectID(),
		Name:      profile.Workspace,
		Slug:      slug,
		Kind:      profile.Kind,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.workspaces.InsertOne(ctx, workspace); err != nil {
		return Workspace{}, err
	}

	return workspace, nil
}

func (s *Server) uniqueUsername(ctx context.Context, base string) (string, error) {
	base = slugify(base)
	if base == "workspace" {
		base = "user"
	}

	for i := 0; i < 50; i++ {
		username := base
		if i > 0 {
			username = fmt.Sprintf("%s-%d", base, i+1)
		}

		count, err := s.users.CountDocuments(ctx, bson.M{"username": username})
		if err != nil {
			return "", err
		}
		if count == 0 {
			return username, nil
		}
	}

	return "", errors.New("could not generate username")
}

func (s *Server) uniqueWorkspaceSlug(ctx context.Context, base string) (string, error) {
	base = slugify(base)
	for i := 0; i < 50; i++ {
		slug := base
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", base, i+1)
		}

		count, err := s.workspaces.CountDocuments(ctx, bson.M{"slug": slug})
		if err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
	}

	return "", errors.New("could not generate workspace slug")
}

func workspaceDefaultsForEmail(email string) (username, workspaceName, workspaceKind string) {
	localPart, domain, ok := splitEmail(email)
	if !ok {
		return "user", "personal", "personal"
	}

	username = slugify(localPart)
	if personalEmailDomains[domain] {
		return username, "personal", "personal"
	}

	label := registrableDomainLabel(domain)
	label = slugify(label)
	if label == "workspace" {
		label = username
	}

	return username, label, "company"
}

func registrableDomainLabel(domain string) string {
	labels := strings.Split(strings.ToLower(strings.TrimSpace(domain)), ".")
	if len(labels) == 0 {
		return domain
	}
	if len(labels) >= 3 && len(labels[len(labels)-1]) == 2 {
		switch labels[len(labels)-2] {
		case "com", "co", "net", "org", "edu", "gov":
			return labels[len(labels)-3]
		}
	}
	if len(labels) >= 2 {
		return labels[len(labels)-2]
	}

	return labels[0]
}

func splitEmail(email string) (localPart, domain string, ok bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(email)), "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func claimString(claims map[string]any, key string) string {
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(value)
}

func claimBool(claims map[string]any, key string) bool {
	value, ok := claims[key].(bool)
	return ok && value
}
