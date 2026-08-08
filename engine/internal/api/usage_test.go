package api

import (
	"testing"
	"time"
)

func TestNewUsageWindowStartsEveryScanTypeAtZero(t *testing.T) {
	since := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	window := newUsageWindow(since)

	if window.Total != 0 {
		t.Fatalf("expected an empty window, got total %d", window.Total)
	}
	if window.Since != "2026-01-02T03:04:05Z" {
		t.Fatalf("unexpected since: %s", window.Since)
	}
	for _, scanType := range usageScanTypes {
		if _, ok := window.ByType[scanType]; !ok {
			t.Fatalf("scan type %q missing from the breakdown", scanType)
		}
	}
	if len(window.ByType) != len(usageScanTypes) {
		t.Fatalf("expected %d scan types, got %d", len(usageScanTypes), len(window.ByType))
	}
}

func TestUsageWindowsAreRolling(t *testing.T) {
	if usageWeeklyWindow != 7*24*time.Hour {
		t.Fatalf("weekly window should cover 7 days, got %s", usageWeeklyWindow)
	}
	if usageMonthlyWindow <= usageWeeklyWindow {
		t.Fatal("the monthly window must be wider than the weekly one")
	}
}

func TestUsageLimitsForPlan(t *testing.T) {
	tests := []struct {
		plan string
		want scanUsageLimits
	}{
		{plan: planFree, want: scanUsageLimits{Weekly: 250, Monthly: 1_000}},
		{plan: planPro, want: scanUsageLimits{Weekly: 2_500, Monthly: 10_000}},
		{plan: planEnterprise, want: scanUsageLimits{Weekly: unlimitedLimit, Monthly: unlimitedLimit}},
	}

	for _, test := range tests {
		if got := usageLimitsForPlan(test.plan); got != test.want {
			t.Fatalf("usageLimitsForPlan(%q) = %+v, want %+v", test.plan, got, test.want)
		}
	}
}

func TestScanUsageLimitErrorAtBoundary(t *testing.T) {
	limits := usageLimitsForPlan(planFree)
	if err := scanUsageLimitError(limits.Weekly-1, limits.Monthly-1, limits); err != nil {
		t.Fatalf("usage below both limits should be accepted: %v", err)
	}

	weeklyErr := scanUsageLimitError(limits.Weekly, limits.Monthly-1, limits)
	if weeklyErr == nil || weeklyErr.Error() != "weekly scan limit reached (250); upgrade your plan or wait for older scans to leave the usage window" {
		t.Fatalf("unexpected weekly limit error: %v", weeklyErr)
	}

	monthlyErr := scanUsageLimitError(0, limits.Monthly, limits)
	if monthlyErr == nil || monthlyErr.Error() != "monthly scan limit reached (1,000); upgrade your plan or wait for older scans to leave the usage window" {
		t.Fatalf("unexpected monthly limit error: %v", monthlyErr)
	}
}

func TestScanUsageLimitErrorNeverBlocksUnlimited(t *testing.T) {
	enterprise := usageLimitsForPlan(planEnterprise)
	if err := scanUsageLimitError(1_000_000, 1_000_000, enterprise); err != nil {
		t.Fatalf("enterprise (unlimited) scan usage must never be blocked, got: %v", err)
	}
}
