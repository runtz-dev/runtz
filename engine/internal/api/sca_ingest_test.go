package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The CLI's Dependency type (runtz-cli/internal/sca.Dependency) always sends
// a per-dependency "file" field. decodeJSON uses DisallowUnknownFields, so
// the engine's Dependency type must mirror every field the CLI sends or
// every SCA scan is rejected with 400 "invalid json body" — see the incident
// where rc12 broke `runtz sca` entirely. This mirrors the CLI's wire shape
// (it lives in a separate repo/module, so it can't be imported directly).
type cliDependency struct {
	Name            string `json:"name"`
	RequestedRange  string `json:"requestedRange"`
	ResolvedVersion string `json:"resolvedVersion"`
	Scope           string `json:"scope"`
	Ecosystem       string `json:"ecosystem"`
	File            string `json:"file,omitempty"`
}

func TestDecodeJSONAcceptsSCAPayloadWithDependencyFile(t *testing.T) {
	payload := struct {
		ProjectName     string          `json:"projectName"`
		Source          string          `json:"source"`
		TargetFile      string          `json:"targetFile"`
		ScannerVersion  string          `json:"scannerVersion"`
		Dependencies    []cliDependency `json:"dependencies"`
		Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	}{
		ProjectName:    "runtz-mcp",
		Source:         "/home/user/runtz-mcp",
		TargetFile:     "go.mod",
		ScannerVersion: "1.0.0-rc8",
		Dependencies: []cliDependency{
			{
				Name:            "github.com/example/pkg",
				RequestedRange:  "v1.2.3",
				ResolvedVersion: "v1.2.3",
				Scope:           "direct",
				Ecosystem:       "go",
				File:            "go.mod",
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/sca", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	var target struct {
		ProjectName     string          `json:"projectName"`
		Source          string          `json:"source"`
		TargetFile      string          `json:"targetFile"`
		ScannerVersion  string          `json:"scannerVersion"`
		Dependencies    []Dependency    `json:"dependencies"`
		Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	}
	if !decodeJSON(recorder, request, &target) {
		t.Fatalf("decodeJSON rejected a real SCA payload: %s", recorder.Body.String())
	}
	if got, want := target.Dependencies[0].File, "go.mod"; got != want {
		t.Fatalf("Dependencies[0].File = %q, want %q", got, want)
	}
}

func TestDecodeJSONStillRejectsUnknownFields(t *testing.T) {
	body := []byte(`{"projectName":"demo","dependencies":[{"name":"pkg","notARealField":true}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/sca", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	var target struct {
		ProjectName  string       `json:"projectName"`
		Dependencies []Dependency `json:"dependencies"`
	}
	if decodeJSON(recorder, request, &target) {
		t.Fatal("decodeJSON accepted a genuinely unknown field; DisallowUnknownFields should still guard the ingest endpoints")
	}
}
