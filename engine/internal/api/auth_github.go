package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	githubAccessTokenURL = "https://github.com/login/oauth/access_token"
	githubAPIURL         = "https://api.github.com"
	githubAPIVersion     = "2022-11-28"
)

type githubProfile struct {
	Subject string
	Email   string
	Name    string
}

type githubOAuthToken struct {
	AccessToken      string `json:"access_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (s *Server) handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.cfg.GitHubClientID) == "" || strings.TrimSpace(s.cfg.GitHubClientSecret) == "" {
		writeError(w, http.StatusServiceUnavailable, "github login is not configured")
		return
	}
	// GitHub sign-in is cloud-only: featuresForPlan never grants
	// featureGitHubAuth in self-hosted, on any plan.
	if s.cfg.DeploymentMode == hostingSelfHosted && !s.hasFeature(r.Context(), nil, featureGitHubAuth) {
		writeError(w, http.StatusNotFound, "github login is not available for self-hosted deployments")
		return
	}

	var request struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirectUri"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	redirectURI := strings.TrimSpace(request.RedirectURI)
	if !s.githubRedirectURIAllowed(redirectURI) {
		writeError(w, http.StatusBadRequest, "invalid github redirect uri")
		return
	}

	accessToken, err := s.exchangeGitHubCode(r.Context(), strings.TrimSpace(request.Code), redirectURI)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid github authorization code")
		return
	}

	profile, err := s.fetchGitHubProfile(r.Context(), accessToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	user, workspaces, err := s.findOrCreateGitHubUser(r.Context(), profile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign in with github")
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

func (s *Server) githubRedirectURIAllowed(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "/login/github/callback" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}

	origin := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	for _, allowed := range s.cfg.CORSAllowedOrigins {
		if allowed == "*" || strings.EqualFold(strings.TrimRight(allowed, "/"), origin) {
			return true
		}
	}

	return false
}

func (s *Server) exchangeGitHubCode(ctx context.Context, code, redirectURI string) (string, error) {
	if code == "" {
		return "", errors.New("missing github authorization code")
	}

	form := url.Values{
		"client_id":     {s.cfg.GitHubClientID},
		"client_secret": {s.cfg.GitHubClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, githubAccessTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response githubOAuthToken
	if err := sendGitHubRequest(request, &response); err != nil {
		return "", err
	}
	if response.Error != "" || strings.TrimSpace(response.AccessToken) == "" {
		return "", errors.New("github rejected authorization code")
	}

	return response.AccessToken, nil
}

func (s *Server) fetchGitHubProfile(ctx context.Context, accessToken string) (githubProfile, error) {
	var user githubUser
	if err := fetchGitHubJSON(ctx, accessToken, "/user", &user); err != nil {
		return githubProfile{}, err
	}
	if user.ID == 0 {
		return githubProfile{}, errors.New("github user id is missing")
	}

	var emails []githubEmail
	if err := fetchGitHubJSON(ctx, accessToken, "/user/emails", &emails); err != nil {
		return githubProfile{}, err
	}
	for _, email := range emails {
		if email.Primary && email.Verified {
			normalizedEmail, err := normalizeEmail(email.Email)
			if err != nil {
				return githubProfile{}, errors.New("github primary email is invalid")
			}
			name := strings.TrimSpace(user.Name)
			if name == "" {
				name = strings.TrimSpace(user.Login)
			}

			return githubProfile{
				Subject: strconv.FormatInt(user.ID, 10),
				Email:   normalizedEmail,
				Name:    name,
			}, nil
		}
	}

	return githubProfile{}, errors.New("github primary email is not verified")
}

func fetchGitHubJSON(ctx context.Context, accessToken, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

	return sendGitHubRequest(request, target)
}

func sendGitHubRequest(request *http.Request, target any) error {
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("github request failed with status %d", response.StatusCode)
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}

	return nil
}

func (s *Server) findOrCreateGitHubUser(ctx context.Context, profile githubProfile) (User, []Workspace, error) {
	now := time.Now().UTC()
	var user User
	err := s.users.FindOne(ctx, bson.M{"github_subject": profile.Subject}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = s.users.FindOne(ctx, bson.M{"email": profile.Email}).Decode(&user)
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, nil, err
	}

	username, workspaceName, workspaceKind := workspaceDefaultsForEmail(profile.Email)
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
			Email:               profile.Email,
			DisplayName:         profile.Name,
			AuthProvider:        "github",
			GitHubSubject:       profile.Subject,
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
		"email":          profile.Email,
		"auth_provider":  "github",
		"github_subject": profile.Subject,
		"last_login_at":  now,
		"updated_at":     now,
	}
	if user.Role == "" {
		set["role"] = "member"
	}
	if user.Username == "" {
		set["username"] = username
	}
	if user.DisplayName == "" {
		set["display_name"] = profile.Name
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
