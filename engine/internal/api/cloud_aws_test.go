package api

import (
	"github.com/runtz-dev/runtz/engine/internal/securityclient"
	"testing"
)

func TestAWSResultScopeIncludesFindingAndRegion(t *testing.T) {
	c := cloudConnection{Provider: "aws", ProjectID: "123456789012", AccountID: "123456789012", Regions: []string{"us-east-1"}}
	result := &securityclient.Result{Provider: "aws", AccountID: c.AccountID, Regions: c.Regions, Findings: []securityclient.Finding{{AccountID: c.AccountID, Region: "us-east-1"}}}
	if !matchesCloudResult(c, result) {
		t.Fatal("valid AWS result rejected")
	}
	result.Findings[0].AccountID = "999999999999"
	if matchesCloudResult(c, result) {
		t.Fatal("finding from another account accepted")
	}
	result.Findings[0].AccountID = c.AccountID
	result.Findings[0].Region = "sa-east-1"
	if matchesCloudResult(c, result) {
		t.Fatal("finding outside selected regions accepted")
	}
}
