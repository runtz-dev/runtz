package api

import (
	"context"
	"github.com/runtz-dev/runtz/engine/internal/securityclient"
	"net/http"
	"time"
)

func (s *Server) handleAWSOnboarding(w http.ResponseWriter, r *http.Request) {
	request, ok := s.decodeCloudRequest(w, r)
	if !ok {
		return
	}
	if !securityclient.ValidAWSAccount(request.AccountID) || request.ProjectID != "" || request.SubscriptionID != "" {
		writeError(w, 400, "provide a 12-digit AWS account ID")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	setup, err := s.security.AWSOnboarding(ctx, request.WorkspaceID, request.AccountID, request.AuthMethod)
	if err != nil {
		writeCloudServiceError(w, err)
		return
	}
	writeJSON(w, 200, setup)
}
func (s *Server) decodeAWSConnect(w http.ResponseWriter, r *http.Request) (cloudConnectRequest, securityclient.AWSCredentials, bool) {
	request, ok := s.decodeCloudRequest(w, r)
	if !ok {
		return request, securityclient.AWSCredentials{}, false
	}
	creds, err := securityclient.ParseAWSCredentials(request.Credentials)
	if err != nil {
		writeError(w, 400, err.Error())
		return request, creds, false
	}
	if !securityclient.ValidAWSAccount(request.AccountID) || (creds.AuthMethod == securityclient.AWSAuthRole && securityclient.AWSRoleAccount(creds.RoleARN) != request.AccountID) || request.ProjectID != "" || request.SubscriptionID != "" || (request.Location != "" && request.Location != "global") {
		writeError(w, 400, "provide the AWS account ID and credentials for that account; omit other provider scopes")
		return request, creds, false
	}
	return request, creds, true
}
func (s *Server) handleValidateAWS(w http.ResponseWriter, r *http.Request) {
	request, _, ok := s.decodeAWSConnect(w, r)
	if !ok {
		return
	}
	defer clear(request.Credentials)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.security.ValidateAWS(ctx, request.WorkspaceID, request.AccountID, request.Credentials); err != nil {
		writeCloudServiceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"valid": true, "accountId": request.AccountID})
}
func (s *Server) handleConnectAWS(w http.ResponseWriter, r *http.Request) {
	request, creds, ok := s.decodeAWSConnect(w, r)
	if !ok {
		return
	}
	defer clear(request.Credentials)
	s.connectCloud(w, r, request, cloudConnection{Provider: "aws", AuthMethod: creds.AuthMethod, ProjectID: request.AccountID, AccountID: request.AccountID, RoleARN: creds.RoleARN, Regions: creds.Regions})
}
