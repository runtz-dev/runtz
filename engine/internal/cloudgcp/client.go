// Package cloudgcp imports native Security Command Center findings. It does not
// run cloud configuration checks or enable/modify services in customer projects.
package cloudgcp

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

const tokenURL = "https://oauth2.googleapis.com/token"
const endpoint = "https://securitycenter.googleapis.com/v2/"
const maxFindings = 10000

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
func ValidLocation(location string) bool {
	return location == "global" || location == "eu" || location == "us" || location == "sa"
}

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
	if _, ok := parsed.(*rsa.PrivateKey); !ok {
		return c, errors.New("service account key must be an RSA private key")
	}
	return c, nil
}

type Finding struct {
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

type nativeResult struct {
	Finding struct {
		Name              string `json:"name"`
		Category          string `json:"category"`
		Description       string `json:"description"`
		Severity          string `json:"severity"`
		State             string `json:"state"`
		Mute              string `json:"mute"`
		ResourceName      string `json:"resourceName"`
		ExternalURI       string `json:"externalUri"`
		EventTime         string `json:"eventTime"`
		ParentDisplayName string `json:"parentDisplayName"`
		NextSteps         string `json:"nextSteps"`
	} `json:"finding"`
	Resource struct {
		Name               string `json:"name"`
		DisplayName        string `json:"displayName"`
		Type               string `json:"type"`
		ProjectDisplayName string `json:"projectDisplayName"`
		Location           string `json:"location"`
	} `json:"resource"`
}

func normalize(n nativeResult) Finding {
	f := n.Finding
	severity := strings.ToLower(f.Severity)
	switch severity {
	case "critical", "high", "medium", "low":
	default:
		severity = "unknown"
	}
	resource := f.ResourceName
	if resource == "" {
		resource = n.Resource.Name
	}
	return Finding{ID: f.Name, Title: strings.ReplaceAll(f.Category, "_", " "), Category: f.Category,
		Description: f.Description, Severity: severity, State: f.State, Mute: f.Mute,
		ResourceName: resource, ResourceType: n.Resource.Type, ProjectName: n.Resource.ProjectDisplayName,
		Location: n.Resource.Location, Source: f.ParentDisplayName, ExternalURI: f.ExternalURI,
		EventTime: f.EventTime, Remediation: f.NextSteps}
}

// Read uses a fixed OAuth endpoint and fixed SCC host. Uploaded JSON cannot
// choose a token server, credential provider, or arbitrary HTTP destination.
func Read(ctx context.Context, raw []byte, project, location string, probe bool) ([]Finding, error) {
	c, err := ParseCredentials(raw)
	if err != nil {
		return nil, err
	}
	if !ValidProject(project) || !ValidLocation(location) {
		return nil, errors.New("invalid project or SCC location")
	}
	canonical, _ := json.Marshal(c)
	conf, err := google.JWTConfigFromJSON(canonical, "https://www.googleapis.com/auth/securitycenter")
	if err != nil {
		return nil, errors.New("invalid service account key")
	}
	conf.TokenURL = tokenURL
	client := conf.Client(ctx)
	client.Timeout = 30 * time.Second
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return list(ctx, client, endpoint, project, location, probe)
}

func list(ctx context.Context, client *http.Client, base, project, location string, probe bool) ([]Finding, error) {
	findings := make([]Finding, 0)
	seenIDs := map[string]bool{}
	seenPages := map[string]bool{}
	pageToken := ""
	totalBytes := 0
	for page := 0; page < 100; page++ {
		q := url.Values{"pageSize": {"500"}}
		if probe {
			q.Set("pageSize", "1")
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"projects/"+project+"/sources/-/locations/"+location+"/findings?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, errors.New("could not reach GCP; check the service account key and network, then retry")
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
		response.Body.Close()
		totalBytes += len(body)
		if response.StatusCode != http.StatusOK {
			return nil, responseError(response.StatusCode, body)
		}
		if readErr != nil || len(body) > 8*1024*1024 || totalBytes > 64*1024*1024 {
			return nil, errors.New("GCP response exceeded the import limit; previous findings were preserved")
		}
		var payload struct {
			Results       []nativeResult `json:"listFindingsResults"`
			NextPageToken string         `json:"nextPageToken"`
		}
		if json.Unmarshal(body, &payload) != nil {
			return nil, errors.New("GCP returned an invalid findings response")
		}
		for _, result := range payload.Results {
			if result.Finding.Name == "" {
				return nil, errors.New("GCP returned a finding without an identifier")
			}
			if seenIDs[result.Finding.Name] {
				continue
			}
			seenIDs[result.Finding.Name] = true
			findings = append(findings, normalize(result))
			if len(findings) > maxFindings {
				return nil, errors.New("project exceeds the 10,000 finding import limit; previous findings were preserved")
			}
		}
		if probe || payload.NextPageToken == "" {
			return findings, nil
		}
		pageToken = payload.NextPageToken
		if seenPages[pageToken] {
			return nil, errors.New("GCP returned repeated pagination; previous findings were preserved")
		}
		seenPages[pageToken] = true
	}
	return nil, errors.New("GCP import exceeded the pagination limit; previous findings were preserved")
}

func responseError(status int, body []byte) error {
	// Never return provider bodies: they can contain customer data or credentials.
	if strings.Contains(string(body), "SERVICE_DISABLED") {
		return errors.New("enable the Security Command Center API in the credential project and activate SCC for the target project")
	}
	switch status {
	case 401:
		return errors.New("GCP rejected this service account key; create a new JSON key and retry")
	case 403:
		return errors.New("GCP denied access; grant Security Reviewer (roles/iam.securityReviewer) on the target project and verify SCC activation")
	case 404:
		return errors.New("project or SCC location not found; verify the project ID, SCC activation and data residency location")
	case 429:
		return errors.New("GCP rate limit reached; retry the import shortly")
	default:
		return fmt.Errorf("GCP findings request failed (HTTP %d); retry the import", status)
	}
}
