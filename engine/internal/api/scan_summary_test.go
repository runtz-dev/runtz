package api

import "testing"

func TestBuildSummarySplitsVulnerabilitiesByFixAvailability(t *testing.T) {
	t.Parallel()

	vulnerabilities := []Vulnerability{
		{ID: "CVE-1", Severity: "critical", FirstPatchedVersion: "2.0.0"},
		{ID: "CVE-2", Severity: "HIGH", FirstPatchedVersion: "  "},
		{ID: "CVE-3", Severity: "medium"},
		{ID: "CVE-4", Severity: "unexpected", FirstPatchedVersion: "1.4.1"},
	}

	summary := buildSummary([]Dependency{{Name: "example"}}, vulnerabilities)

	if !summary.FixStatusComputed {
		t.Fatal("FixStatusComputed = false, want true")
	}
	if summary.TotalDependencies != 1 || summary.Vulnerabilities != 4 {
		t.Fatalf(
			"summary totals = (%d dependencies, %d vulnerabilities), want (1, 4)",
			summary.TotalDependencies,
			summary.Vulnerabilities,
		)
	}
	if summary.Critical != 1 || summary.High != 1 || summary.Medium != 1 || summary.Unknown != 1 {
		t.Fatalf("severity summary = %+v, want one critical, high, medium, and unknown", summary)
	}
	if summary.WithFix.Vulnerabilities != 2 || summary.WithFix.Critical != 1 || summary.WithFix.Unknown != 1 {
		t.Fatalf("with-fix summary = %+v, want two vulnerabilities", summary.WithFix)
	}
	if summary.WithoutFix.Vulnerabilities != 2 || summary.WithoutFix.High != 1 || summary.WithoutFix.Medium != 1 {
		t.Fatalf("without-fix summary = %+v, want two vulnerabilities", summary.WithoutFix)
	}
}

func TestBuildFindingSummaryDoesNotClassifyFindingsAsCVEs(t *testing.T) {
	t.Parallel()

	summary := buildFindingSummary(3, []Finding{{ID: "SAST-1", Severity: "high"}})

	if summary.WithFix.Vulnerabilities != 0 || summary.WithoutFix.Vulnerabilities != 0 {
		t.Fatalf("finding fix summaries = (%+v, %+v), want both empty", summary.WithFix, summary.WithoutFix)
	}
}
