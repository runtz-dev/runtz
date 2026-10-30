package cloudgcp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPaginationNormalizationAndDeduplication(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "securitycenter.googleapis.com" || r.URL.Path != "/v2/projects/test-project/sources/-/locations/eu/findings" {
			t.Fatalf("unexpected destination: %s", r.URL)
		}
		if r.Method != http.MethodGet {
			t.Fatal("import must only read cloud data")
		}
		body := `{"listFindingsResults":[{"finding":{"name":"finding-1","category":"PUBLIC_BUCKET","severity":"HIGH","state":"ACTIVE","resourceName":"//storage/bucket","externalUri":"https://console.cloud.google.com/","nextSteps":"Restrict access"},"resource":{"type":"storage.googleapis.com/Bucket","location":"europe-west1"}}],"nextPageToken":"second"}`
		if calls == 2 {
			if r.URL.Query().Get("pageToken") != "second" {
				t.Fatal("missing continuation token")
			}
			body = `{"listFindingsResults":[{"finding":{"name":"finding-1","severity":"HIGH"}},{"finding":{"name":"finding-2","category":"NEW_TYPE","severity":"SEVERITY_UNSPECIFIED","state":"INACTIVE"}}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	findings, err := list(context.Background(), client, endpoint, "test-project", "eu", false)
	if err != nil || calls != 2 || len(findings) != 2 {
		t.Fatalf("calls=%d, count=%d, err=%v", calls, len(findings), err)
	}
	if findings[0].Severity != "high" || findings[0].Title != "PUBLIC BUCKET" || findings[0].ResourceType != "storage.googleapis.com/Bucket" || findings[0].Remediation != "Restrict access" || findings[1].Severity != "unknown" {
		t.Fatalf("normalization failed: %+v", findings)
	}
}

func TestProbeReadsOnePageAndProviderErrorsAreRedacted(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("pageSize") != "1" {
			t.Fatal("probe must request a single result")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"nextPageToken":"more"}`))}, nil
	})}
	if _, err := list(context.Background(), client, endpoint, "test-project", "global", true); err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{401, 403, 404, 429, 500} {
		err := responseError(status, []byte("customer-private-data"))
		if strings.Contains(err.Error(), "customer-private-data") {
			t.Fatal("provider error body leaked")
		}
	}
	if !strings.Contains(responseError(403, []byte(`{"reason":"SERVICE_DISABLED"}`)).Error(), "enable") {
		t.Fatal("missing disabled API guidance")
	}
}

func TestRepeatedPaginationFailsWithoutPartialResults(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"nextPageToken":"same"}`))}, nil
	})}
	findings, err := list(context.Background(), client, endpoint, "test-project", "global", false)
	if err == nil || findings != nil {
		t.Fatal("partial results must not be accepted")
	}
}

func TestCredentialsRejectUntrustedEndpointsAndTypes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	c := Credentials{Type: "service_account", ProjectID: "test-project", ClientEmail: "test@test-project.iam.gserviceaccount.com", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), PrivateKeyID: "test-id", TokenURI: tokenURL}
	raw, _ := json.Marshal(c)
	if _, err := ParseCredentials(raw); err != nil {
		t.Fatal(err)
	}
	c.TokenURI = "https://attacker.example/token"
	raw, _ = json.Marshal(c)
	if _, err := ParseCredentials(raw); err == nil {
		t.Fatal("accepted an untrusted credential endpoint")
	}
	if _, err := ParseCredentials([]byte(`{"type":"external_account","credential_source":{"url":"http://localhost"}}`)); err == nil {
		t.Fatal("accepted external credentials")
	}
	for _, project := range []string{"../../secret", "123456", "test/project", "test-project?query=1"} {
		if ValidProject(project) {
			t.Fatalf("accepted project %s", project)
		}
	}
}

// Opt-in, read-only smoke test. The key is only sent to Google's fixed OAuth
// endpoint and is never printed or copied into repository fixtures.
func TestLiveGCPReadOnly(t *testing.T) {
	path := os.Getenv("RUNTZ_TEST_GCP_KEY_FILE")
	if path == "" {
		t.Skip("set RUNTZ_TEST_GCP_KEY_FILE for an authorized GCP smoke test")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("could not read service account file")
	}
	c, err := ParseCredentials(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	findings, err := Read(ctx, raw, c.ProjectID, "global", false)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	t.Logf("Native SCC import: %d findings, severity counts: %v", len(findings), counts)
}
