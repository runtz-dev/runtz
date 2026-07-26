package api

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	playgroundScanSource     = "playground"
	playgroundWorkspaceName  = "Runtz Playground"
	playgroundWorkspaceSlug  = "runtz-playground"
	playgroundScannerVersion = "playground-3.0.0"
	playgroundHistoryDays    = 30
)

func (s *Server) handleListAllPlaygroundScans(w http.ResponseWriter, r *http.Request) {
	workspace, err := s.ensurePlaygroundData(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare playground")
		return
	}

	scans, err := s.listPlaygroundScans(r.Context(), workspace, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list scans")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"scans": scans})
}

func (s *Server) handleListPlaygroundScans(w http.ResponseWriter, r *http.Request) {
	scanType := r.PathValue("type")
	if !isPlaygroundScanType(scanType) {
		writeError(w, http.StatusNotFound, "scan type not found")
		return
	}

	workspace, err := s.ensurePlaygroundData(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare playground")
		return
	}

	scans, err := s.listPlaygroundScans(r.Context(), workspace, scanType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list scans")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"scans": scans})
}

func (s *Server) listPlaygroundScans(ctx context.Context, workspace Workspace, scanType string) ([]Scan, error) {
	filter := bson.M{
		"workspace_id": workspace.ID,
		"source":       playgroundScanSource,
	}
	if scanType != "" {
		filter["type"] = scanType
	}

	cursor, err := s.scans.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(1000),
	)
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cursor)

	scans := make([]Scan, 0)
	if err := cursor.All(ctx, &scans); err != nil {
		return nil, err
	}

	return scans, nil
}

func (s *Server) ensurePlaygroundData(ctx context.Context) (Workspace, error) {
	now := time.Now().UTC()
	today := playgroundDayKey(now)

	s.playgroundMu.Lock()
	defer s.playgroundMu.Unlock()

	if s.playgroundReadyDay == today && !s.playgroundWorkspace.ID.IsZero() {
		return s.playgroundWorkspace, nil
	}

	workspace, err := s.ensurePlaygroundWorkspace(ctx)
	if err != nil {
		return Workspace{}, err
	}

	seedScans := playgroundScans(workspace, now)
	filter := bson.M{
		"workspace_id": workspace.ID,
		"source":       playgroundScanSource,
	}
	count, err := s.scans.CountDocuments(ctx, filter)
	if err != nil {
		return Workspace{}, err
	}
	currentSeedCount, err := s.scans.CountDocuments(ctx, bson.M{
		"workspace_id":    workspace.ID,
		"source":          playgroundScanSource,
		"scanner_version": playgroundScannerVersion,
	})
	if err != nil {
		return Workspace{}, err
	}

	latestCurrent, err := s.playgroundLatestScanIsCurrent(ctx, filter, now)
	if err != nil {
		return Workspace{}, err
	}
	if count == int64(len(seedScans)) && currentSeedCount == count && latestCurrent {
		s.playgroundReadyDay = today
		s.playgroundWorkspace = workspace
		return workspace, nil
	}

	// Wipe stale data and reseed from scratch.
	if _, err := s.scans.DeleteMany(ctx, filter); err != nil {
		return Workspace{}, err
	}

	docs := make([]any, len(seedScans))
	for i, scan := range seedScans {
		docs[i] = scan
	}
	if _, err := s.scans.InsertMany(ctx, docs); err != nil {
		return Workspace{}, err
	}

	s.playgroundReadyDay = today
	s.playgroundWorkspace = workspace
	return workspace, nil
}

func (s *Server) playgroundLatestScanIsCurrent(ctx context.Context, filter bson.M, now time.Time) (bool, error) {
	var latest Scan
	err := s.scans.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})).Decode(&latest)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if latest.CreatedAt.After(now) {
		return false, nil
	}
	return playgroundDayKey(latest.CreatedAt.UTC()) == playgroundDayKey(now), nil
}

func playgroundDayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func isPlaygroundScanType(scanType string) bool {
	return scanType == "sca" || scanType == "sast" || scanType == "host" || scanType == "container" || scanType == "k8s"
}

func (s *Server) ensurePlaygroundWorkspace(ctx context.Context) (Workspace, error) {
	var workspace Workspace
	err := s.workspaces.FindOne(ctx, bson.M{"slug": playgroundWorkspaceSlug}).Decode(&workspace)
	if err == nil {
		return workspace, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return Workspace{}, err
	}

	now := time.Now().UTC()
	workspace = Workspace{
		ID:        bson.NewObjectID(),
		Name:      playgroundWorkspaceName,
		Slug:      playgroundWorkspaceSlug,
		Kind:      "playground",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.workspaces.InsertOne(ctx, workspace); err != nil {
		if findErr := s.workspaces.FindOne(ctx, bson.M{"slug": playgroundWorkspaceSlug}).Decode(&workspace); findErr == nil {
			return workspace, nil
		}
		return Workspace{}, err
	}
	return workspace, nil
}

// ── Algorithmic playground generation ─────────────────────────────────────────
//
// 14 SCA apps · 14 SAST apps · 14 container images (same app names) · 5 hosts · 5 K8s clusters
// 2 SCA/day + 2 SAST/day + 2 container/day + 1 host/day + 1 K8s/day = 8 scans/day × 30 days = 240 scans
//
// Trend: team implemented Runtz and saw ALL existing vulns at once (high plateau),
// started fixing them, saw a new CVE batch published (spike), then resumed
// remediation.
//
//   Day -29 → -20 : initial discovery plateau                     intensity ~9.4-8.0
//   Day -19 → -14 : initial remediation                           intensity ~7.6-5.9
//   Day -13 → -10 : new CVE batch                                 intensity ~6.8-9.3
//   Day  -9 →   0 : steady remediation                            intensity ~8.8-3.8
//
// Each scan varies naturally, with a maximum of 10 CVEs or findings.

const pgMaxVulns = 10
const pgMaxFindings = 10

type scaProfile struct {
	projectName string
	targetFile  string
	deps        []Dependency
	vulnPool    []Vulnerability
}

type infraProfile struct {
	imageName      string
	hostname       string
	osName         string
	osVersion      string
	osCodename     string
	packageManager string
	packages       []Package
	vulnPool       []Vulnerability
}

type findingProfile struct {
	targetName       string
	filesScanned     int
	resourcesScanned int
	findingPool      []Finding
}

// trendIntensity returns a 0-10 score for the given day offset from today.
// Higher = more vulnerabilities expected in each scan.
func trendIntensity(dayOffset int) float64 {
	switch {
	case dayOffset >= 20:
		return 8.0 + float64(dayOffset-20)*0.15
	case dayOffset >= 14:
		return 8.0 - float64(20-dayOffset)*0.35
	case dayOffset >= 10:
		return 5.9 + float64(14-dayOffset)*0.85
	default:
		return 9.3 - float64(10-dayOffset)*0.55
	}
}

// vulnProbability returns the probability a vulnerability of the given severity
// appears in a scan at the given intensity level (0-10 scale).
// Critical uses a quadratic curve so it disappears fastest when teams patch.
// Medium has the highest floor so it remains the most common severity.
func vulnProbability(severity string, intensity float64) float64 {
	t := intensity / 10.0
	if t > 1.0 {
		t = 1.0
	}
	switch strings.ToLower(severity) {
	case "critical":
		return 0.02 + t*t*0.18
	case "high":
		return 0.07 + t*0.28
	case "medium":
		return 0.14 + t*0.36
	case "low":
		return 0.10 + t*0.22
	}
	return 0
}

// pickVulns samples the pool probabilistically and caps outlier scans.
func pickVulns(pool []Vulnerability, rng *rand.Rand, intensity float64) []Vulnerability {
	out := make([]Vulnerability, 0, len(pool))
	for _, v := range pool {
		if rng.Float64() < vulnProbability(v.Severity, intensity) {
			out = append(out, v)
		}
	}
	if len(out) > pgMaxVulns {
		rng.Shuffle(len(out), func(i, j int) {
			out[i], out[j] = out[j], out[i]
		})
		out = out[:pgMaxVulns]
	}
	return out
}

func pickFindings(pool []Finding, rng *rand.Rand, intensity float64) []Finding {
	out := make([]Finding, 0, len(pool))
	for _, finding := range pool {
		if rng.Float64() < vulnProbability(finding.Severity, intensity) {
			out = append(out, finding)
		}
	}
	if len(out) == 0 && len(pool) > 0 {
		out = append(out, pool[rng.Intn(len(pool))])
	}
	if len(out) > pgMaxFindings {
		rng.Shuffle(len(out), func(i, j int) {
			out[i], out[j] = out[j], out[i]
		})
		out = out[:pgMaxFindings]
	}
	return out
}

func pgID(counter int) bson.ObjectID {
	id, err := bson.ObjectIDFromHex(fmt.Sprintf("66000000000000%010x", counter))
	if err != nil {
		panic(fmt.Sprintf("pgID overflow at counter %d", counter))
	}
	return id
}

func playgroundScans(workspace Workspace, now time.Time) []Scan {
	rng := rand.New(rand.NewSource(0x52756e74))

	sca := buildSCAProfiles()
	sast := buildSASTProfiles()
	containers := buildContainerProfiles()
	hosts := buildHostProfiles()
	k8s := buildK8sProfiles()

	var scans []Scan
	counter := 0
	next := func(s Scan) Scan {
		counter++
		s.ID = pgID(counter)
		return s
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	at := func(dayOffset, hour int) time.Time {
		if dayOffset == 0 {
			return now.Add(-time.Duration(hour+1) * time.Minute)
		}
		return today.AddDate(0, 0, -dayOffset).Add(time.Duration(hour) * time.Hour)
	}

	for dayOffset := playgroundHistoryDays - 1; dayOffset >= 0; dayOffset-- {
		intensity := trendIntensity(dayOffset)

		// SCA: 2 per day, rotating through 14 profiles
		for i := 0; i < 2; i++ {
			p := sca[(dayOffset*2+i)%len(sca)]
			scans = append(scans, next(pgSCA(workspace, p.projectName, p.targetFile,
				at(dayOffset, 6+rng.Intn(4)+i*9), p.deps,
				pickVulns(p.vulnPool, rng, intensity))))
		}

		// SAST: 2 per day, matching the SCA application names
		for i := 0; i < 2; i++ {
			p := sast[(dayOffset*2+i+3)%len(sast)]
			filesScanned := p.filesScanned + rng.Intn(18)
			scans = append(scans, next(pgSAST(workspace, p.targetName,
				at(dayOffset, 5+rng.Intn(3)+i*8),
				filesScanned, pickFindings(p.findingPool, rng, intensity))))
		}

		// Container: 2 per day, rotating through 14 profiles
		for i := 0; i < 2; i++ {
			p := containers[(dayOffset*2+i+5)%len(containers)]
			scans = append(scans, next(pgContainer(workspace, p.imageName,
				at(dayOffset, 2+rng.Intn(3)+i*7),
				p.osName, p.osVersion, p.osCodename, p.packageManager,
				p.packages, pickVulns(p.vulnPool, rng, intensity))))
		}

		// Host: 1 per day, rotating through 5 profiles
		{
			p := hosts[dayOffset%len(hosts)]
			scans = append(scans, next(pgHost(workspace, p.hostname,
				at(dayOffset, 1+rng.Intn(2)),
				p.osName, p.osVersion, p.osCodename, p.packageManager,
				p.packages, pickVulns(p.vulnPool, rng, intensity))))
		}

		// K8s: 1 per day, rotating through 5 clusters
		{
			p := k8s[(dayOffset+2)%len(k8s)]
			resourcesScanned := p.resourcesScanned + rng.Intn(24)
			scans = append(scans, next(pgK8s(workspace, p.targetName,
				at(dayOffset, 3+rng.Intn(3)),
				resourcesScanned, pickFindings(p.findingPool, rng, intensity))))
		}
	}

	return scans
}

// ── helpers ────────────────────────────────────────────────────────────────────

func npm(n, r, v string) Dependency { return pgDep(n, r, v, "npm") }
func pip(n, r, v string) Dependency { return pgDep(n, r, v, "pypi") }
func mvn(n, r, v string) Dependency { return pgDep(n, r, v, "maven") }
func gom(n, r, v string) Dependency { return pgDep(n, r, v, "go") }
func nv(id, pkg, sev, inst, rng, fix, sum, eco string) Vulnerability {
	return pgVuln(id, pkg, sev, inst, rng, fix, sum, eco)
}

// commonNPMVulns returns a slice of well-known npm ecosystem CVEs that affect
// a broad range of Node.js projects regardless of their specific dependencies.
func commonNPMVulns() []Vulnerability {
	return []Vulnerability{
		nv("CVE-2024-27980", "nodejs", "critical", "20.x", "< 20.12.1", "20.12.1", "Command injection via child_process in Windows batch-file handling.", "npm"),
		nv("CVE-2025-27152", "axios", "critical", "1.7.4", "< 1.8.2", "1.8.2", "SSRF via credential exposure when following absolute redirect URLs.", "npm"),
		nv("CVE-2024-45296", "path-to-regexp", "high", "0.1.7", "< 0.1.10", "0.1.10", "ReDoS via backtracking in route-pattern matching.", "npm"),
		nv("CVE-2024-21538", "cross-spawn", "high", "7.0.3", "< 7.0.5", "7.0.5", "ReDoS in shell argument handling.", "npm"),
		nv("CVE-2024-4067", "micromatch", "medium", "4.0.5", "< 4.0.8", "4.0.8", "ReDoS via backtracking in glob pattern matching.", "npm"),
		nv("CVE-2024-4068", "braces", "medium", "3.0.2", "< 3.0.3", "3.0.3", "ReDoS via crafted string in braces expansion.", "npm"),
		nv("CVE-2023-26115", "semver", "medium", "7.5.2", "< 7.5.4", "7.5.4", "ReDoS in semver range parsing.", "npm"),
		nv("CVE-2024-29041", "express", "medium", "4.18.x", "< 4.21.2", "4.21.2", "Open redirect via malformed URL in res.location().", "npm"),
		nv("CVE-2024-37890", "ws", "medium", "8.17.0", "< 8.17.1", "8.17.1", "DoS via crafted HTTP upgrade request to WebSocket server.", "npm"),
		nv("CVE-2022-3517", "minimatch", "medium", "3.0.4", "< 3.0.5", "3.0.5", "ReDoS via crafted glob pattern in minimatch.", "npm"),
		nv("CVE-2024-28849", "follow-redirects", "medium", "1.15.4", "< 1.15.6", "1.15.6", "Proxy-Authorization header leaked on cross-protocol redirect.", "npm"),
		nv("GHSA-w5p7-h5w8-2hfq", "trim", "medium", "0.0.1", "< 1.0.0", "1.0.0", "ReDoS via crafted string in trim prototype method.", "npm"),
		nv("CVE-2024-45812", "vite", "low", "5.x", "< 5.4.15", "5.4.15", "Source file read via path traversal in dev server.", "npm"),
		nv("CVE-2023-26136", "tough-cookie", "low", "4.1.2", "< 4.1.3", "4.1.3", "Prototype pollution via CookieJar.setCookie().", "npm"),
	}
}

// commonPypiVulns returns CVEs that affect the Python ecosystem broadly.
func commonPypiVulns() []Vulnerability {
	return []Vulnerability{
		nv("CVE-2024-6345", "setuptools", "critical", "69.1.1", "< 70.0.0", "70.0.0", "RCE via crafted package URL in package_index download.", "pypi"),
		nv("CVE-2022-40897", "setuptools", "critical", "65.5.0", "< 65.5.1", "65.5.1", "ReDoS in HTML parser in package_index.py.", "pypi"),
		nv("CVE-2024-3651", "idna", "high", "3.6", "< 3.7", "3.7", "CPU exhaustion with crafted DNS label.", "pypi"),
		nv("CVE-2023-43804", "urllib3", "high", "1.26.15", "< 1.26.18", "1.26.18", "Cookie header not stripped on cross-origin redirects.", "pypi"),
		nv("CVE-2023-45803", "urllib3", "medium", "1.26.15", "< 1.26.18", "1.26.18", "Request body not cleared after redirect with method change.", "pypi"),
		nv("CVE-2024-35195", "requests", "medium", "2.31.0", "< 2.32.2", "2.32.2", "Credentials sent to redirected non-HTTPS hosts.", "pypi"),
		nv("CVE-2023-27043", "python", "medium", "3.11.0", "< 3.11.4", "3.11.4", "Email address parsing discrepancy allows domain spoofing.", "pypi"),
		nv("CVE-2024-7264", "lxml", "medium", "5.1.0", "< 5.2.1", "5.2.1", "Null pointer dereference in lxml HTML parser.", "pypi"),
		nv("CVE-2024-28849", "aiohttp", "medium", "3.9.3", "< 3.9.4", "3.9.4", "Proxy-Authorization header leaked on cross-protocol redirect.", "pypi"),
		nv("CVE-2024-52304", "aiohttp", "medium", "3.9.3", "< 3.10.11", "3.10.11", "HTTP request smuggling via crafted Content-Length and Transfer-Encoding.", "pypi"),
	}
}

// commonMavenVulns returns CVEs that affect Spring/Jackson/Netty projects broadly.
func commonMavenVulns() []Vulnerability {
	return []Vulnerability{
		nv("CVE-2021-44228", "org.apache.logging.log4j:log4j-core", "critical", "2.14.1", ">= 2.0, < 2.15.0", "2.15.0", "Log4Shell: JNDI lookup in log messages enables RCE.", "maven"),
		nv("CVE-2021-44832", "org.apache.logging.log4j:log4j-core", "critical", "2.17.0", ">= 2.0, < 2.17.2", "2.17.2", "RCE via JDBC Appender when attacker controls log4j configuration.", "maven"),
		nv("CVE-2023-35116", "com.fasterxml.jackson.core:jackson-databind", "critical", "2.15.1", "< 2.15.2", "2.15.2", "Stack overflow DoS via deeply nested JSON arrays.", "maven"),
		nv("CVE-2022-42003", "com.fasterxml.jackson.core:jackson-databind", "high", "2.14.3", "< 2.14.4", "2.14.4", "Uncontrolled resource consumption in deep deserialization.", "maven"),
		nv("CVE-2022-42004", "com.fasterxml.jackson.core:jackson-databind", "high", "2.14.3", "< 2.14.4", "2.14.4", "DoS via deeply nested array deserialization.", "maven"),
		nv("CVE-2023-34462", "io.netty:netty-all", "high", "4.1.87.Final", "< 4.1.94.Final", "4.1.94.Final", "Memory exhaustion via crafted HTTP/2 headers.", "maven"),
		nv("CVE-2023-20883", "org.springframework.boot:spring-boot-starter-web", "high", "3.0.6", "< 3.0.7", "3.0.7", "DoS via Spring MVC error handling with specific Accept headers.", "maven"),
		nv("CVE-2024-22243", "org.springframework:spring-webmvc", "high", "6.0.0", "< 6.0.18", "6.0.18", "URL parsing discrepancy allows open redirect.", "maven"),
		nv("CVE-2022-1471", "org.yaml:snakeyaml", "medium", "1.33", "< 2.0", "2.0", "Deserialization allows arbitrary Java class instantiation.", "maven"),
		nv("CVE-2021-29425", "commons-io:commons-io", "medium", "2.11.0", "< 2.14.0", "2.14.0", "Path traversal in FileNameUtils.normalize().", "maven"),
		nv("CVE-2024-38808", "org.springframework:spring-expression", "medium", "6.1.10", "< 6.1.11", "6.1.11", "DoS via crafted SpEL expression.", "maven"),
		nv("CVE-2024-26308", "org.apache.commons:commons-compress", "medium", "1.24.0", "< 1.26.0", "1.26.0", "OOM via crafted Brotli compressed data.", "maven"),
	}
}

// commonOSVulns returns vulnerabilities affecting Debian/Ubuntu base images.
func commonOSVulns() []Vulnerability {
	return []Vulnerability{
		nv("CVE-2023-0465", "openssl", "critical", "3.0.8", "< 3.0.9", "3.0.9", "Invalid certificate policy constraint bypass in X.509 chain.", "debian"),
		nv("CVE-2023-0466", "openssl", "critical", "3.0.8", "< 3.0.9", "3.0.9", "Certificate policy check ignored with X509_V_FLAG_EXPLICIT_POLICY.", "debian"),
		nv("CVE-2023-38545", "curl", "critical", "7.88.x", "< 7.88.1-10+deb12u6", "7.88.1-10+deb12u6", "SOCKS5 heap buffer overflow via oversized hostname.", "debian"),
		nv("CVE-2024-0727", "openssl", "high", "3.0.11", "< 3.0.13", "3.0.13", "DoS via null pointer in PKCS12 certificate processing.", "debian"),
		nv("CVE-2023-38039", "curl", "high", "7.88.x", "< 7.88.1-10+deb12u6", "7.88.1-10+deb12u6", "Memory exhaustion via excessive HTTP response headers.", "debian"),
		nv("CVE-2023-3817", "openssl", "high", "3.0.11", "< 3.0.12", "3.0.12", "Excessive time spent checking DH keys and parameters.", "debian"),
		nv("CVE-2024-25062", "libxml2", "medium", "2.9.14", "< 2.9.14+dfsg-1.3~deb12u2", "2.9.14+dfsg-1.3~deb12u2", "Use-after-free in XML DTD validation.", "debian"),
		nv("CVE-2023-5363", "openssl", "medium", "3.0.11", "< 3.0.12", "3.0.12", "Incorrect cipher key and IV handling in EVP functions.", "debian"),
		nv("CVE-2024-2511", "openssl", "medium", "3.0.11", "< 3.0.14", "3.0.14", "Unbounded memory growth in TLS session cache.", "debian"),
		nv("CVE-2023-46218", "curl", "medium", "7.88.x", "< 7.88.1-10+deb12u7", "7.88.1-10+deb12u7", "Cookie mixed case PSL bypass allows exfiltration.", "debian"),
		nv("CVE-2024-6197", "curl", "medium", "7.88.x", "< 7.88.1-10+deb12u8", "7.88.1-10+deb12u8", "ASN.1 integer overflow in TLS certificate chain parsing.", "debian"),
		nv("CVE-2023-52425", "libxml2", "medium", "2.9.14", "< 2.9.14+dfsg-1.3~deb12u3", "2.9.14+dfsg-1.3~deb12u3", "DoS in XML_ExpandEntitiesInAttributeValues.", "debian"),
		nv("CVE-2024-9143", "openssl", "low", "3.0.11", "< 3.0.15", "3.0.15", "Out-of-bounds memory access in EC key operations.", "debian"),
		nv("CVE-2024-7264", "openssl", "low", "3.0.11", "< 3.0.15", "3.0.15", "Out-of-bounds read in ASN.1 GeneralizedTime parser.", "debian"),
	}
}

// ── SCA profiles (14 projects) ─────────────────────────────────────────────────

func buildSCAProfiles() []scaProfile {
	return []scaProfile{
		{
			projectName: "payments-api", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("axios", "^0.21.1", "0.21.1"), npm("express", "^4.18.2", "4.18.2"),
				npm("jsonwebtoken", "^8.5.1", "8.5.1"), npm("lodash", "^4.17.20", "4.17.20"),
				npm("minimist", "^1.2.5", "1.2.5"), npm("validator", "^13.9.0", "13.9.0"),
				npm("bcrypt", "^5.1.0", "5.1.0"), npm("uuid", "^9.0.0", "9.0.0"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2022-23529", "jsonwebtoken", "critical", "8.5.1", "< 9.0.0", "9.0.0", "Token verification bypass — forged JWTs accepted without valid signature.", "npm"),
				nv("CVE-2022-23540", "jsonwebtoken", "critical", "8.5.1", "< 9.0.0", "9.0.0", "Insecure default algorithm allows signature forgery.", "npm"),
				nv("GHSA-67mh-4wv8-2f99", "axios", "high", "0.21.1", "< 1.6.0", "1.6.0", "SSRF when axios follows unsafe absolute redirect URLs.", "npm"),
				nv("CVE-2023-45857", "axios", "high", "0.21.1", "< 1.6.8", "1.6.8", "CSRF via credential leakage in XSRF-TOKEN header.", "npm"),
				nv("GHSA-x32c-gh8h-3cvq", "validator", "high", "13.9.0", "< 13.12.0", "13.12.0", "ReDoS in isEmail() with deeply nested quoted-string input.", "npm"),
				nv("CVE-2022-23541", "jsonwebtoken", "high", "8.5.1", "< 9.0.0", "9.0.0", "Algorithm confusion allows token forgery via none algorithm.", "npm"),
				nv("CVE-2021-23337", "lodash", "medium", "4.17.20", "< 4.17.21", "4.17.21", "Command injection via template handling.", "npm"),
				nv("CVE-2020-28500", "lodash", "medium", "4.17.20", "< 4.17.21", "4.17.21", "ReDoS in lodash string utility functions.", "npm"),
				nv("GHSA-vh95-rmgr-6w4m", "minimist", "medium", "1.2.5", "< 1.2.6", "1.2.6", "Prototype pollution in argument parsing.", "npm"),
				nv("CVE-2019-10744", "lodash", "low", "4.17.20", "< 4.17.21", "4.17.21", "Prototype pollution via defaultsDeep.", "npm"),
				nv("CVE-2022-0536", "follow-redirects", "low", "1.14.8", "< 1.14.9", "1.14.9", "Sensitive URL data exposed in debug log.", "npm"),
			),
		},
		{
			projectName: "backoffice-web", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("vite", "^4.4.9", "4.4.9"), npm("webpack", "^5.89.0", "5.89.0"),
				npm("postcss", "^8.4.31", "8.4.31"), npm("semver", "^7.5.2", "7.5.2"),
				npm("cookie", "^0.5.0", "0.5.0"), npm("@next/env", "^14.0.0", "14.0.0"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2024-23331", "vite", "critical", "4.4.9", "< 5.0.12", "5.0.12", "Path traversal in dev server exposes arbitrary project files.", "npm"),
				nv("CVE-2025-30208", "vite", "critical", "4.4.9", "< 6.2.3", "6.2.3", "Query-string bypass allows reading arbitrary source files.", "npm"),
				nv("GHSA-92r3-m2mg-pj97", "vite", "high", "4.4.9", "< 4.5.2", "4.5.2", "Dev server exposes arbitrary files via crafted requests.", "npm"),
				nv("CVE-2023-28154", "webpack", "high", "5.89.0", "< 5.94.0", "5.94.0", "Prototype pollution via module federation manifest.", "npm"),
				nv("GHSA-c2qf-rxjj-qqgw", "cookie", "high", "0.5.0", "< 0.7.0", "0.7.0", "Out-of-bounds cookie version causes denial of service.", "npm"),
				nv("GHSA-7fh5-64p2-3v2j", "postcss", "medium", "8.4.31", "< 8.4.32", "8.4.32", "Line-return parsing causes unexpected CSS output.", "npm"),
				nv("GHSA-3xgq-45jj-v275", "postcss", "medium", "8.4.31", "< 8.4.39", "8.4.39", "CSS injection via malformed postcss-js input.", "npm"),
				nv("CVE-2024-45812", "vite", "low", "4.4.9", "< 5.4.15", "5.4.15", "Arbitrary file read via URL path traversal.", "npm"),
			),
		},
		{
			projectName: "auth-service", targetFile: "pom.xml",
			deps: []Dependency{
				mvn("org.springframework.boot:spring-boot-starter-web", "3.1.0", "3.1.0"),
				mvn("org.apache.logging.log4j:log4j-core", "2.17.1", "2.17.1"),
				mvn("com.fasterxml.jackson.core:jackson-databind", "2.14.3", "2.14.3"),
				mvn("io.netty:netty-all", "4.1.87.Final", "4.1.87.Final"),
				mvn("org.yaml:snakeyaml", "1.33", "1.33"),
				mvn("commons-io:commons-io", "2.11.0", "2.11.0"),
			},
			vulnPool: append(commonMavenVulns(),
				nv("CVE-2023-51074", "com.jayway.jsonpath:json-path", "critical", "2.8.0", "< 2.9.0", "2.9.0", "Stack overflow DoS via deeply nested filter path expressions.", "maven"),
				nv("CVE-2022-31692", "org.springframework.security:spring-security-core", "high", "6.0.3", "< 6.0.5", "6.0.5", "Authorization bypass via mixed-case roles.", "maven"),
				nv("CVE-2023-34034", "org.springframework.security:spring-security-core", "high", "6.0.3", "< 6.1.6", "6.1.6", "Authentication bypass via wildcard in WebFlux security config.", "maven"),
				nv("CVE-2024-22234", "org.springframework.security:spring-security-core", "medium", "6.0.3", "< 6.2.3", "6.2.3", "Authentication check bypassed when no authority patterns exist.", "maven"),
				nv("CVE-2023-34035", "org.springframework.security:spring-security-core", "medium", "6.0.3", "< 6.1.6", "6.1.6", "Authorization rules incorrectly applied to actuator endpoints.", "maven"),
				nv("CVE-2023-34462", "io.netty:netty-all", "low", "4.1.87.Final", "< 4.1.94.Final", "4.1.94.Final", "Partial memory leak in SniHandler on connection error.", "maven"),
			),
		},
		{
			projectName: "data-pipeline", targetFile: "requirements.txt",
			deps: []Dependency{
				pip("requests", "==2.28.2", "2.28.2"), pip("boto3", "==1.26.140", "1.26.140"),
				pip("cryptography", "==40.0.2", "40.0.2"), pip("Pillow", "==9.5.0", "9.5.0"),
				pip("paramiko", "==3.1.0", "3.1.0"), pip("pandas", "==2.0.3", "2.0.3"),
			},
			vulnPool: append(commonPypiVulns(),
				nv("CVE-2023-38408", "paramiko", "critical", "3.1.0", "< 3.4.0", "3.4.0", "RCE via malicious PKCS#11 provider loaded through ssh-agent.", "pypi"),
				nv("CVE-2023-49083", "cryptography", "high", "40.0.2", "< 41.0.6", "41.0.6", "NULL pointer dereference in PKCS12 parsing.", "pypi"),
				nv("CVE-2023-32681", "requests", "high", "2.28.2", "< 2.31.0", "2.31.0", "Proxy-Authorization header leaked on cross-origin redirect.", "pypi"),
				nv("CVE-2023-44271", "Pillow", "medium", "9.5.0", "< 10.0.2", "10.0.2", "Uncontrolled resource consumption via crafted image.", "pypi"),
				nv("CVE-2023-4863", "Pillow", "medium", "9.5.0", "< 10.0.2", "10.0.2", "Heap buffer overflow in WebP image parsing.", "pypi"),
				nv("CVE-2024-28219", "Pillow", "medium", "9.5.0", "< 10.3.0", "10.3.0", "Buffer overflow in _imagingcms C extension.", "pypi"),
				nv("CVE-2024-52304", "aiohttp", "low", "3.9.3", "< 3.10.11", "3.10.11", "HTTP request smuggling via chunked Transfer-Encoding.", "pypi"),
			),
		},
		{
			projectName: "billing-service", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("stripe", "^13.5.0", "13.5.0"), npm("express", "^4.17.1", "4.17.1"),
				npm("jsonwebtoken", "^8.5.1", "8.5.1"), npm("validator", "^13.9.0", "13.9.0"),
				npm("tough-cookie", "^4.1.2", "4.1.2"), npm("nodemailer", "^6.9.5", "6.9.5"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2022-23529", "jsonwebtoken", "critical", "8.5.1", "< 9.0.0", "9.0.0", "Token verification bypass allows forged JWTs.", "npm"),
				nv("CVE-2022-23540", "jsonwebtoken", "critical", "8.5.1", "< 9.0.0", "9.0.0", "Insecure default algorithm enables signature forgery.", "npm"),
				nv("CVE-2023-26136", "tough-cookie", "high", "4.1.2", "< 4.1.3", "4.1.3", "Prototype pollution via CookieJar.setCookie().", "npm"),
				nv("GHSA-x32c-gh8h-3cvq", "validator", "high", "13.9.0", "< 13.12.0", "13.12.0", "ReDoS in isEmail with nested quoted strings.", "npm"),
				nv("CVE-2022-23541", "jsonwebtoken", "medium", "8.5.1", "< 9.0.0", "9.0.0", "Algorithm confusion allows unauthorized token acceptance.", "npm"),
				nv("GHSA-c2qf-rxjj-qqgw", "cookie", "medium", "0.5.0", "< 0.7.0", "0.7.0", "DoS via out-of-bounds cookie version.", "npm"),
				nv("CVE-2022-0536", "follow-redirects", "low", "1.14.8", "< 1.14.9", "1.14.9", "Sensitive URL data in debug log.", "npm"),
			),
		},
		{
			projectName: "ml-inference", targetFile: "requirements.txt",
			deps: []Dependency{
				pip("tensorflow", "==2.12.0", "2.12.0"), pip("Pillow", "==9.4.0", "9.4.0"),
				pip("flask", "==2.3.0", "2.3.0"), pip("gunicorn", "==20.1.0", "20.1.0"),
				pip("numpy", "==1.24.2", "1.24.2"), pip("celery", "==5.3.4", "5.3.4"),
			},
			vulnPool: append(commonPypiVulns(),
				nv("CVE-2023-25659", "tensorflow", "critical", "2.12.0", "< 2.14.0", "2.14.0", "RCE via crafted SavedModel deserialization.", "pypi"),
				nv("CVE-2023-30861", "flask", "high", "2.3.0", "< 2.3.3", "2.3.3", "Session cookie sent to third parties due to missing SameSite.", "pypi"),
				nv("CVE-2023-44271", "Pillow", "high", "9.4.0", "< 10.0.2", "10.0.2", "Uncontrolled resource consumption via crafted image.", "pypi"),
				nv("CVE-2023-4863", "Pillow", "medium", "9.4.0", "< 10.0.2", "10.0.2", "Heap buffer overflow in WebP image parsing.", "pypi"),
				nv("CVE-2024-28219", "Pillow", "medium", "9.4.0", "< 10.3.0", "10.3.0", "Buffer overflow in _imagingcms C extension.", "pypi"),
				nv("CVE-2023-32681", "requests", "medium", "2.28.0", "< 2.31.0", "2.31.0", "Proxy-Authorization header leaked on redirect.", "pypi"),
				nv("CVE-2024-52304", "aiohttp", "low", "3.9.3", "< 3.10.11", "3.10.11", "HTTP request smuggling via chunked encoding.", "pypi"),
			),
		},
		{
			projectName: "mobile-bff", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("fastify", "^4.21.0", "4.21.0"), npm("sharp", "^0.32.5", "0.32.5"),
				npm("multer", "^1.4.5-lts.1", "1.4.5-lts.1"), npm("mongoose", "^7.5.0", "7.5.0"),
				npm("ioredis", "^5.3.2", "5.3.2"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2024-28176", "jose", "critical", "4.15.4", "< 4.15.5", "4.15.5", "Excessive resource consumption via crafted JWE token.", "npm"),
				nv("CVE-2023-44270", "sharp", "high", "0.32.5", "< 0.33.2", "0.33.2", "Out-of-bounds read via crafted image in libvips.", "npm"),
				nv("GHSA-7m27-7ghc-44w9", "multer", "high", "1.4.5-lts.1", "< 1.4.5-lts.2", "1.4.5-lts.2", "Path traversal via unsanitized filename on upload.", "npm"),
				nv("CVE-2023-44378", "mongoose", "medium", "7.5.0", "< 7.5.3", "7.5.3", "Prototype pollution in Model.sanitize() bypasses field filters.", "npm"),
				nv("CVE-2024-29415", "ip", "medium", "2.0.0", "< 2.0.1", "2.0.1", "SSRF bypass via ambiguous private IP representation.", "npm"),
				nv("CVE-2023-42282", "ip", "medium", "1.1.8", "< 1.1.9", "1.1.9", "Improper IP categorization allows SSRF bypass.", "npm"),
				nv("GHSA-rp65-9cf3-cjxr", "nth-check", "low", "2.0.1", "< 2.0.2", "2.0.2", "Inefficient regex complexity in nth-check selector parsing.", "npm"),
			),
		},
		{
			projectName: "user-service", targetFile: "pom.xml",
			deps: []Dependency{
				mvn("org.springframework.security:spring-security-core", "6.0.3", "6.0.3"),
				mvn("com.google.code.gson:gson", "2.10.0", "2.10.0"),
				mvn("commons-io:commons-io", "2.11.0", "2.11.0"),
				mvn("org.yaml:snakeyaml", "1.33", "1.33"),
				mvn("com.jayway.jsonpath:json-path", "2.8.0", "2.8.0"),
			},
			vulnPool: append(commonMavenVulns(),
				nv("CVE-2023-51074", "com.jayway.jsonpath:json-path", "critical", "2.8.0", "< 2.9.0", "2.9.0", "Stack overflow DoS via deeply nested filter expressions.", "maven"),
				nv("CVE-2023-20883", "org.springframework.boot:spring-boot-starter-web", "critical", "3.0.0", "< 3.0.7", "3.0.7", "DoS via Spring MVC error handling with specific Accept headers.", "maven"),
				nv("CVE-2022-31692", "org.springframework.security:spring-security-core", "high", "6.0.3", "< 6.0.5", "6.0.5", "Authorization bypass via mixed-case role names.", "maven"),
				nv("CVE-2023-34034", "org.springframework.security:spring-security-core", "high", "6.0.3", "< 6.1.6", "6.1.6", "Authentication bypass via wildcard in WebFlux config.", "maven"),
				nv("CVE-2023-34035", "org.springframework.security:spring-security-core", "medium", "6.0.3", "< 6.1.6", "6.1.6", "Authorization rules incorrectly applied to actuator endpoints.", "maven"),
				nv("CVE-2024-22234", "org.springframework.security:spring-security-core", "low", "6.0.3", "< 6.2.3", "6.2.3", "Authentication check bypassed when no authority patterns exist.", "maven"),
			),
		},
		{
			projectName: "gateway-proxy", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("http-proxy", "^1.18.1", "1.18.1"), npm("express", "^4.18.2", "4.18.2"),
				npm("helmet", "^7.1.0", "7.1.0"), npm("cors", "^2.8.5", "2.8.5"),
				npm("node-fetch", "^2.6.11", "2.6.11"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2024-28176", "jose", "critical", "4.15.4", "< 4.15.5", "4.15.5", "Resource exhaustion via crafted JWE token.", "npm"),
				nv("CVE-2023-46809", "http-proxy", "high", "1.18.1", "< 1.18.2", "1.18.2", "SSRF via malformed host header in proxy requests.", "npm"),
				nv("CVE-2023-45133", "node-fetch", "high", "2.6.11", "< 2.7.0", "2.7.0", "Authorization header exposed on redirect to different origin.", "npm"),
				nv("CVE-2023-42282", "ip", "medium", "1.1.8", "< 1.1.9", "1.1.9", "Improper IP categorization allows SSRF bypass.", "npm"),
				nv("CVE-2022-24785", "moment", "medium", "2.29.4", "< 2.29.5", "2.29.5", "Path traversal in moment locale loading.", "npm"),
				nv("GHSA-rp65-9cf3-cjxr", "nth-check", "low", "2.0.1", "< 2.0.2", "2.0.2", "Inefficient regex in CSS selector parsing.", "npm"),
			),
		},
		{
			projectName: "admin-dashboard", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("next", "^13.4.12", "13.4.12"), npm("react", "^18.2.0", "18.2.0"),
				npm("webpack", "^5.88.1", "5.88.1"), npm("postcss", "^8.4.27", "8.4.27"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2024-34351", "next", "critical", "13.4.12", "< 14.1.1", "14.1.1", "SSRF via Host header injection in Next.js server actions.", "npm"),
				nv("CVE-2025-29927", "next", "critical", "13.4.12", "< 14.2.25", "14.2.25", "Authorization bypass via crafted x-middleware-subrequest header.", "npm"),
				nv("CVE-2023-46298", "next", "high", "13.4.12", "< 14.1.1", "14.1.1", "DoS via crafted HTTP request to Next.js API.", "npm"),
				nv("CVE-2023-28154", "webpack", "high", "5.88.1", "< 5.94.0", "5.94.0", "Prototype pollution via module federation manifest.", "npm"),
				nv("GHSA-3xgq-45jj-v275", "postcss", "medium", "8.4.27", "< 8.4.39", "8.4.39", "CSS injection via malformed postcss-js input.", "npm"),
				nv("GHSA-rp65-9cf3-cjxr", "nth-check", "low", "2.0.1", "< 2.0.2", "2.0.2", "Inefficient regex in CSS selector parsing.", "npm"),
			),
		},
		{
			projectName: "recommendation-engine", targetFile: "requirements.txt",
			deps: []Dependency{
				pip("flask", "==2.3.0", "2.3.0"), pip("scikit-learn", "==1.3.0", "1.3.0"),
				pip("numpy", "==1.25.2", "1.25.2"), pip("gunicorn", "==21.2.0", "21.2.0"),
				pip("urllib3", "==1.26.15", "1.26.15"),
			},
			vulnPool: append(commonPypiVulns(),
				nv("CVE-2023-37276", "aiohttp", "critical", "3.8.5", "< 3.9.0", "3.9.0", "HTTP request smuggling via crafted chunked body.", "pypi"),
				nv("CVE-2023-30861", "flask", "high", "2.3.0", "< 2.3.3", "2.3.3", "Session cookie sent to third parties due to missing SameSite.", "pypi"),
				nv("CVE-2023-32681", "requests", "medium", "2.28.0", "< 2.31.0", "2.31.0", "Proxy-Authorization leaked on cross-origin redirect.", "pypi"),
				nv("CVE-2024-52304", "aiohttp", "medium", "3.9.3", "< 3.10.11", "3.10.11", "HTTP request smuggling via chunked Transfer-Encoding.", "pypi"),
				nv("CVE-2024-28849", "aiohttp", "low", "3.9.3", "< 3.9.4", "3.9.4", "Proxy-Authorization header leaked on cross-protocol redirect.", "pypi"),
			),
		},
		{
			projectName: "cli-agent", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("commander", "^12.0.0", "12.0.0"), npm("tar", "^6.2.1", "6.2.1"),
				npm("undici", "^6.13.0", "6.13.0"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2024-28176", "jose", "critical", "4.15.4", "< 4.15.5", "4.15.5", "Resource exhaustion via crafted JWE token.", "npm"),
				nv("CVE-2024-28863", "tar", "high", "6.2.1", "< 6.2.2", "6.2.2", "DoS via crafted tar archive path traversal loop.", "npm"),
				nv("CVE-2024-30260", "undici", "high", "6.13.0", "< 6.18.2", "6.18.2", "Authorization header not stripped on cross-origin redirect.", "npm"),
				nv("CVE-2024-30261", "undici", "medium", "6.13.0", "< 6.18.2", "6.18.2", "Request body not cleared after redirect.", "npm"),
				nv("GHSA-rp65-9cf3-cjxr", "nth-check", "low", "2.0.1", "< 2.0.2", "2.0.2", "Inefficient regex in CSS selector parsing.", "npm"),
			),
		},
		{
			projectName: "notification-worker", targetFile: "go.sum",
			deps: []Dependency{
				gom("github.com/gin-gonic/gin", "v1.9.1", "v1.9.1"),
				gom("github.com/redis/go-redis/v9", "v9.2.1", "v9.2.1"),
				gom("github.com/segmentio/kafka-go", "v0.4.47", "v0.4.47"),
				gom("github.com/jackc/pgx/v5", "v5.4.3", "v5.4.3"),
				gom("golang.org/x/net", "v0.15.0", "v0.15.0"),
			},
			vulnPool: []Vulnerability{
				nv("GHSA-4374-p667-p6c8", "golang.org/x/net", "critical", "0.15.0", "< 0.17.0", "0.17.0", "HTTP/2 rapid reset enables DoS via stream cancellation flood.", "go"),
				nv("CVE-2023-44487", "golang.org/x/net", "critical", "0.15.0", "< 0.17.0", "0.17.0", "HTTP/2 Rapid Reset Attack server-side resource exhaustion.", "go"),
				nv("GHSA-m425-mq94-257g", "google.golang.org/grpc", "critical", "1.56.2", "< 1.56.3", "1.56.3", "HTTP/2 rapid reset DoS in gRPC server request handling.", "go"),
				nv("GHSA-jfh8-c2jp-hdph", "golang.org/x/net", "high", "0.15.0", "< 0.17.0", "0.17.0", "Uncontrolled resource consumption via HTTP/2 RST_STREAM flood.", "go"),
				nv("CVE-2024-24783", "golang.org/x/crypto", "high", "0.13.0", "< 0.17.0", "0.17.0", "RSA verification bypass via crafted certificate in TLS.", "go"),
				nv("GHSA-2wrh-6pvc-2jm9", "golang.org/x/net", "high", "0.15.0", "< 0.17.0", "0.17.0", "HTTP response smuggling in net/http.", "go"),
				nv("CVE-2024-24784", "golang.org/x/net", "medium", "0.15.0", "< 0.17.0", "0.17.0", "Incorrect parsing in net/mail ParseAddressList.", "go"),
				nv("CVE-2024-24785", "html/template", "medium", "go1.21.6", "< go1.21.7", "go1.21.7", "HTML template injection via malformed HTML input.", "go"),
				nv("GHSA-45x7-px36-x8w8", "golang.org/x/crypto", "medium", "0.13.0", "< 0.17.0", "0.17.0", "Panic in ssh package via crafted NewSessionRequest.", "go"),
				nv("CVE-2023-39323", "github.com/gin-gonic/gin", "medium", "1.9.1", "< 1.9.2", "1.9.2", "Path traversal in gin router via encoded path segments.", "go"),
				nv("CVE-2024-24790", "golang.org/x/net", "low", "0.15.0", "< 0.17.0", "0.17.0", "Incorrect handling of IPv6 addresses with zone ID.", "go"),
				nv("CVE-2024-24789", "archive/zip", "low", "go1.21.6", "< go1.21.11", "go1.21.11", "Zip slip vulnerability in archive/zip reader.", "go"),
			},
		},
		{
			projectName: "analytics-sdk", targetFile: "package-lock.json",
			deps: []Dependency{
				npm("lodash", "^4.17.20", "4.17.20"), npm("moment", "^2.29.4", "2.29.4"),
				npm("mixpanel", "^2.54.0", "2.54.0"), npm("axios", "^0.27.2", "0.27.2"),
			},
			vulnPool: append(commonNPMVulns(),
				nv("CVE-2023-45857", "axios", "critical", "0.27.2", "< 1.6.8", "1.6.8", "CSRF via XSRF-TOKEN credential leakage on redirect.", "npm"),
				nv("GHSA-67mh-4wv8-2f99", "axios", "high", "0.27.2", "< 1.6.0", "1.6.0", "SSRF when axios follows unsafe absolute redirect URLs.", "npm"),
				nv("CVE-2022-24785", "moment", "high", "2.29.4", "< 2.29.5", "2.29.5", "Path traversal in moment locale loading.", "npm"),
				nv("CVE-2021-23337", "lodash", "medium", "4.17.20", "< 4.17.21", "4.17.21", "Command injection via template handling.", "npm"),
				nv("CVE-2020-28500", "lodash", "medium", "4.17.20", "< 4.17.21", "4.17.21", "ReDoS in lodash string utility functions.", "npm"),
				nv("CVE-2019-10744", "lodash", "low", "4.17.20", "< 4.17.21", "4.17.21", "Prototype pollution via defaultsDeep.", "npm"),
			),
		},
	}
}

// ── SAST profiles (14 projects — matching SCA apps) ───────────────────────────

func buildSASTProfiles() []findingProfile {
	makeSAST := func(name string, filesScanned int, root string, extra []Finding) findingProfile {
		pool := []Finding{
			pgFinding("SAST003", "Possible hardcoded secret", "high", "secret", root+"/config/secrets.ts", 18, "", "", "", "A credential-like variable is assigned a long literal value.", "Move secrets to environment variables or a secret manager and rotate exposed values."),
			pgFinding("SAST004", "Dynamic code execution", "high", "injection", root+"/lib/expression-evaluator.ts", 44, "", "", "", "The code calls eval, which can execute attacker-controlled input.", "Replace eval with a parser or an explicit dispatch table for expected values."),
			pgFinding("SAST005", "Shell execution enabled", "high", "command-injection", root+"/jobs/export.ts", 71, "", "", "", "The code enables shell execution for a child process.", "Avoid shell mode and pass arguments as an array to a safe process execution API."),
			pgFinding("SAST006", "TLS verification disabled", "high", "transport-security", root+"/clients/internal-api.py", 39, "", "", "", "The code disables certificate verification.", "Keep TLS certificate verification enabled and configure trusted roots when needed."),
			pgFinding("SAST007", "Weak hash function", "low", "cryptography", root+"/lib/hash.ts", 26, "", "", "", "The code uses MD5 or SHA-1, which are not suitable for security-sensitive hashing.", "Use SHA-256 or a purpose-built password hashing function such as bcrypt or Argon2."),
		}
		pool = append(pool, extra...)
		return findingProfile{targetName: name, filesScanned: filesScanned, findingPool: pool}
	}

	return []findingProfile{
		makeSAST("payments-api", 118, "src/payments", []Finding{
			pgFinding("SAST001", "Possible private key committed", "critical", "secret", "src/payments/testdata/private-key.pem", 1, "", "", "", "The file contains a private key marker.", "Remove the key from source control, rotate it, and load it from a secret manager."),
			pgFinding("SAST002", "Possible AWS access key", "high", "secret", "src/payments/integrations/aws.ts", 23, "", "", "", "The line contains a value shaped like an AWS access key id.", "Rotate the key and use the runtime secret mechanism instead of committing credentials."),
		}),
		makeSAST("backoffice-web", 96, "apps/backoffice", []Finding{
			pgFinding("SAST003", "Possible hardcoded secret", "high", "secret", "apps/backoffice/.env.production", 5, "", "", "", "A credential-like variable is assigned a long literal value.", "Move secrets to environment variables or a secret manager and rotate exposed values."),
		}),
		makeSAST("auth-service", 132, "services/auth", []Finding{
			pgFinding("SAST006", "TLS verification disabled", "high", "transport-security", "services/auth/src/main/java/AuthClient.java", 88, "", "", "", "The code disables certificate verification.", "Keep TLS certificate verification enabled and configure trusted roots when needed."),
			pgFinding("SAST007", "Weak hash function", "low", "cryptography", "services/auth/src/main/java/LegacyPasswordHash.java", 41, "", "", "", "The code uses SHA-1 for password hashing.", "Use bcrypt, scrypt, or Argon2 for password storage."),
		}),
		makeSAST("data-pipeline", 84, "pipelines/data", []Finding{
			pgFinding("SAST005", "Shell execution enabled", "high", "command-injection", "pipelines/data/jobs/sync_s3.py", 63, "", "", "", "The code enables shell execution for a child process.", "Pass arguments as a list and validate user-controlled input."),
		}),
		makeSAST("billing-service", 106, "services/billing", []Finding{
			pgFinding("SAST002", "Possible AWS access key", "high", "secret", "services/billing/src/providers/aws.ts", 29, "", "", "", "The line contains a value shaped like an AWS access key id.", "Rotate the key and use the runtime secret mechanism instead of committing credentials."),
		}),
		makeSAST("ml-inference", 74, "services/ml", []Finding{
			pgFinding("SAST006", "TLS verification disabled", "high", "transport-security", "services/ml/inference/client.py", 52, "", "", "", "The code disables certificate verification.", "Keep TLS certificate verification enabled and configure trusted roots when needed."),
		}),
		makeSAST("mobile-bff", 91, "services/mobile-bff", []Finding{
			pgFinding("SAST004", "Dynamic code execution", "high", "injection", "services/mobile-bff/src/rules/runtime.ts", 57, "", "", "", "The code calls eval, which can execute attacker-controlled input.", "Replace eval with a parser or explicit dispatch table."),
		}),
		makeSAST("user-service", 127, "services/user", []Finding{
			pgFinding("SAST007", "Weak hash function", "low", "cryptography", "services/user/src/main/java/TokenDigest.java", 36, "", "", "", "The code uses MD5 or SHA-1, which are not suitable for security-sensitive hashing.", "Use SHA-256 or a purpose-built password hashing function such as bcrypt or Argon2."),
		}),
		makeSAST("gateway-proxy", 88, "services/gateway", []Finding{
			pgFinding("SAST005", "Shell execution enabled", "high", "command-injection", "services/gateway/scripts/reload.ts", 18, "", "", "", "The code enables shell execution for a child process.", "Avoid shell mode and pass arguments as an array to a safe process execution API."),
		}),
		makeSAST("admin-dashboard", 79, "apps/admin", []Finding{
			pgFinding("SAST003", "Possible hardcoded secret", "high", "secret", "apps/admin/src/lib/bootstrap.ts", 22, "", "", "", "A credential-like variable is assigned a long literal value.", "Move secrets to environment variables or a secret manager and rotate exposed values."),
		}),
		makeSAST("recommendation-engine", 69, "services/recommendations", []Finding{
			pgFinding("SAST004", "Dynamic code execution", "high", "injection", "services/recommendations/rules.py", 33, "", "", "", "The code calls eval, which can execute attacker-controlled input.", "Replace eval with a parser or an explicit dispatch table for expected values."),
		}),
		makeSAST("cli-agent", 58, "cmd/agent", []Finding{
			pgFinding("SAST001", "Possible private key committed", "critical", "secret", "cmd/agent/test-fixtures/id_rsa", 1, "", "", "", "The file contains a private key marker.", "Remove the key from source control, rotate it, and load it from a secret manager."),
		}),
		makeSAST("notification-worker", 82, "workers/notifications", []Finding{
			pgFinding("SAST002", "Possible AWS access key", "high", "secret", "workers/notifications/localstack.ts", 14, "", "", "", "The line contains a value shaped like an AWS access key id.", "Rotate the key and use the runtime secret mechanism instead of committing credentials."),
		}),
		makeSAST("analytics-sdk", 64, "packages/analytics-sdk", []Finding{
			pgFinding("SAST007", "Weak hash function", "low", "cryptography", "packages/analytics-sdk/src/fingerprint.ts", 49, "", "", "", "The code uses MD5 or SHA-1, which are not suitable for security-sensitive hashing.", "Use SHA-256 for non-secret fingerprints and stronger primitives for security-sensitive hashing."),
		}),
	}
}

// ── Kubernetes profiles (5 clusters) ──────────────────────────────────────────

func buildK8sProfiles() []findingProfile {
	makeK8s := func(cluster string, resourcesScanned int, namespace string, extra []Finding) findingProfile {
		pool := []Finding{
			pgFinding("K8S006", "Privilege escalation not disabled", "medium", "container-security", "kubectl:get/deployments", 0, "Deployment", "checkout-api/app", namespace, "Container does not set allowPrivilegeEscalation to false.", "Set securityContext.allowPrivilegeEscalation: false."),
			pgFinding("K8S007", "Container may run as root", "medium", "container-security", "kubectl:get/deployments", 0, "Deployment", "checkout-api/app", namespace, "Container does not require a non-root user.", "Set runAsNonRoot: true and provide a non-zero runAsUser."),
			pgFinding("K8S008", "Writable root filesystem", "low", "container-security", "kubectl:get/deployments", 0, "Deployment", "checkout-api/app", namespace, "Container root filesystem is writable.", "Set readOnlyRootFilesystem: true and mount writable volumes only where needed."),
			pgFinding("K8S009", "Mutable image tag", "medium", "supply-chain", "kubectl:get/deployments", 0, "Deployment", "worker/app", namespace, "Container image uses no tag or the latest tag.", "Pin images to an immutable tag or digest."),
			pgFinding("K8S015", "Missing resource requests or limits", "medium", "resilience", "kubectl:get/deployments", 0, "Deployment", "worker/app", namespace, "Container does not define both resource requests and limits.", "Set CPU and memory requests and limits for predictable scheduling and blast-radius control."),
			pgFinding("K8S016", "Missing readiness probe", "low", "resilience", "kubectl:get/deployments", 0, "Deployment", "worker/app", namespace, "Container has no readiness probe.", "Add a readiness probe for application workloads that receive traffic."),
			pgFinding("K8S017", "Missing liveness probe", "low", "resilience", "kubectl:get/deployments", 0, "Deployment", "worker/app", namespace, "Container has no liveness probe.", "Add a liveness probe when the app can enter an unrecoverable unhealthy state."),
			pgFinding("K8S019", "Service account token automounted", "medium", "rbac", "kubectl:get/deployments", 0, "Deployment", "checkout-api", namespace, "Workload can automatically mount a service account token.", "Set automountServiceAccountToken: false unless the workload needs to call the Kubernetes API."),
		}
		pool = append(pool, extra...)
		return findingProfile{targetName: cluster, resourcesScanned: resourcesScanned, findingPool: pool}
	}

	return []findingProfile{
		makeK8s("prod-us-east-1", 186, "payments", []Finding{
			pgFinding("K8S005", "Privileged container", "critical", "container-security", "kubectl:get/daemonsets", 0, "DaemonSet", "node-debug/debugger", "platform", "Container runs in privileged mode.", "Remove privileged mode and grant only the exact capabilities required."),
			pgFinding("K8S012", "cluster-admin binding", "critical", "rbac", "kubectl:get/clusterrolebindings", 0, "ClusterRoleBinding", "legacy-ci-admin", "", "Binding grants cluster-admin privileges.", "Bind the narrowest role required by the subject instead of cluster-admin."),
		}),
		makeK8s("prod-eu-west-1", 164, "billing", []Finding{
			pgFinding("K8S011", "Ingress without TLS", "medium", "network", "kubectl:get/ingress", 0, "Ingress", "billing-api", "billing", "Ingress has no TLS block configured.", "Configure TLS for ingress hosts and redirect HTTP traffic to HTTPS."),
			pgFinding("K8S013", "Wildcard RBAC rule", "high", "rbac", "kubectl:get/roles", 0, "Role", "billing-operator", "billing", "Role grants wildcard verbs or resources.", "Replace wildcards with explicit verbs and resource names."),
		}),
		makeK8s("staging-us-east-1", 119, "staging", []Finding{
			pgFinding("K8S010", "Public service exposure", "medium", "network", "kubectl:get/services", 0, "Service", "preview-api", "staging", "Service type loadbalancer can expose workloads outside the cluster.", "Use ClusterIP by default and expose services through controlled ingress when possible."),
			pgFinding("K8S018", "Default service account", "low", "rbac", "kubectl:get/deployments", 0, "Deployment", "preview-worker", "staging", "Workload uses the default service account.", "Create a dedicated service account with only the permissions required by this workload."),
		}),
		makeK8s("shared-tools", 91, "tools", []Finding{
			pgFinding("K8S004", "hostPath volume mounted", "high", "pod-security", "kubectl:get/pods", 0, "Pod", "docker-builder/buildkit", "tools", "Workload mounts host path /var/run/docker.sock.", "Avoid hostPath volumes or constrain them with strict admission policies."),
			pgFinding("K8S020", "Unable to read Kubernetes resource", "medium", "scanner", "kubectl:get/clusterroles", 0, "KubernetesResource", "clusterroles", "", "kubectl get clusterroles: forbidden by RBAC.", "Verify kubectl connectivity and RBAC permissions for this resource type."),
		}),
		makeK8s("dev-sandbox", 73, "sandbox", []Finding{
			pgFinding("K8S001", "hostNetwork enabled", "high", "pod-security", "kubectl:get/pods", 0, "Pod", "netshoot", "sandbox", "Workload uses the node network namespace.", "Disable hostNetwork unless the workload explicitly requires node networking."),
			pgFinding("K8S014", "Dangerous Linux capability", "high", "container-security", "kubectl:get/pods", 0, "Pod", "netshoot/debug", "sandbox", "Container adds broad Linux capabilities.", "Drop all capabilities by default and add only narrowly required capabilities."),
		}),
	}
}

// ── Container profiles (14 images — named after SCA apps) ─────────────────────

func buildContainerProfiles() []infraProfile {
	// All company containers share the same OS-level vuln pool; only packages differ.
	makeContainer := func(appName, baseImage, os, osVer, osCode, pkgMgr string, pkgs []Package, extra []Vulnerability) infraProfile {
		pool := append(commonOSVulns(), extra...)
		return infraProfile{
			imageName: "ghcr.io/company/" + appName + ":latest",
			osName:    os, osVersion: osVer, osCodename: osCode,
			packageManager: pkgMgr, packages: pkgs, vulnPool: pool,
		}
	}

	debPkgs := func(name string) []Package {
		return []Package{
			pgPkg("openssl", "3.0.11-1~deb12u2", "dpkg"),
			pgPkg("curl", "7.88.1-10+deb12u5", "dpkg"),
			pgPkg("libssl3", "3.0.11-1~deb12u2", "dpkg"),
			pgPkg("ca-certificates", "20230311", "dpkg"),
			pgPkg(name+"-bin", "1.0.0", "dpkg"),
		}
	}
	alpPkgs := func(name string) []Package {
		return []Package{
			pgPkg("openssl", "3.1.4-r4", "apk"),
			pgPkg("musl", "1.2.4-r2", "apk"),
			pgPkg("libssl3", "3.1.4-r4", "apk"),
			pgPkg(name+"-bin", "1.0.0", "apk"),
		}
	}

	alpExtra := func() []Vulnerability {
		return []Vulnerability{
			nv("CVE-2024-5535", "openssl", "critical", "3.1.4-r4", "< 3.1.7-r0", "3.1.7-r0", "SSL_select_next_proto buffer overread via crafted ALPN list.", "alpine"),
			nv("CVE-2023-4807", "openssl", "critical", "3.1.4-r4", "< 3.1.5-r0", "3.1.5-r0", "Buffer overread in POLY1305 MAC on X86_64.", "alpine"),
			nv("CVE-2024-0553", "openssl", "critical", "3.1.4-r4", "< 3.1.5-r0", "3.1.5-r0", "Excessive RSA PSS key check allows DoS.", "alpine"),
			nv("CVE-2024-4603", "openssl", "high", "3.1.4-r4", "< 3.1.5-r0", "3.1.5-r0", "Excessive time checking DSA key validity.", "alpine"),
			nv("CVE-2023-3446", "openssl", "medium", "3.1.4-r4", "< 3.1.5-r0", "3.1.5-r0", "Excessive time in DH check with large q parameter.", "alpine"),
		}
	}

	debExtra := func() []Vulnerability {
		return []Vulnerability{
			nv("CVE-2023-0465", "openssl", "critical", "3.0.11-1~deb12u2", "< 3.0.13", "3.0.13", "Invalid certificate policy constraint bypass.", "debian"),
			nv("CVE-2023-0466", "openssl", "critical", "3.0.11-1~deb12u2", "< 3.0.13", "3.0.13", "Certificate policy check ignored with explicit flag.", "debian"),
		}
	}

	return []infraProfile{
		makeContainer("payments-api", "debian:12", "Debian GNU/Linux", "12", "bookworm", "dpkg", debPkgs("payments-api"), debExtra()),
		makeContainer("backoffice-web", "node:20-bookworm-slim", "Debian GNU/Linux", "12", "bookworm", "dpkg", debPkgs("backoffice-web"), debExtra()),
		makeContainer("auth-service", "eclipse-temurin:21-jre", "Debian GNU/Linux", "12", "bookworm", "dpkg", debPkgs("auth-service"), debExtra()),
		makeContainer("data-pipeline", "python:3.11-slim", "Debian GNU/Linux", "11", "bullseye", "dpkg", debPkgs("data-pipeline"), debExtra()),
		makeContainer("billing-service", "node:20-bullseye-slim", "Debian GNU/Linux", "11", "bullseye", "dpkg", debPkgs("billing-service"), debExtra()),
		makeContainer("ml-inference", "python:3.11-slim", "Debian GNU/Linux", "11", "bullseye", "dpkg", debPkgs("ml-inference"), debExtra()),
		makeContainer("mobile-bff", "node:20-alpine", "Alpine Linux", "3.19", "", "apk", alpPkgs("mobile-bff"), alpExtra()),
		makeContainer("user-service", "eclipse-temurin:21-jre-alpine", "Alpine Linux", "3.19", "", "apk", alpPkgs("user-service"), alpExtra()),
		makeContainer("gateway-proxy", "nginx:alpine", "Alpine Linux", "3.18", "", "apk", alpPkgs("gateway-proxy"), alpExtra()),
		makeContainer("admin-dashboard", "node:20-alpine", "Alpine Linux", "3.19", "", "apk", alpPkgs("admin-dashboard"), alpExtra()),
		makeContainer("recommendation-engine", "python:3.11-alpine", "Alpine Linux", "3.19", "", "apk", alpPkgs("recommendation-engine"), alpExtra()),
		makeContainer("cli-agent", "alpine:3.19", "Alpine Linux", "3.19", "", "apk", alpPkgs("cli-agent"), alpExtra()),
		makeContainer("notification-worker", "golang:1.22-alpine", "Alpine Linux", "3.19", "", "apk", alpPkgs("notification-worker"), alpExtra()),
		makeContainer("analytics-sdk", "node:20-bookworm-slim", "Debian GNU/Linux", "12", "bookworm", "dpkg", debPkgs("analytics-sdk"), debExtra()),
	}
}

// ── Host profiles (5 machines) ────────────────────────────────────────────────

func buildHostProfiles() []infraProfile {
	v := nv
	return []infraProfile{
		{
			hostname: "prod-api-01", osName: "Ubuntu", osVersion: "22.04", osCodename: "jammy", packageManager: "dpkg",
			packages: []Package{pgPkg("openssl", "3.0.2-0ubuntu1.12", "dpkg"), pgPkg("curl", "7.81.0-1ubuntu1.15", "dpkg"), pgPkg("systemd", "249.11-0ubuntu3.11", "dpkg"), pgPkg("openssh-server", "1:8.9p1-3ubuntu0.6", "dpkg"), pgPkg("sudo", "1.9.9-1ubuntu2.4", "dpkg")},
			vulnPool: []Vulnerability{
				v("CVE-2023-38408", "openssh-server", "critical", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.7", "1:8.9p1-3ubuntu0.7", "RCE via ssh-agent forwarding with malicious PKCS#11 provider.", "ubuntu"),
				v("CVE-2024-6387", "openssh-server", "critical", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.10", "1:8.9p1-3ubuntu0.10", "Unauthenticated RCE via race condition in signal handler (regreSSHion).", "ubuntu"),
				v("CVE-2023-0465", "openssl", "critical", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.15", "3.0.2-0ubuntu1.15", "Invalid certificate policy constraint bypass.", "ubuntu"),
				v("CVE-2023-38545", "curl", "high", "7.81.0-1ubuntu1.15", "< 7.81.0-1ubuntu1.16", "7.81.0-1ubuntu1.16", "SOCKS5 heap buffer overflow.", "ubuntu"),
				v("CVE-2024-0727", "openssl", "high", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.15", "3.0.2-0ubuntu1.15", "DoS via null pointer in PKCS12 processing.", "ubuntu"),
				v("CVE-2023-48795", "openssh-server", "high", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.8", "1:8.9p1-3ubuntu0.8", "Terrapin attack downgrades SSH connection security.", "ubuntu"),
				v("CVE-2023-50868", "systemd", "medium", "249.11-0ubuntu3.11", "< 249.11-0ubuntu3.12", "249.11-0ubuntu3.12", "Resolver resource exhaustion via crafted DNS.", "ubuntu"),
				v("CVE-2023-26604", "systemd", "medium", "249.11-0ubuntu3.11", "< 249.11-0ubuntu3.11", "249.11-0ubuntu3.11", "Local privilege escalation via polkit policy.", "ubuntu"),
				v("CVE-2024-28085", "util-linux", "medium", "2.37.2-4ubuntu3.3", "< 2.37.2-4ubuntu3.4", "2.37.2-4ubuntu3.4", "Privilege escalation via wall command.", "ubuntu"),
				v("CVE-2023-51385", "openssh-server", "medium", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.8", "1:8.9p1-3ubuntu0.8", "Command injection via crafted hostname in ProxyCommand.", "ubuntu"),
				v("CVE-2023-5363", "openssl", "low", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.13", "3.0.2-0ubuntu1.13", "Incorrect cipher key handling in EVP functions.", "ubuntu"),
				v("CVE-2024-9143", "openssl", "low", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.17", "3.0.2-0ubuntu1.17", "Out-of-bounds memory access in EC key operations.", "ubuntu"),
			},
		},
		{
			hostname: "bastion-01", osName: "Ubuntu", osVersion: "22.04", osCodename: "jammy", packageManager: "dpkg",
			packages: []Package{pgPkg("openssh-server", "1:8.9p1-3ubuntu0.6", "dpkg"), pgPkg("sudo", "1.9.9-1ubuntu2.4", "dpkg"), pgPkg("libpam-modules", "1.4.0-11ubuntu2.4", "dpkg"), pgPkg("fail2ban", "0.11.2-9", "dpkg"), pgPkg("ufw", "0.36.1-4ubuntu0.1", "dpkg")},
			vulnPool: []Vulnerability{
				v("CVE-2023-38408", "openssh-server", "critical", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.7", "1:8.9p1-3ubuntu0.7", "RCE via ssh-agent forwarding with malicious PKCS#11 provider.", "ubuntu"),
				v("CVE-2024-6387", "openssh-server", "critical", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.10", "1:8.9p1-3ubuntu0.10", "Unauthenticated RCE via race condition in signal handler (regreSSHion).", "ubuntu"),
				v("CVE-2023-28531", "openssh-server", "critical", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.3", "1:8.9p1-3ubuntu0.3", "Private key exposure via ssh-add -L with PKCS11.", "ubuntu"),
				v("CVE-2023-48795", "openssh-server", "high", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.8", "1:8.9p1-3ubuntu0.8", "Terrapin attack downgrades SSH connection security.", "ubuntu"),
				v("CVE-2023-51385", "openssh-server", "high", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.8", "1:8.9p1-3ubuntu0.8", "Command injection via crafted hostname in ProxyCommand.", "ubuntu"),
				v("CVE-2023-4641", "sudo", "high", "1.9.9-1ubuntu2.4", "< 1.9.9-1ubuntu2.5", "1.9.9-1ubuntu2.5", "Memory disclosure when running commands with specific plugins.", "ubuntu"),
				v("CVE-2024-28085", "util-linux", "medium", "2.37.2-4ubuntu3.3", "< 2.37.2-4ubuntu3.4", "2.37.2-4ubuntu3.4", "Privilege escalation via wall command.", "ubuntu"),
				v("CVE-2023-26604", "systemd", "medium", "249.11-0ubuntu3.9", "< 249.11-0ubuntu3.11", "249.11-0ubuntu3.11", "Local privilege escalation via polkit policy.", "ubuntu"),
				v("CVE-2023-50868", "systemd", "medium", "249.11-0ubuntu3.9", "< 249.11-0ubuntu3.12", "249.11-0ubuntu3.12", "Resolver resource exhaustion via crafted DNS.", "ubuntu"),
				v("CVE-2023-5363", "openssl", "medium", "3.0.2-0ubuntu1.10", "< 3.0.2-0ubuntu1.12", "3.0.2-0ubuntu1.12", "Incorrect cipher key handling in EVP functions.", "ubuntu"),
				v("CVE-2023-38545", "curl", "low", "7.81.0-1ubuntu1.12", "< 7.81.0-1ubuntu1.16", "7.81.0-1ubuntu1.16", "SOCKS5 heap buffer overflow.", "ubuntu"),
				v("CVE-2024-9143", "openssl", "low", "3.0.2-0ubuntu1.10", "< 3.0.2-0ubuntu1.17", "3.0.2-0ubuntu1.17", "Out-of-bounds memory access in EC key operations.", "ubuntu"),
			},
		},
		{
			hostname: "prod-db-primary", osName: "Ubuntu", osVersion: "22.04", osCodename: "jammy", packageManager: "dpkg",
			packages: []Package{pgPkg("postgresql-15", "15.3-1.pgdg22.04+1", "dpkg"), pgPkg("openssl", "3.0.2-0ubuntu1.12", "dpkg"), pgPkg("curl", "7.81.0-1ubuntu1.15", "dpkg"), pgPkg("systemd", "249.11-0ubuntu3.11", "dpkg"), pgPkg("libssl3", "3.0.2-0ubuntu1.12", "dpkg")},
			vulnPool: []Vulnerability{
				v("CVE-2023-0465", "openssl", "critical", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.15", "3.0.2-0ubuntu1.15", "Invalid certificate policy constraint bypass.", "ubuntu"),
				v("CVE-2024-6387", "openssh-server", "critical", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.10", "1:8.9p1-3ubuntu0.10", "Unauthenticated RCE via race condition in signal handler.", "ubuntu"),
				v("CVE-2023-38545", "curl", "critical", "7.81.0-1ubuntu1.15", "< 7.81.0-1ubuntu1.16", "7.81.0-1ubuntu1.16", "SOCKS5 heap buffer overflow via oversized hostname.", "ubuntu"),
				v("CVE-2023-39417", "postgresql-15", "high", "15.3-1.pgdg22.04+1", "< 15.4-1.pgdg22.04+1", "15.4-1.pgdg22.04+1", "SQL injection via extension scripts with special search_path.", "ubuntu"),
				v("CVE-2024-0727", "openssl", "high", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.15", "3.0.2-0ubuntu1.15", "DoS via null pointer in PKCS12 certificate processing.", "ubuntu"),
				v("CVE-2023-38039", "curl", "high", "7.81.0-1ubuntu1.15", "< 7.81.0-1ubuntu1.16", "7.81.0-1ubuntu1.16", "Memory exhaustion via excessive HTTP response headers.", "ubuntu"),
				v("CVE-2023-50868", "systemd", "medium", "249.11-0ubuntu3.11", "< 249.11-0ubuntu3.12", "249.11-0ubuntu3.12", "Resolver resource exhaustion via crafted DNS.", "ubuntu"),
				v("CVE-2023-5363", "openssl", "medium", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.13", "3.0.2-0ubuntu1.13", "Incorrect cipher key handling in EVP functions.", "ubuntu"),
				v("CVE-2024-4317", "postgresql-15", "medium", "15.3-1.pgdg22.04+1", "< 15.7-1.pgdg22.04+1", "15.7-1.pgdg22.04+1", "Visibility restriction bypass via leaky EXPLAIN output.", "ubuntu"),
				v("CVE-2024-28085", "util-linux", "medium", "2.37.2-4ubuntu3.3", "< 2.37.2-4ubuntu3.4", "2.37.2-4ubuntu3.4", "Privilege escalation via wall command.", "ubuntu"),
				v("CVE-2024-9143", "openssl", "low", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.17", "3.0.2-0ubuntu1.17", "Out-of-bounds memory access in EC key operations.", "ubuntu"),
				v("CVE-2024-7264", "openssl", "low", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.17", "3.0.2-0ubuntu1.17", "Out-of-bounds read in ASN.1 GeneralizedTime parser.", "ubuntu"),
			},
		},
		{
			hostname: "prod-api-02", osName: "Ubuntu", osVersion: "22.04", osCodename: "jammy", packageManager: "dpkg",
			packages: []Package{pgPkg("openssl", "3.0.2-0ubuntu1.12", "dpkg"), pgPkg("nginx", "1.18.0-6ubuntu14.4", "dpkg"), pgPkg("curl", "7.81.0-1ubuntu1.15", "dpkg"), pgPkg("systemd", "249.11-0ubuntu3.11", "dpkg")},
			vulnPool: []Vulnerability{
				v("CVE-2023-0465", "openssl", "critical", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.15", "3.0.2-0ubuntu1.15", "Invalid certificate policy constraint bypass.", "ubuntu"),
				v("CVE-2024-6387", "openssh-server", "critical", "1:8.9p1-3ubuntu0.6", "< 1:8.9p1-3ubuntu0.10", "1:8.9p1-3ubuntu0.10", "Unauthenticated RCE via race condition in signal handler.", "ubuntu"),
				v("CVE-2023-38545", "curl", "critical", "7.81.0-1ubuntu1.15", "< 7.81.0-1ubuntu1.16", "7.81.0-1ubuntu1.16", "SOCKS5 heap buffer overflow via oversized hostname.", "ubuntu"),
				v("CVE-2023-44487", "nginx", "high", "1.18.0-6ubuntu14.4", "< 1.18.0-6ubuntu14.5", "1.18.0-6ubuntu14.5", "HTTP/2 Rapid Reset Attack causes DoS.", "ubuntu"),
				v("CVE-2023-38039", "curl", "high", "7.81.0-1ubuntu1.15", "< 7.81.0-1ubuntu1.16", "7.81.0-1ubuntu1.16", "Memory exhaustion via excessive HTTP response headers.", "ubuntu"),
				v("CVE-2024-0727", "openssl", "high", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.15", "3.0.2-0ubuntu1.15", "DoS via null pointer in PKCS12 processing.", "ubuntu"),
				v("CVE-2023-50868", "systemd", "medium", "249.11-0ubuntu3.11", "< 249.11-0ubuntu3.12", "249.11-0ubuntu3.12", "Resolver resource exhaustion via crafted DNS.", "ubuntu"),
				v("CVE-2023-5363", "openssl", "medium", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.13", "3.0.2-0ubuntu1.13", "Incorrect cipher key handling in EVP functions.", "ubuntu"),
				v("CVE-2024-28085", "util-linux", "medium", "2.37.2-4ubuntu3.3", "< 2.37.2-4ubuntu3.4", "2.37.2-4ubuntu3.4", "Privilege escalation via wall command.", "ubuntu"),
				v("CVE-2023-26604", "systemd", "medium", "249.11-0ubuntu3.11", "< 249.11-0ubuntu3.11", "249.11-0ubuntu3.11", "Local privilege escalation via polkit.", "ubuntu"),
				v("CVE-2024-9143", "openssl", "low", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.17", "3.0.2-0ubuntu1.17", "Out-of-bounds memory access in EC key operations.", "ubuntu"),
				v("CVE-2024-7264", "openssl", "low", "3.0.2-0ubuntu1.12", "< 3.0.2-0ubuntu1.17", "3.0.2-0ubuntu1.17", "Out-of-bounds read in ASN.1 GeneralizedTime parser.", "ubuntu"),
			},
		},
		{
			hostname: "build-runner-02", osName: "Ubuntu", osVersion: "20.04", osCodename: "focal", packageManager: "dpkg",
			packages: []Package{pgPkg("git", "1:2.25.1-1ubuntu3.11", "dpkg"), pgPkg("openssh-client", "1:8.2p1-4ubuntu0.9", "dpkg"), pgPkg("python3.8", "3.8.10-0ubuntu1~20.04.9", "dpkg"), pgPkg("sudo", "1.8.31-1ubuntu1.5", "dpkg"), pgPkg("curl", "7.68.0-1ubuntu2.20", "dpkg")},
			vulnPool: []Vulnerability{
				v("CVE-2024-32002", "git", "critical", "1:2.25.1-1ubuntu3.11", "< 1:2.25.1-1ubuntu3.13", "1:2.25.1-1ubuntu3.13", "RCE via recursive clone of malicious repository with submodule.", "ubuntu"),
				v("CVE-2023-38408", "openssh-client", "critical", "1:8.2p1-4ubuntu0.9", "< 1:8.2p1-4ubuntu0.11", "1:8.2p1-4ubuntu0.11", "RCE via ssh-agent forwarding with malicious PKCS#11 provider.", "ubuntu"),
				v("CVE-2023-38545", "curl", "critical", "7.68.0-1ubuntu2.20", "< 7.68.0-1ubuntu2.22", "7.68.0-1ubuntu2.22", "SOCKS5 heap buffer overflow via oversized hostname.", "ubuntu"),
				v("CVE-2023-48795", "openssh-client", "high", "1:8.2p1-4ubuntu0.9", "< 1:8.2p1-4ubuntu0.11", "1:8.2p1-4ubuntu0.11", "Terrapin attack downgrades SSH security.", "ubuntu"),
				v("CVE-2024-32465", "git", "high", "1:2.25.1-1ubuntu3.11", "< 1:2.25.1-1ubuntu3.13", "1:2.25.1-1ubuntu3.13", "Path traversal in git archive command.", "ubuntu"),
				v("CVE-2023-25652", "git", "high", "1:2.25.1-1ubuntu3.11", "< 1:2.25.1-1ubuntu3.12", "1:2.25.1-1ubuntu3.12", "Crafted archive path exposes arbitrary file contents.", "ubuntu"),
				v("CVE-2023-38039", "curl", "medium", "7.68.0-1ubuntu2.20", "< 7.68.0-1ubuntu2.22", "7.68.0-1ubuntu2.22", "Memory exhaustion via excessive HTTP headers.", "ubuntu"),
				v("CVE-2024-28085", "util-linux", "medium", "2.34-0.1ubuntu9.4", "< 2.34-0.1ubuntu9.6", "2.34-0.1ubuntu9.6", "Privilege escalation via wall command.", "ubuntu"),
				v("CVE-2023-4641", "sudo", "medium", "1.8.31-1ubuntu1.5", "< 1.8.31-1ubuntu1.6", "1.8.31-1ubuntu1.6", "Memory disclosure via sudo command plugins.", "ubuntu"),
				v("CVE-2024-6197", "curl", "medium", "7.68.0-1ubuntu2.20", "< 7.68.0-1ubuntu2.23", "7.68.0-1ubuntu2.23", "ASN.1 integer overflow in TLS certificate parsing.", "ubuntu"),
				v("CVE-2023-46218", "curl", "low", "7.68.0-1ubuntu2.20", "< 7.68.0-1ubuntu2.22", "7.68.0-1ubuntu2.22", "Cookie mixed case PSL bypass.", "ubuntu"),
				v("CVE-2024-9143", "openssl", "low", "1.1.1f-1ubuntu2.20", "< 1.1.1f-1ubuntu2.23", "1.1.1f-1ubuntu2.23", "Out-of-bounds memory access in EC key operations.", "ubuntu"),
			},
		},
	}
}

// ── Scan constructors ─────────────────────────────────────────────────────────

func pgSCA(workspace Workspace, projectName, targetFile string, createdAt time.Time, deps []Dependency, vulns []Vulnerability) Scan {
	if deps == nil {
		deps = []Dependency{}
	}
	if vulns == nil {
		vulns = []Vulnerability{}
	}
	return Scan{
		ID: bson.NewObjectID(), Type: "sca",
		WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		ProjectName: projectName, Source: playgroundScanSource,
		TargetFile: targetFile, Status: "completed",
		ScannerVersion: playgroundScannerVersion,
		Summary:        buildSummary(deps, vulns),
		Dependencies:   deps, Vulnerabilities: vulns,
		CreatedAt: createdAt,
	}
}

func pgSAST(workspace Workspace, targetName string, createdAt time.Time, filesScanned int, findings []Finding) Scan {
	if findings == nil {
		findings = []Finding{}
	}
	return Scan{
		ID: bson.NewObjectID(), Type: "sast",
		WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		ProjectName: targetName, TargetName: targetName,
		Source: playgroundScanSource, Status: "completed",
		ScannerVersion: playgroundScannerVersion,
		FilesScanned:   filesScanned,
		Summary:        buildFindingSummary(filesScanned, findings),
		Findings:       findings, Vulnerabilities: []Vulnerability{},
		CreatedAt: createdAt,
	}
}

func pgContainer(workspace Workspace, imageName string, createdAt time.Time, osName, osVersion, osCodename, packageManager string, packages []Package, vulns []Vulnerability) Scan {
	if packages == nil {
		packages = []Package{}
	}
	if vulns == nil {
		vulns = []Vulnerability{}
	}
	return Scan{
		ID: bson.NewObjectID(), Type: "container",
		WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		ProjectName: imageName, TargetName: imageName,
		ImageName: imageName, ImageRef: imageName,
		Source: playgroundScanSource, Status: "completed",
		OSID: "linux", OSName: osName, OSVersion: osVersion, OSCodename: osCodename,
		PackageManager: packageManager, ScannerVersion: playgroundScannerVersion,
		Summary:  buildPackageSummary(packages, vulns),
		Packages: packages, Vulnerabilities: vulns,
		CreatedAt: createdAt,
	}
}

func pgHost(workspace Workspace, hostname string, createdAt time.Time, osName, osVersion, osCodename, packageManager string, packages []Package, vulns []Vulnerability) Scan {
	if packages == nil {
		packages = []Package{}
	}
	if vulns == nil {
		vulns = []Vulnerability{}
	}
	return Scan{
		ID: bson.NewObjectID(), Type: "host",
		WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		ProjectName: hostname, TargetName: hostname, Hostname: hostname,
		Source: playgroundScanSource, Status: "completed",
		OSID: "linux", OSName: osName, OSVersion: osVersion, OSCodename: osCodename,
		PackageManager: packageManager, ScannerVersion: playgroundScannerVersion,
		Summary:  buildPackageSummary(packages, vulns),
		Packages: packages, Vulnerabilities: vulns,
		CreatedAt: createdAt,
	}
}

func pgK8s(workspace Workspace, targetName string, createdAt time.Time, resourcesScanned int, findings []Finding) Scan {
	if findings == nil {
		findings = []Finding{}
	}
	return Scan{
		ID: bson.NewObjectID(), Type: "k8s",
		WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		ProjectName: targetName, TargetName: targetName,
		Source: playgroundScanSource, Status: "completed",
		ScannerVersion:   playgroundScannerVersion,
		ResourcesScanned: resourcesScanned,
		Summary:          buildFindingSummary(resourcesScanned, findings),
		Findings:         findings, Vulnerabilities: []Vulnerability{},
		CreatedAt: createdAt,
	}
}

func pgDep(name, requestedRange, resolvedVersion, ecosystem string) Dependency {
	return Dependency{Name: name, RequestedRange: requestedRange, ResolvedVersion: resolvedVersion, Scope: "prod", Ecosystem: ecosystem}
}

func pgPkg(name, version, manager string) Package {
	return Package{Name: name, Version: version, Manager: manager}
}

func pgVuln(id, packageName, severity, installedVersion, vulnerableRange, firstPatchedVersion, summary, ecosystem string) Vulnerability {
	return Vulnerability{
		ID: id, PackageName: packageName, InstalledPackage: packageName,
		Ecosystem: ecosystem, InstalledVersion: installedVersion,
		VulnerableRange: vulnerableRange, FirstPatchedVersion: firstPatchedVersion,
		Severity: severity, Summary: summary, AdvisoryURL: "https://runtz.dev/home",
	}
}

func pgFinding(id, title, severity, category, file string, line int, resourceKind, resourceName, namespace, description, remediation string) Finding {
	finding := Finding{
		ID:           id,
		Title:        title,
		Description:  description,
		Severity:     severity,
		Category:     category,
		File:         file,
		Line:         line,
		ResourceKind: resourceKind,
		ResourceName: resourceName,
		Namespace:    namespace,
		Remediation:  remediation,
	}
	if line > 0 {
		finding.Column = 1
	}
	return finding
}

// Legacy stubs.
func playgroundDependency(name, requestedRange, resolvedVersion string) Dependency {
	return pgDep(name, requestedRange, resolvedVersion, "npm")
}
func playgroundPackage(name, version string) Package { return pgPkg(name, version, "dpkg") }
func playgroundVulnerability(id, packageName, severity, installedVersion, vulnerableRange, firstPatchedVersion, summary string) Vulnerability {
	return pgVuln(id, packageName, severity, installedVersion, vulnerableRange, firstPatchedVersion, summary, "demo")
}
