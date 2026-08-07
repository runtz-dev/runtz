package api

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Workspace struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string        `bson:"name" json:"name"`
	Slug      string        `bson:"slug" json:"slug"`
	Kind      string        `bson:"kind,omitempty" json:"kind,omitempty"`
	CreatedBy bson.ObjectID `bson:"created_by,omitempty" json:"createdBy,omitempty"`
	CreatedAt time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time     `bson:"updated_at" json:"updatedAt"`
}

type User struct {
	ID                    bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Username              string          `bson:"username" json:"username"`
	Email                 string          `bson:"email,omitempty" json:"email,omitempty"`
	DisplayName           string          `bson:"display_name,omitempty" json:"displayName,omitempty"`
	AvatarURL             string          `bson:"avatar_url,omitempty" json:"avatarUrl,omitempty"`
	AuthProvider          string          `bson:"auth_provider,omitempty" json:"authProvider,omitempty"`
	GoogleSubject         string          `bson:"google_subject,omitempty" json:"-"`
	GitHubSubject         string          `bson:"github_subject,omitempty" json:"-"`
	PasswordHash          string          `bson:"password_hash" json:"-"`
	Role                  string          `bson:"role" json:"role"`
	WorkspaceIDs          []bson.ObjectID `bson:"workspace_ids" json:"workspaceIds"`
	RequirePasswordChange bool            `bson:"require_password_change" json:"requirePasswordChange"`
	OnboardingCompleted   bool            `bson:"onboarding_completed" json:"onboardingCompleted"`
	LastLoginAt           *time.Time      `bson:"last_login_at,omitempty" json:"lastLoginAt,omitempty"`
	CreatedAt             time.Time       `bson:"created_at" json:"createdAt"`
	UpdatedAt             time.Time       `bson:"updated_at" json:"updatedAt"`
}

type APIKey struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	WorkspaceID bson.ObjectID `bson:"workspace_id" json:"workspaceId"`
	Name        string        `bson:"name" json:"name"`
	Prefix      string        `bson:"prefix" json:"prefix"`
	KeyHash     string        `bson:"key_hash" json:"-"`
	Scopes      []string      `bson:"scopes,omitempty" json:"scopes,omitempty"`
	CreatedBy   bson.ObjectID `bson:"created_by" json:"createdBy"`
	LastUsedAt  *time.Time    `bson:"last_used_at,omitempty" json:"lastUsedAt,omitempty"`
	RevokedAt   *time.Time    `bson:"revoked_at,omitempty" json:"revokedAt,omitempty"`
	ExpiresAt   *time.Time    `bson:"expires_at,omitempty" json:"expiresAt,omitempty"`
	CreatedAt   time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updatedAt"`
}

type BillingSubscription struct {
	ID                      bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID                  bson.ObjectID `bson:"user_id,omitempty" json:"userId,omitempty"`
	Email                   string        `bson:"email,omitempty" json:"email,omitempty"`
	Plan                    string        `bson:"plan" json:"plan"`
	DeploymentMode          string        `bson:"deployment_mode" json:"deploymentMode"`
	Status                  string        `bson:"status" json:"status"`
	StripeCustomerID        string        `bson:"stripe_customer_id,omitempty" json:"stripeCustomerId,omitempty"`
	StripeSubscriptionID    string        `bson:"stripe_subscription_id,omitempty" json:"stripeSubscriptionId,omitempty"`
	StripeCheckoutSessionID string        `bson:"stripe_checkout_session_id,omitempty" json:"stripeCheckoutSessionId,omitempty"`
	StripePriceID           string        `bson:"stripe_price_id,omitempty" json:"stripePriceId,omitempty"`
	CurrentPeriodEnd        *time.Time    `bson:"current_period_end,omitempty" json:"currentPeriodEnd,omitempty"`
	CancelAtPeriodEnd       bool          `bson:"cancel_at_period_end,omitempty" json:"cancelAtPeriodEnd,omitempty"`
	LicenseKeyHash          string        `bson:"license_key_hash,omitempty" json:"-"`
	LicenseKeyPrefix        string        `bson:"license_key_prefix,omitempty" json:"licenseKeyPrefix,omitempty"`
	InstallationID          string        `bson:"installation_id,omitempty" json:"installationId,omitempty"`
	LastHeartbeatAt         *time.Time    `bson:"last_heartbeat_at,omitempty" json:"lastHeartbeatAt,omitempty"`
	CreatedAt               time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt               time.Time     `bson:"updated_at" json:"updatedAt"`
}

type InstanceState struct {
	ID                  bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	Key                 string         `bson:"key" json:"key"`
	InstallationID      string         `bson:"installation_id" json:"installationId"`
	LicenseKey          string         `bson:"license_key,omitempty" json:"-"`
	CheckoutSessionID   string         `bson:"checkout_session_id,omitempty" json:"-"`
	LicenseKeyPrefix    string         `bson:"license_key_prefix,omitempty" json:"licenseKeyPrefix,omitempty"`
	LicensePayload      LicensePayload `bson:"license_payload,omitempty" json:"licensePayload,omitempty"`
	LicensePayloadRaw   string         `bson:"license_payload_raw,omitempty" json:"-"`
	LicenseSignature    string         `bson:"license_signature,omitempty" json:"-"`
	LastValidatedAt     *time.Time     `bson:"last_validated_at,omitempty" json:"lastValidatedAt,omitempty"`
	LastValidationError string         `bson:"last_validation_error,omitempty" json:"lastValidationError,omitempty"`
	CreatedAt           time.Time      `bson:"created_at" json:"createdAt"`
	UpdatedAt           time.Time      `bson:"updated_at" json:"updatedAt"`
}

type LicensePayload struct {
	LicenseKeyPrefix string    `bson:"license_key_prefix,omitempty" json:"licenseKeyPrefix,omitempty"`
	InstallationID   string    `bson:"installation_id,omitempty" json:"installationId,omitempty"`
	Plan             string    `bson:"plan,omitempty" json:"plan,omitempty"`
	DeploymentMode   string    `bson:"deployment_mode,omitempty" json:"deploymentMode,omitempty"`
	Status           string    `bson:"status,omitempty" json:"status,omitempty"`
	Features         []string  `bson:"features,omitempty" json:"features,omitempty"`
	IssuedAt         time.Time `bson:"issued_at,omitempty" json:"issuedAt,omitempty"`
	ExpiresAt        time.Time `bson:"expires_at,omitempty" json:"expiresAt,omitempty"`
	CurrentPeriodEnd string    `bson:"current_period_end,omitempty" json:"currentPeriodEnd,omitempty"`
}

type Dependency struct {
	Name            string `bson:"name" json:"name"`
	RequestedRange  string `bson:"requested_range" json:"requestedRange"`
	ResolvedVersion string `bson:"resolved_version" json:"resolvedVersion"`
	Scope           string `bson:"scope" json:"scope"`
	Ecosystem       string `bson:"ecosystem" json:"ecosystem"`
	File            string `bson:"file,omitempty" json:"file,omitempty"`
}

type Vulnerability struct {
	ID                  string    `bson:"id" json:"id"`
	GHSAID              string    `bson:"ghsa_id,omitempty" json:"ghsaId,omitempty"`
	CVEID               string    `bson:"cve_id,omitempty" json:"cveId,omitempty"`
	PackageName         string    `bson:"package_name" json:"packageName"`
	InstalledPackage    string    `bson:"installed_package,omitempty" json:"installedPackage,omitempty"`
	SourcePackage       string    `bson:"source_package,omitempty" json:"sourcePackage,omitempty"`
	Ecosystem           string    `bson:"ecosystem" json:"ecosystem"`
	InstalledVersion    string    `bson:"installed_version" json:"installedVersion"`
	VulnerableRange     string    `bson:"vulnerable_range" json:"vulnerableRange"`
	FirstPatchedVersion string    `bson:"first_patched_version,omitempty" json:"firstPatchedVersion,omitempty"`
	Severity            string    `bson:"severity" json:"severity"`
	Summary             string    `bson:"summary" json:"summary"`
	AdvisoryURL         string    `bson:"advisory_url" json:"advisoryUrl"`
	CVSSScore           float64   `bson:"cvss_score,omitempty" json:"cvssScore,omitempty"`
	References          []string  `bson:"references,omitempty" json:"references,omitempty"`
	PublishedAt         time.Time `bson:"published_at,omitempty" json:"publishedAt,omitempty"`
	UpdatedAt           time.Time `bson:"updated_at,omitempty" json:"updatedAt,omitempty"`
}

type Package struct {
	Name          string `bson:"name" json:"name"`
	Version       string `bson:"version" json:"version"`
	Architecture  string `bson:"architecture,omitempty" json:"architecture,omitempty"`
	SourceName    string `bson:"source_name,omitempty" json:"sourceName,omitempty"`
	SourceVersion string `bson:"source_version,omitempty" json:"sourceVersion,omitempty"`
	Manager       string `bson:"manager" json:"manager"`
}

type Finding struct {
	ID           string `bson:"id" json:"id"`
	Title        string `bson:"title" json:"title"`
	Description  string `bson:"description,omitempty" json:"description,omitempty"`
	Severity     string `bson:"severity" json:"severity"`
	Category     string `bson:"category,omitempty" json:"category,omitempty"`
	File         string `bson:"file,omitempty" json:"file,omitempty"`
	Line         int    `bson:"line,omitempty" json:"line,omitempty"`
	Column       int    `bson:"column,omitempty" json:"column,omitempty"`
	ResourceKind string `bson:"resource_kind,omitempty" json:"resourceKind,omitempty"`
	ResourceName string `bson:"resource_name,omitempty" json:"resourceName,omitempty"`
	Namespace    string `bson:"namespace,omitempty" json:"namespace,omitempty"`
	Remediation  string `bson:"remediation,omitempty" json:"remediation,omitempty"`
}

type ScanSummary struct {
	TotalDependencies int                 `bson:"total_dependencies" json:"totalDependencies"`
	Vulnerabilities   int                 `bson:"vulnerabilities" json:"vulnerabilities"`
	Critical          int                 `bson:"critical" json:"critical"`
	High              int                 `bson:"high" json:"high"`
	Medium            int                 `bson:"medium" json:"medium"`
	Low               int                 `bson:"low" json:"low"`
	Unknown           int                 `bson:"unknown" json:"unknown"`
	WithFix           VulnerabilityCounts `bson:"with_fix,omitempty" json:"withFix"`
	WithoutFix        VulnerabilityCounts `bson:"without_fix,omitempty" json:"withoutFix"`
	FixStatusComputed bool                `bson:"fix_status_computed,omitempty" json:"fixStatusComputed"`
}

type VulnerabilityCounts struct {
	Vulnerabilities int `bson:"vulnerabilities" json:"vulnerabilities"`
	Critical        int `bson:"critical" json:"critical"`
	High            int `bson:"high" json:"high"`
	Medium          int `bson:"medium" json:"medium"`
	Low             int `bson:"low" json:"low"`
	Unknown         int `bson:"unknown" json:"unknown"`
}

type Scan struct {
	ID               bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Type             string          `bson:"type" json:"type"`
	WorkspaceID      bson.ObjectID   `bson:"workspace_id" json:"workspaceId"`
	WorkspaceName    string          `bson:"workspace_name" json:"workspaceName"`
	ProjectName      string          `bson:"project_name" json:"projectName"`
	TargetName       string          `bson:"target_name,omitempty" json:"targetName,omitempty"`
	Hostname         string          `bson:"hostname,omitempty" json:"hostname,omitempty"`
	ImageName        string          `bson:"image_name,omitempty" json:"imageName,omitempty"`
	ImageRef         string          `bson:"image_ref,omitempty" json:"imageRef,omitempty"`
	ImageDigest      string          `bson:"image_digest,omitempty" json:"imageDigest,omitempty"`
	Source           string          `bson:"source" json:"source"`
	TargetFile       string          `bson:"target_file" json:"targetFile"`
	Status           string          `bson:"status" json:"status"`
	OSID             string          `bson:"os_id,omitempty" json:"osId,omitempty"`
	OSName           string          `bson:"os_name,omitempty" json:"osName,omitempty"`
	OSVersion        string          `bson:"os_version,omitempty" json:"osVersion,omitempty"`
	OSCodename       string          `bson:"os_codename,omitempty" json:"osCodename,omitempty"`
	PackageManager   string          `bson:"package_manager,omitempty" json:"packageManager,omitempty"`
	ScannerVersion   string          `bson:"scanner_version" json:"scannerVersion"`
	FilesScanned     int             `bson:"files_scanned,omitempty" json:"filesScanned,omitempty"`
	ResourcesScanned int             `bson:"resources_scanned,omitempty" json:"resourcesScanned,omitempty"`
	Summary          ScanSummary     `bson:"summary" json:"summary"`
	Dependencies     []Dependency    `bson:"dependencies" json:"dependencies"`
	Packages         []Package       `bson:"packages,omitempty" json:"packages,omitempty"`
	Findings         []Finding       `bson:"findings,omitempty" json:"findings,omitempty"`
	Vulnerabilities  []Vulnerability `bson:"vulnerabilities" json:"vulnerabilities"`
	CreatedAt        time.Time       `bson:"created_at" json:"createdAt"`
}

type publicUser struct {
	ID                    string   `json:"id"`
	Username              string   `json:"username"`
	Email                 string   `json:"email,omitempty"`
	DisplayName           string   `json:"displayName,omitempty"`
	AvatarURL             string   `json:"avatarUrl,omitempty"`
	AuthProvider          string   `json:"authProvider,omitempty"`
	Role                  string   `json:"role"`
	WorkspaceIDs          []string `json:"workspaceIds"`
	RequirePasswordChange bool     `json:"requirePasswordChange"`
	OnboardingCompleted   bool     `json:"onboardingCompleted"`
	LastLoginAt           string   `json:"lastLoginAt,omitempty"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
}

type publicWorkspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Kind      string `json:"kind,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type publicAPIKey struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspaceId"`
	Name        string   `json:"name"`
	Prefix      string   `json:"prefix"`
	Scopes      []string `json:"scopes,omitempty"`
	CreatedBy   string   `json:"createdBy"`
	LastUsedAt  string   `json:"lastUsedAt,omitempty"`
	RevokedAt   string   `json:"revokedAt,omitempty"`
	ExpiresAt   string   `json:"expiresAt,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

func serializeUser(user User) publicUser {
	workspaceIDs := make([]string, 0, len(user.WorkspaceIDs))
	for _, workspaceID := range user.WorkspaceIDs {
		workspaceIDs = append(workspaceIDs, workspaceID.Hex())
	}

	response := publicUser{
		ID:                    user.ID.Hex(),
		Username:              user.Username,
		Email:                 user.Email,
		DisplayName:           user.DisplayName,
		AvatarURL:             user.AvatarURL,
		AuthProvider:          user.AuthProvider,
		Role:                  user.Role,
		WorkspaceIDs:          workspaceIDs,
		RequirePasswordChange: user.RequirePasswordChange,
		OnboardingCompleted:   user.OnboardingCompleted,
		CreatedAt:             user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             user.UpdatedAt.Format(time.RFC3339),
	}
	if user.LastLoginAt != nil {
		response.LastLoginAt = user.LastLoginAt.Format(time.RFC3339)
	}

	return response
}

func serializeWorkspace(workspace Workspace) publicWorkspace {
	response := publicWorkspace{
		ID:        workspace.ID.Hex(),
		Name:      workspace.Name,
		Slug:      workspace.Slug,
		Kind:      workspace.Kind,
		CreatedAt: workspace.CreatedAt.Format(time.RFC3339),
		UpdatedAt: workspace.UpdatedAt.Format(time.RFC3339),
	}
	if !workspace.CreatedBy.IsZero() {
		response.CreatedBy = workspace.CreatedBy.Hex()
	}

	return response
}

func serializeAPIKey(apiKey APIKey) publicAPIKey {
	response := publicAPIKey{
		ID:          apiKey.ID.Hex(),
		WorkspaceID: apiKey.WorkspaceID.Hex(),
		Name:        apiKey.Name,
		Prefix:      apiKey.Prefix,
		Scopes:      apiKey.Scopes,
		CreatedBy:   apiKey.CreatedBy.Hex(),
		CreatedAt:   apiKey.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   apiKey.UpdatedAt.Format(time.RFC3339),
	}
	if apiKey.LastUsedAt != nil {
		response.LastUsedAt = apiKey.LastUsedAt.Format(time.RFC3339)
	}
	if apiKey.RevokedAt != nil {
		response.RevokedAt = apiKey.RevokedAt.Format(time.RFC3339)
	}
	if apiKey.ExpiresAt != nil {
		response.ExpiresAt = apiKey.ExpiresAt.Format(time.RFC3339)
	}

	return response
}
