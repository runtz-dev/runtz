package securityclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
)

var azureIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidAzureID(id string) bool { return azureIDPattern.MatchString(id) }

// AzureCredentials are forwarded to Security; the engine never persists the secret.
type AzureCredentials struct {
	TenantID     string `json:"tenantId"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

func ParseAzureCredentials(raw []byte) (AzureCredentials, error) {
	var c AzureCredentials
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32768 || decoder.Decode(&c) != nil || decoder.Decode(new(any)) != io.EOF || !ValidAzureID(c.TenantID) || !ValidAzureID(c.ClientID) || strings.TrimSpace(c.ClientSecret) == "" || len(c.ClientSecret) > 4096 {
		return AzureCredentials{}, errors.New("provide Azure tenantId, clientId (UUIDs) and clientSecret")
	}
	c.TenantID, c.ClientID = strings.ToLower(c.TenantID), strings.ToLower(c.ClientID)
	return c, nil
}
