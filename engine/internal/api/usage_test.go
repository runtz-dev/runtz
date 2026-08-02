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
