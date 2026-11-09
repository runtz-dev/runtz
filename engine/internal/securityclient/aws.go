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
	AuthMethod      string   `json:"authMethod,omitempty"`
	RoleARN         string   `json:"roleArn,omitempty"`
	ExternalID      string   `json:"externalId,omitempty"`
	AccessKeyID     string   `json:"accessKeyId,omitempty"`
	SecretAccessKey string   `json:"secretAccessKey,omitempty"`
	Regions         []string `json:"regions"`
}

const AWSAuthRole = "role"
const AWSAuthAccessKey = "access_key"

func AWSAuthMethod(method string) string {
	if method == "" {
		return AWSAuthRole
	}
	return method
}

var awsRolePattern = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):role/[a-zA-Z0-9+=,.@_/-]{1,512}$`)
var awsAccessKeyPattern = regexp.MustCompile(`^AKIA[A-Z0-9]{16}$`)
var awsSecretKeyPattern = regexp.MustCompile(`^[A-Za-z0-9/+=]{40}$`)
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
	if len(raw) > 8192 || d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF || len(c.Regions) == 0 || len(c.Regions) > 32 {
		return c, errors.New("provide one AWS authentication method and 1–32 commercial regions")
	}
	c.AuthMethod = AWSAuthMethod(c.AuthMethod)
	switch c.AuthMethod {
	case AWSAuthRole:
		if AWSRoleAccount(c.RoleARN) == "" || !awsExternalPattern.MatchString(c.ExternalID) || c.AccessKeyID != "" || c.SecretAccessKey != "" {
			return c, errors.New("role authentication requires an AWS role ARN and Runtz external ID, without access keys")
		}
	case AWSAuthAccessKey:
		if !awsAccessKeyPattern.MatchString(c.AccessKeyID) || !awsSecretKeyPattern.MatchString(c.SecretAccessKey) || c.RoleARN != "" || c.ExternalID != "" {
			return c, errors.New("access key authentication requires an IAM user access key ID and secret access key, without role settings")
		}
	default:
		return c, errors.New("AWS authMethod must be role or access_key")
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
	AuthMethod     string          `json:"authMethod"`
	AccountID      string          `json:"accountId"`
	PrincipalARN   string          `json:"principalArn"`
	ExternalID     string          `json:"externalId"`
	RoleARN        string          `json:"roleArn"`
	RoleName       string          `json:"roleName"`
	Template       json.RawMessage `json:"template"`
	Policy         json.RawMessage `json:"policy"`
	QuickCreateURL string          `json:"quickCreateUrl,omitempty"`
}

func (c *Client) AWSOnboarding(ctx context.Context, tenant, account, method string) (AWSSetup, error) {
	var out AWSSetup
	method = AWSAuthMethod(method)
	if method != AWSAuthRole && method != AWSAuthAccessKey {
		return out, &Error{400, "AWS authMethod must be role or access_key"}
	}
	err := c.request(ctx, tenant, "POST", "/v1/aws/onboarding", map[string]any{"accountId": account, "authMethod": method}, &out)
	valid := out.AccountID == account && AWSAuthMethod(out.AuthMethod) == method
	if method == AWSAuthAccessKey {
		valid = valid && json.Valid(out.Policy) && string(out.Policy) != "null" && out.RoleARN == "" && out.ExternalID == "" && out.PrincipalARN == ""
	} else {
		valid = valid && AWSRoleAccount(out.RoleARN) == account && AWSRoleAccount(out.PrincipalARN) != "" && awsExternalPattern.MatchString(out.ExternalID) && json.Valid(out.Template)
	}
	if err == nil && !valid {
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
		actual := out.Connection
		if e != nil || AWSAuthMethod(actual.AuthMethod) != expected.AuthMethod || actual.RoleARN != expected.RoleARN || !SameRegions(actual.Regions, expected.Regions) {
			return out, &Error{502, "Runtz Security returned a different AWS authentication method, role or region scope"}
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
