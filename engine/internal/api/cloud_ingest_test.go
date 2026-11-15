package api

import (
	"testing"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/securityclient"
)

func cliCloudFixture(provider string) securityclient.Result {
	now := time.Now().UTC()
	result := securityclient.Result{Provider: provider, CatalogVersion: "catalog", StartedAt: now.Add(-time.Minute), FinishedAt: now, Checks: []securityclient.Check{}, Findings: []securityclient.Finding{}}
	prefix := map[string]string{"gcp": "RZ-GCP-001", "aws": "RZ-AWS-001", "azure": "RZ-AZURE-001"}[provider]
	region := ""
	switch provider {
	case "gcp":
		result.ProjectID = "test-project"
	case "aws":
		result.AccountID = "123456789012"
		result.Regions = []string{"us-east-1"}
		region = "us-east-1"
	case "azure":
		result.SubscriptionID = "11111111-1111-4111-8111-111111111111"
	}
	result.Checks = append(result.Checks, securityclient.Check{RuleID: prefix, Title: "Read-only check", Status: "fail", Severity: "high", Resource: "resource", Region: region}, securityclient.Check{RuleID: prefix, Title: "Missing permission", Status: "skipped", Severity: "high", Resource: "other", Region: region})
	result.Findings = append(result.Findings, securityclient.Finding{ID: "finding", RuleID: prefix, Title: "Finding", Severity: "high", ResourceName: "resource", Region: region, ProjectID: result.ProjectID, AccountID: result.AccountID, SubscriptionID: result.SubscriptionID})
	return result
}

func TestCLICloudCountsAreDerivedAndPartialCoveragePreserved(t *testing.T) {
	for _, provider := range []string{"gcp", "aws", "azure"} {
		t.Run(provider, func(t *testing.T) {
			result := cliCloudFixture(provider)
			result.Summary = securityclient.Summary{Passed: 999, Critical: 999}
			if err := validateCLICloudResult(&result); err != nil {
				t.Fatal(err)
			}
			if result.Summary.High != 1 || result.Summary.Failed != 1 || result.Summary.Skipped != 1 || result.Summary.Passed != 0 || result.Summary.Critical != 0 {
				t.Fatalf("incorrect derived coverage: %+v", result.Summary)
			}
		})
	}
}

func TestCLICloudRejectsInconsistentResults(t *testing.T) {
	cases := map[string]func(*securityclient.Result){
		"unknown provider":        func(r *securityclient.Result) { r.Provider = "other" },
		"cross cloud scope":       func(r *securityclient.Result) { r.AccountID = "123456789012" },
		"cross project finding":   func(r *securityclient.Result) { r.Findings[0].ProjectID = "other-project" },
		"finding without failure": func(r *securityclient.Result) { r.Checks[0].Status = "pass" },
		"omitted finding":         func(r *securityclient.Result) { r.Findings = nil },
		"duplicate finding":       func(r *securityclient.Result) { r.Findings = append(r.Findings, r.Findings[0]) },
		"future result":           func(r *securityclient.Result) { r.FinishedAt = time.Now().Add(time.Hour) },
		"unfinished result":       func(r *securityclient.Result) { r.FinishedAt = time.Time{} },
		"unknown check status":    func(r *securityclient.Result) { r.Checks[0].Status = "secure" },
		"foreign rule":            func(r *securityclient.Result) { r.Checks[0].RuleID = "RZ-AWS-001" },
		"invalid severity":        func(r *securityclient.Result) { r.Checks[0].Severity = "informational" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			result := cliCloudFixture("gcp")
			mutate(&result)
			if validateCLICloudResult(&result) == nil {
				t.Fatal("accepted inconsistent result")
			}
		})
	}
	result := cliCloudFixture("aws")
	result.Checks[0].Region = "sa-east-1"
	if validateCLICloudResult(&result) == nil {
		t.Fatal("accepted check outside selected regions")
	}
}
