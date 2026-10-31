// Package cloudgcp defines cloud finding DTOs and validates uploaded key format.
// GCP reads and security checks run in the separate Runtz Security service.
package cloudgcp

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"regexp"
	"strings"
)

const tokenURL = "https://oauth2.googleapis.com/token"

type Credentials struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	PrivateKeyID string `json:"private_key_id"`
	TokenURI     string `json:"token_uri"`
}

var projectPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,61}[a-z0-9]$`)

func ValidProject(project string) bool { return projectPattern.MatchString(project) }

func ParseCredentials(raw []byte) (Credentials, error) {
	var c Credentials
	if len(raw) > 32768 || json.Unmarshal(raw, &c) != nil || c.Type != "service_account" ||
		!ValidProject(c.ProjectID) || !strings.HasSuffix(c.ClientEmail, ".iam.gserviceaccount.com") ||
		!strings.Contains(c.ClientEmail, "@") || c.PrivateKeyID == "" || c.TokenURI != tokenURL {
		return c, errors.New("upload a valid Google Cloud service account JSON key")
	}
	block, _ := pem.Decode([]byte(c.PrivateKey))
	if block == nil {
		return c, errors.New("service account private key is invalid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return c, errors.New("service account private key is invalid")
	}
	if key, ok := parsed.(*rsa.PrivateKey); !ok || key.N.BitLen() < 2048 {
		return c, errors.New("service account key must be an RSA private key")
	}
	return c, nil
}

type Finding struct {
	RuleID       string `bson:"rule_id,omitempty" json:"ruleId,omitempty"`
	Evidence     string `bson:"evidence,omitempty" json:"evidence,omitempty"`
	ID           string `bson:"id" json:"id"`
	Title        string `bson:"title" json:"title"`
	Category     string `bson:"category" json:"category"`
	Description  string `bson:"description" json:"description"`
	Severity     string `bson:"severity" json:"severity"`
	State        string `bson:"state" json:"state"`
	Mute         string `bson:"mute" json:"mute"`
	ResourceName string `bson:"resource_name" json:"resourceName"`
	ResourceType string `bson:"resource_type" json:"resourceType"`
	ProjectName  string `bson:"project_name" json:"projectName"`
	Location     string `bson:"location" json:"location"`
	Source       string `bson:"source" json:"source"`
	ExternalURI  string `bson:"external_uri" json:"externalUri"`
	EventTime    string `bson:"event_time" json:"eventTime"`
	Remediation  string `bson:"remediation" json:"remediation"`
}
