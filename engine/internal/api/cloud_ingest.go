package api

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/cloudgcp"
	"github.com/runtz-dev/runtz/engine/internal/securityclient"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var cliAWSRegionPattern = regexp.MustCompile(`^(af|ap|ca|eu|il|me|mx|sa|us)-(central|east|west|north|south|northeast|southeast)-[1-9][0-9]?$`)

// validateCLICloudResult validates the wire contract and derives counts from
// checks/findings. Client-supplied totals are never trusted.
func validateCLICloudResult(result *securityclient.Result) error {
	scope := result.ScopeID()
	switch result.Provider {
	case "gcp":
		if !cloudgcp.ValidProject(scope) || result.AccountID != "" || result.SubscriptionID != "" || len(result.Regions) != 0 {
			return errors.New("invalid GCP scope")
		}
	case "aws":
		if !securityclient.ValidAWSAccount(scope) || result.ProjectID != "" || result.SubscriptionID != "" || len(result.Regions) == 0 || len(result.Regions) > 32 {
			return errors.New("invalid AWS scope")
		}
		seen := map[string]bool{}
		for _, region := range result.Regions {
			if !cliAWSRegionPattern.MatchString(region) || seen[region] {
				return errors.New("invalid AWS regions")
			}
			seen[region] = true
		}
	case "azure":
		if !securityclient.ValidAzureID(scope) || result.ProjectID != "" || result.AccountID != "" || len(result.Regions) != 0 {
			return errors.New("invalid Azure scope")
		}
	default:
		return errors.New("provider must be gcp, aws or azure")
	}
	if result.StartedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) || result.FinishedAt.After(time.Now().Add(5*time.Minute)) || len(result.Checks) == 0 || len(result.Checks) > 10000 || len(result.Findings) > 10000 || result.CatalogVersion == "" || len(result.CatalogVersion) > 128 {
		return errors.New("provide a completed scan with its catalog version, timestamps and 1–10,000 checks")
	}
	result.Summary = securityclient.Summary{}
	knownFailures := map[string]bool{}
	prefix := "RZ-" + strings.ToUpper(result.Provider) + "-"
	validSeverity := func(s string) bool { return s == "critical" || s == "high" || s == "medium" || s == "low" }
	validRegion := func(s string) bool {
		if result.Provider != "aws" {
			return s == ""
		}
		if s == "global" {
			return true
		}
		for _, region := range result.Regions {
			if region == s {
				return true
			}
		}
		return false
	}
	for _, check := range result.Checks {
		if !strings.HasPrefix(check.RuleID, prefix) || len(check.RuleID) > 128 || !validSeverity(check.Severity) || !validRegion(check.Region) || check.Title == "" || len(check.Title) > 1024 || check.Resource == "" || len(check.Resource) > 4096 || len(check.Evidence) > 8192 {
			return errors.New("invalid check identity, region or evidence")
		}
		switch check.Status {
		case "pass":
			result.Summary.Passed++
		case "fail":
			result.Summary.Failed++
			knownFailures[check.RuleID+"\x00"+check.Resource+"\x00"+check.Region+"\x00"+check.Severity] = true
		case "error":
			result.Summary.Errors++
		case "skipped":
			result.Summary.Skipped++
		case "not_applicable":
			result.Summary.NotApplicable++
		default:
			return errors.New("unknown check status")
		}
	}
	seenFindings := map[string]bool{}
	seenFailureKeys := map[string]bool{}
	for _, f := range result.Findings {
		key := f.RuleID + "\x00" + f.ResourceName + "\x00" + f.Region + "\x00" + f.Severity
		if !knownFailures[key] || seenFailureKeys[key] || f.ID == "" || len(f.ID) > 256 || seenFindings[f.ID] || f.Title == "" || len(f.Title) > 1024 || len(f.Evidence) > 8192 || len(f.Description) > 16384 || len(f.Remediation) > 16384 || len(f.Documentation) > 4096 || len(f.Source) > 1024 || len(f.Category) > 256 || (f.ProjectID != result.ProjectID) || (f.AccountID != result.AccountID) || (f.SubscriptionID != result.SubscriptionID) {
			return errors.New("finding does not match a failed check in this scope")
		}
		seenFindings[f.ID] = true
		seenFailureKeys[key] = true
		switch f.Severity {
		case "critical":
			result.Summary.Critical++
		case "high":
			result.Summary.High++
		case "medium":
			result.Summary.Medium++
		case "low":
			result.Summary.Low++
		}
	}
	if len(seenFailureKeys) != len(knownFailures) {
		return errors.New("each failed check must have a finding")
	}
	return nil
}

func (s *Server) handleIngestCloud(w http.ResponseWriter, r *http.Request) {
	workspace, ok := s.authenticateIngest(w, r)
	if !ok {
		return
	}
	if workspace.Kind == "playground" {
		writeError(w, 403, "cloud scanning is unavailable in playground workspaces")
		return
	}
	var request struct {
		Name   string                `json:"name"`
		Result securityclient.Result `json:"result"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024*1024)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, 400, "invalid cloud scan JSON body")
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 80 {
		writeError(w, 400, "enter a connection name of up to 80 characters")
		return
	}
	result := &request.Result
	result.StartedAt = result.StartedAt.UTC().Truncate(time.Millisecond)
	result.FinishedAt = result.FinishedAt.UTC().Truncate(time.Millisecond)
	if err := validateCLICloudResult(result); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	scope := result.ScopeID()
	now := time.Now().UTC()
	filter := bson.M{"workspace_id": workspace.ID, "provider": result.Provider, "project_id": scope, "location": "global"}
	c := cloudConnection{ID: bson.NewObjectID(), WorkspaceID: workspace.ID, CreatedBy: workspace.CreatedBy, Provider: result.Provider, Source: "cli", ProjectID: scope, AccountID: result.AccountID, SubscriptionID: result.SubscriptionID, Regions: result.Regions, Name: request.Name, Location: "global", Status: "waiting", CreatedAt: now, UpdatedAt: now}
	// Reserve once; concurrent first uploads converge on the existing scope.
	var existing cloudConnection
	err := s.cloudConnections.FindOneAndUpdate(r.Context(), filter, bson.M{"$setOnInsert": c}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&existing)
	if mongo.IsDuplicateKeyError(err) {
		err = s.cloudConnections.FindOne(r.Context(), filter).Decode(&existing)
	}
	if err != nil {
		writeError(w, 500, "could not reserve cloud scan")
		return
	}
	c = existing
	if c.Source != "cli" {
		writeError(w, 409, "this scope is connected through the site; disconnect it before switching to CLI scanning")
		return
	}
	if c.LastScanAt != nil && result.FinishedAt.Before(*c.LastScanAt) {
		writeError(w, 409, "a newer cloud scan is already available")
		return
	}
	raw, _ := json.Marshal(result)
	digest := sha256.Sum256(append([]byte(workspace.ID.Hex()+":"+c.ID.Hex()+":"), raw...))
	var scanID bson.ObjectID
	copy(scanID[:], digest[:12])
	if c.CLIScanID == scanID {
		writeJSON(w, 200, map[string]any{"id": c.ID.Hex(), "scanId": scanID.Hex(), "status": "stored"})
		return
	}
	counts := result.Summary
	// Persist an immutable scan in the normal usage ledger before publishing its
	// pointer. Retries have a deterministic ID and never charge a second scan.
	summary := ScanSummary{Vulnerabilities: len(result.Findings), Critical: counts.Critical, High: counts.High, Medium: counts.Medium, Low: counts.Low}
	_, err = s.scans.InsertOne(r.Context(), bson.M{"_id": scanID, "type": "cloud", "workspace_id": workspace.ID, "workspace_name": workspace.Name, "project_name": request.Name, "source": "cli", "scanner_version": result.CatalogVersion, "status": "completed", "summary": summary, "created_at": now, "cloud_connection_id": c.ID, "cloud_result": result})
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		writeError(w, 500, "could not store cloud result; retry the upload")
		return
	}
	status := "connected"
	if counts.Errors+counts.Skipped > 0 {
		status = "partial"
	}
	filter = bson.M{"_id": c.ID, "workspace_id": workspace.ID, "source": "cli", "$or": []bson.M{{"last_scan_at": bson.M{"$exists": false}}, {"last_scan_at": nil}, {"last_scan_at": bson.M{"$lte": result.FinishedAt}}}}
	update, err := s.cloudConnections.UpdateOne(r.Context(), filter, bson.M{"$set": bson.M{"cli_scan_id": scanID, "name": request.Name, "regions": result.Regions, "last_scan_at": result.FinishedAt, "coverage": counts, "status": status, "last_error": "", "summary": cloudCounts{Total: len(result.Findings), Critical: counts.Critical, High: counts.High, Medium: counts.Medium, Low: counts.Low}, "updated_at": now}})
	if err != nil {
		writeError(w, 500, "scan stored; retry to publish its results")
		return
	}
	if update.MatchedCount != 1 {
		writeError(w, 409, "connection was removed or a newer scan arrived")
		return
	}
	writeJSON(w, 201, map[string]any{"id": c.ID.Hex(), "scanId": scanID.Hex(), "status": "stored", "summary": counts})
}

func (s *Server) latestCloudResult(r *http.Request, c cloudConnection) (securityclient.Job, error) {
	if c.Source != "cli" {
		return s.security.Latest(r.Context(), c.WorkspaceID.Hex(), c.SecurityConnectionID)
	}
	var record struct {
		Result *securityclient.Result `bson:"cloud_result"`
	}
	if c.CLIScanID.IsZero() {
		return securityclient.Job{}, &securityclient.Error{Status: 404, Message: "waiting for CLI scan"}
	}
	err := s.scans.FindOne(r.Context(), bson.M{"_id": c.CLIScanID, "workspace_id": c.WorkspaceID, "cloud_connection_id": c.ID, "type": "cloud"}).Decode(&record)
	if err != nil {
		return securityclient.Job{}, fmt.Errorf("could not read CLI cloud results")
	}
	return securityclient.Job{ID: c.CLIScanID.Hex(), Result: record.Result}, nil
}
