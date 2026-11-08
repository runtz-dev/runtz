package securityclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
)

type AWSCredentials struct {
	RoleARN    string   `json:"roleArn"`
	ExternalID string   `json:"externalId"`
	Regions    []string `json:"regions"`
}

var awsRolePattern = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):role/[a-zA-Z0-9+=,.@_/-]{1,512}$`)
var awsAccountPattern = regexp.MustCompile(`^[0-9]{12}$`)
var awsExternalPattern = regexp.MustCompile(`^runtz-[a-f0-9]{64}$`)
var awsRegionPattern = regexp.MustCompile(`^(af|ap|ca|eu|il|me|mx|sa|us)-(central|east|west|north|south|northeast|southeast)-[1-9][0-9]?$`)

func ValidAWSAccount(id string) bool { return awsAccountPattern.MatchString(id) }
func AWSRoleAccount(arn string) string {
	m := awsRolePattern.FindStringSubmatch(arn)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}
func ParseAWSCredentials(raw []byte) (AWSCredentials, error) {
	var c AWSCredentials
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if len(raw) > 8192 || d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF || AWSRoleAccount(c.RoleARN) == "" || !awsExternalPattern.MatchString(c.ExternalID) || len(c.Regions) == 0 || len(c.Regions) > 32 {
		return c, errors.New("provide an AWS role ARN, Runtz external ID and 1–32 commercial AWS regions")
	}
	seen := map[string]bool{}
	for _, r := range c.Regions {
		if !awsRegionPattern.MatchString(r) || seen[r] {
			return c, errors.New("AWS regions must be valid and unique")
		}
		seen[r] = true
	}
	sort.Strings(c.Regions)
	return c, nil
}

type AWSSetup struct {
	AccountID      string          `json:"accountId"`
	PrincipalARN   string          `json:"principalArn"`
	ExternalID     string          `json:"externalId"`
	RoleARN        string          `json:"roleArn"`
	RoleName       string          `json:"roleName"`
	Template       json.RawMessage `json:"template"`
	QuickCreateURL string          `json:"quickCreateUrl,omitempty"`
}

func (c *Client) AWSOnboarding(ctx context.Context, tenant, account string) (AWSSetup, error) {
	var out AWSSetup
	err := c.request(ctx, tenant, "POST", "/v1/aws/onboarding", map[string]any{"accountId": account}, &out)
	if err == nil && (out.AccountID != account || AWSRoleAccount(out.RoleARN) != account || AWSRoleAccount(out.PrincipalARN) == "" || !awsExternalPattern.MatchString(out.ExternalID) || !json.Valid(out.Template)) {
		err = &Error{502, "Runtz Security returned invalid AWS onboarding"}
	}
	return out, err
}
func (c *Client) ValidateAWS(ctx context.Context, tenant, account string, credential json.RawMessage) error {
	var out struct {
		Valid     bool   `json:"valid"`
		AccountID string `json:"accountId"`
	}
	err := c.request(ctx, tenant, "POST", "/v1/aws/validate", map[string]any{"accountId": account, "credentials": credential}, &out)
	if err == nil && (!out.Valid || out.AccountID != account) {
		return &Error{502, "Runtz Security did not verify the requested AWS account"}
	}
	return err
}
func (c *Client) CreateAWS(ctx context.Context, tenant, name, account string, credential json.RawMessage) (CreateResponse, error) {
	out, err := c.create(ctx, tenant, name, "aws", account, credential)
	if err == nil {
		expected, e := ParseAWSCredentials(credential)
		actual := AWSCredentials{RoleARN: out.Connection.RoleARN, Regions: out.Connection.Regions}
		if e != nil || actual.RoleARN != expected.RoleARN || !SameRegions(actual.Regions, expected.Regions) {
			return out, &Error{502, "Runtz Security returned a different AWS role or region scope"}
		}
	}
	return out, err
}
func SameRegions(a, b []string) bool {
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
