package api

import (
	"strings"
	"testing"
	"time"
)

func TestPlaygroundScansShape(t *testing.T) {
	now := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)
	scans := playgroundScans(
		Workspace{Name: playgroundWorkspaceName},
		now,
	)

	const scansPerDay = 8
	if got, want := len(scans), playgroundHistoryDays*scansPerDay; got != want {
		t.Fatalf("len(playgroundScans()) = %d, want %d", got, want)
	}

	scaProjects := make(map[string]bool)
	sastProjects := make(map[string]bool)
	containerApps := make(map[string]bool)
	hosts := make(map[string]bool)
	k8sClusters := make(map[string]bool)
	vulnerabilityCounts := make(map[int]bool)
	findingCounts := make(map[int]bool)
	var latest time.Time

	for _, scan := range scans {
		if scan.ScannerVersion != playgroundScannerVersion {
			t.Fatalf("ScannerVersion = %q, want %q", scan.ScannerVersion, playgroundScannerVersion)
		}
		if scan.CreatedAt.After(now) {
			t.Fatalf("scan %s created_at = %s, want not after %s", scan.ID.Hex(), scan.CreatedAt, now)
		}
		if scan.CreatedAt.After(latest) {
			latest = scan.CreatedAt
		}

		switch scan.Type {
		case "sca":
			scaProjects[scan.ProjectName] = true
			if got := len(scan.Vulnerabilities); got > pgMaxVulns {
				t.Fatalf("scan %s has %d vulnerabilities, want at most %d", scan.ID.Hex(), got, pgMaxVulns)
			} else {
				vulnerabilityCounts[got] = true
			}
		case "sast":
			sastProjects[scan.TargetName] = true
			if scan.FilesScanned <= 0 {
				t.Fatalf("SAST scan %s FilesScanned = %d, want > 0", scan.ID.Hex(), scan.FilesScanned)
			}
			assertFindingSummary(t, scan, scan.FilesScanned)
			if got := len(scan.Findings); got > pgMaxFindings {
				t.Fatalf("SAST scan %s has %d findings, want at most %d", scan.ID.Hex(), got, pgMaxFindings)
			} else {
				findingCounts[got] = true
			}
		case "container":
			app := strings.TrimSuffix(strings.TrimPrefix(scan.ImageName, "ghcr.io/company/"), ":latest")
			containerApps[app] = true
			if got := len(scan.Vulnerabilities); got > pgMaxVulns {
				t.Fatalf("scan %s has %d vulnerabilities, want at most %d", scan.ID.Hex(), got, pgMaxVulns)
			} else {
				vulnerabilityCounts[got] = true
			}
		case "host":
			hosts[scan.Hostname] = true
			if got := len(scan.Vulnerabilities); got > pgMaxVulns {
				t.Fatalf("scan %s has %d vulnerabilities, want at most %d", scan.ID.Hex(), got, pgMaxVulns)
			} else {
				vulnerabilityCounts[got] = true
			}
		case "k8s":
			k8sClusters[scan.TargetName] = true
			if scan.ResourcesScanned <= 0 {
				t.Fatalf("K8s scan %s ResourcesScanned = %d, want > 0", scan.ID.Hex(), scan.ResourcesScanned)
			}
			assertFindingSummary(t, scan, scan.ResourcesScanned)
			if got := len(scan.Findings); got > pgMaxFindings {
				t.Fatalf("K8s scan %s has %d findings, want at most %d", scan.ID.Hex(), got, pgMaxFindings)
			} else {
				findingCounts[got] = true
			}
		default:
			t.Fatalf("unexpected scan type %q", scan.Type)
		}
	}

	if playgroundDayKey(latest) != playgroundDayKey(now) {
		t.Fatalf("latest scan day = %s, want %s", playgroundDayKey(latest), playgroundDayKey(now))
	}
	if got, want := len(scaProjects), 14; got != want {
		t.Fatalf("SCA projects = %d, want %d", got, want)
	}
	if got, want := len(sastProjects), 14; got != want {
		t.Fatalf("SAST projects = %d, want %d", got, want)
	}
	if got, want := len(containerApps), 14; got != want {
		t.Fatalf("container apps = %d, want %d", got, want)
	}
	if got, want := len(hosts), 5; got != want {
		t.Fatalf("hosts = %d, want %d", got, want)
	}
	if got, want := len(k8sClusters), 5; got != want {
		t.Fatalf("K8s clusters = %d, want %d", got, want)
	}
	for app := range containerApps {
		if !scaProjects[app] {
			t.Fatalf("container app %q does not have a matching SCA project", app)
		}
	}
	for app := range sastProjects {
		if !scaProjects[app] {
			t.Fatalf("SAST app %q does not have a matching SCA project", app)
		}
	}
	if len(vulnerabilityCounts) < 3 {
		t.Fatalf("vulnerability count variants = %d, want at least 3", len(vulnerabilityCounts))
	}
	if len(findingCounts) < 3 {
		t.Fatalf("finding count variants = %d, want at least 3", len(findingCounts))
	}
}

func assertFindingSummary(t *testing.T, scan Scan, totalScanned int) {
	t.Helper()

	if scan.Summary.TotalDependencies != totalScanned {
		t.Fatalf("scan %s TotalDependencies = %d, want %d", scan.ID.Hex(), scan.Summary.TotalDependencies, totalScanned)
	}
	if scan.Summary.Vulnerabilities != len(scan.Findings) {
		t.Fatalf("scan %s summary vulnerabilities = %d, want findings len %d", scan.ID.Hex(), scan.Summary.Vulnerabilities, len(scan.Findings))
	}
	if len(scan.Findings) == 0 {
		t.Fatalf("scan %s has no findings", scan.ID.Hex())
	}
}
