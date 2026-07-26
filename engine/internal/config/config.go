package config

import (
	"fmt"
	"os"
	"strings"
)

// LicenseVerificationKey is the base64-encoded Ed25519 public key that verifies
// self-hosted license certificates issued by the central engine at
// https://engine.runtz.dev. It is compiled into the binary on purpose: a
// self-hosted installation must not be able to disable or replace license
// verification by setting an environment variable. The matching private key is
// held only by RAW DEVOPS LTDA and never ships in this repository.
//
// Overridable at build time for tests only:
//
//	-ldflags "-X github.com/runtz-dev/runtz/engine/internal/config.LicenseVerificationKey=<base64>"
//
// Removing or bypassing this verification is a breach of the license (BUSL-1.1);
// see NOTICE.
var LicenseVerificationKey = "6+L0cjUXo2j4XvhjcJCcOZNfeU5s8haY5fhaZQ/WnkM="

type Config struct {
	Port                            string
	MongoURI                        string
	MongoDatabase                   string
	JWTSecret                       string
	IngestToken                     string
	DeploymentMode                  string
	PublicURL                       string
	GoogleClientID                  string
	GitHubClientID                  string
	GitHubClientSecret              string
	ResendAPIKey                    string
	ResendFromEmail                 string
	StripeSecretKey                 string
	StripeWebhookSecret             string
	StripePriceProCloud             string
	StripePriceEnterpriseCloud      string
	StripePriceProSelfHosted        string
	StripePriceEnterpriseSelfHosted string
	LicensePrivateKey               string
	LicensePublicKey                string
	CORSAllowedOrigins              []string
}

func Load() Config {
	return Config{
		Port:                            getEnv("PORT", "8080"),
		MongoURI:                        getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDatabase:                   getEnv("MONGODB_DATABASE", "runtz"),
		JWTSecret:                       getEnv("JWT_SECRET", ""),
		IngestToken:                     getEnv("RUNTZ_INGEST_TOKEN", ""),
		DeploymentMode:                  normalizeDeploymentMode(getEnv("RUNTZ_DEPLOYMENT_MODE", "self-hosted")),
		PublicURL:                       strings.TrimRight(getEnv("RUNTZ_PUBLIC_URL", "http://localhost:3000"), "/"),
		GoogleClientID:                  getEnv("GOOGLE_CLIENT_ID", ""),
		GitHubClientID:                  getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:              getEnv("GITHUB_CLIENT_SECRET", ""),
		ResendAPIKey:                    getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:                 getEnv("RESEND_FROM_EMAIL", "Runtz <login@runtz.dev>"),
		StripeSecretKey:                 getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:             getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceProCloud:             getEnv("STRIPE_PRICE_PRO_CLOUD", ""),
		StripePriceEnterpriseCloud:      getEnv("STRIPE_PRICE_ENTERPRISE_CLOUD", ""),
		StripePriceProSelfHosted:        getEnv("STRIPE_PRICE_PRO_SELF_HOSTED", ""),
		StripePriceEnterpriseSelfHosted: getEnv("STRIPE_PRICE_ENTERPRISE_SELF_HOSTED", ""),
		LicensePrivateKey:               getEnv("RUNTZ_LICENSE_PRIVATE_KEY", ""),
		// The verification key is compiled in (see LicenseVerificationKey) and is
		// intentionally not configurable via the environment.
		LicensePublicKey:   LicenseVerificationKey,
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
	}
}

// weakSecrets are placeholder values that previously shipped as defaults or in
// example files. They must never authenticate a real deployment.
var weakSecrets = map[string]bool{
	"change-me-in-production":     true,
	"change-me-before-sharing":    true,
	"change-me-before-production": true,
	"dev-ingest-token":            true,
}

const (
	minJWTSecretLength   = 32
	minIngestTokenLength = 16
)

// Validate rejects configurations that would run the engine with missing or
// well-known placeholder credentials. It is meant to be called once at startup.
func (c Config) Validate() error {
	if c.JWTSecret == "" || weakSecrets[c.JWTSecret] {
		return fmt.Errorf("JWT_SECRET must be set to a strong random value; generate one with: openssl rand -base64 32")
	}
	if len(c.JWTSecret) < minJWTSecretLength {
		return fmt.Errorf("JWT_SECRET must be at least %d characters; generate one with: openssl rand -base64 32", minJWTSecretLength)
	}
	if c.IngestToken == "" || weakSecrets[c.IngestToken] {
		return fmt.Errorf("RUNTZ_INGEST_TOKEN must be set to a strong random value; generate one with: openssl rand -base64 24")
	}
	if len(c.IngestToken) < minIngestTokenLength {
		return fmt.Errorf("RUNTZ_INGEST_TOKEN must be at least %d characters; generate one with: openssl rand -base64 24", minIngestTokenLength)
	}

	return nil
}

func normalizeDeploymentMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "cloud") {
		return "cloud"
	}

	return "self-hosted"
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	return items
}
