package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/cloudgcp"
	"github.com/runtz-dev/runtz/engine/internal/securityclient"
)

func (s *Server) decodeAzureConnect(w http.ResponseWriter, r *http.Request) (cloudConnectRequest, securityclient.AzureCredentials, bool) {
	request, ok := s.decodeCloudRequest(w, r)
	if !ok {
		return request, securityclient.AzureCredentials{}, false
	}
	creds, err := securityclient.ParseAzureCredentials(request.Credentials)
	if err != nil {
		writeError(w, 400, err.Error())
		return request, creds, false
	}
	request.SubscriptionID = strings.ToLower(strings.TrimSpace(request.SubscriptionID))
	if !securityclient.ValidAzureID(request.SubscriptionID) || request.ProjectID != "" || request.AccountID != "" || (request.Location != "" && request.Location != "global") {
		writeError(w, 400, "provide an Azure subscription ID (UUID); Azure checks run at subscription scope")
		return request, creds, false
	}
	return request, creds, true
}

func (s *Server) handleValidateAzure(w http.ResponseWriter, r *http.Request) {
	request, creds, ok := s.decodeAzureConnect(w, r)
	if !ok {
		return
	}
	defer clear(request.Credentials)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.security.ValidateAzure(ctx, request.WorkspaceID, request.SubscriptionID, request.Credentials); err != nil {
		writeCloudServiceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"valid": true, "subscriptionId": request.SubscriptionID, "directoryTenantId": creds.TenantID, "clientId": creds.ClientID})
}

func (s *Server) handleConnectAzure(w http.ResponseWriter, r *http.Request) {
	request, creds, ok := s.decodeAzureConnect(w, r)
	if !ok {
		return
	}
	defer clear(request.Credentials)
	// Security and the existing unique index use project_id as the shared scope
	// key. Keep subscriptionId explicit for Azure API consumers and metadata.
	s.connectCloud(w, r, request, cloudConnection{Provider: "azure", ProjectID: request.SubscriptionID, SubscriptionID: request.SubscriptionID, DirectoryTenantID: creds.TenantID, ClientID: creds.ClientID})
}

func cloudCredentialIdentity(c cloudConnection, raw json.RawMessage) (string, error) {
	if c.Provider == "aws" {
		creds, err := securityclient.ParseAWSCredentials(raw)
		if err != nil {
			return "", err
		}
		if securityclient.AWSAuthMethod(c.AuthMethod) != securityclient.AWSAuthAccessKey || creds.AuthMethod != securityclient.AWSAuthAccessKey || !securityclient.SameRegions(c.Regions, creds.Regions) {
			return "", errors.New("AWS key rotation must preserve access key authentication, account and regions; reconnect to change scope")
		}
		return "", nil
	}
	if c.Provider == "azure" {
		creds, err := securityclient.ParseAzureCredentials(raw)
		if err != nil {
			return "", err
		}
		if creds.TenantID != c.DirectoryTenantID || creds.ClientID != c.ClientID {
			return "", errors.New("credential replacement must preserve the Azure tenant and application identity")
		}
		return "", nil
	}
	creds, err := cloudgcp.ParseCredentials(raw)
	return creds.ClientEmail, err
}

func matchesCloudResult(c cloudConnection, result *securityclient.Result) bool {
	if result == nil || result.Provider != c.Provider || result.ScopeID() != c.ProjectID {
		return false
	}
	if c.Provider != "aws" {
		return true
	}
	if !securityclient.SameRegions(c.Regions, result.Regions) {
		return false
	}
	inScope := func(region string) bool {
		if region == "global" {
			return true
		}
		for _, r := range c.Regions {
			if region == r {
				return true
			}
		}
		return false
	}
	for _, f := range result.Findings {
		if f.AccountID != c.AccountID || !inScope(f.Region) {
			return false
		}
	}
	for _, check := range result.Checks {
		if !inScope(check.Region) {
			return false
		}
	}
	return true
}
