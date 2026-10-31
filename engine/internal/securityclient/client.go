// Package securityclient calls the trusted Runtz Security backend. Workspace
// authorization stays in the engine; tenant headers are never copied from users.
package securityclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	base, key string
	http      *http.Client
}
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

func New(endpoint, key string) *Client {
	return &Client{base: strings.TrimRight(endpoint, "/"), key: key, http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) Configured() bool { return c != nil && c.base != "" && c.key != "" }
func (c *Client) request(ctx context.Context, tenant, method, path string, body, out any) error {
	if !c.Configured() {
		return &Error{503, "Runtz Security is not configured; ask your administrator to configure the service URL and API key"}
	}
	if !idPattern.MatchString(tenant) {
		return &Error{400, "invalid cloud workspace"}
	}
	var input io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return errors.New("could not encode cloud request")
		}
		defer clear(raw)
		input = bytes.NewReader(raw)
	}
	r, err := http.NewRequestWithContext(ctx, method, c.base+path, input)
	if err != nil {
		return &Error{503, "Runtz Security endpoint is invalid"}
	}
	r.Header.Set("Authorization", "Bearer "+c.key)
	r.Header.Set("X-Runtz-Tenant-ID", tenant)
	r.Header.Set("User-Agent", "runtz-engine/1.0")
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(r)
	if err != nil {
		return &Error{503, "Could not reach Runtz Security; previous results are preserved"}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 10*1024*1024+1))
	defer clear(raw)
	if err != nil || len(raw) > 10*1024*1024 {
		return &Error{502, "Runtz Security response exceeded the allowed size or could not be read"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := "Runtz Security request failed"
		var failure struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &failure) == nil && failure.Error != "" && len(failure.Error) <= 1024 {
			message = failure.Error
		}
		if response.StatusCode == 401 || response.StatusCode == 403 {
			message = "Runtz Security authentication failed; ask your administrator to check the engine service key"
		}
		if out != nil {
			_ = json.Unmarshal(raw, out)
		}
		return &Error{response.StatusCode, message}
	}
	if out != nil && json.Unmarshal(raw, out) != nil {
		return &Error{502, "Runtz Security returned an invalid response"}
	}
	return nil
}

type Summary struct {
	Passed        int `json:"passed"`
	Failed        int `json:"failed"`
	Errors        int `json:"errors"`
	Skipped       int `json:"skipped"`
	NotApplicable int `json:"notApplicable"`
	Critical      int `json:"critical"`
	High          int `json:"high"`
	Medium        int `json:"medium"`
	Low           int `json:"low"`
}
type Check struct {
	RuleID   string `json:"ruleId"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Resource string `json:"resource"`
	Evidence string `json:"evidence"`
}
type Finding struct {
	ID            string `json:"id"`
	RuleID        string `json:"ruleId"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Severity      string `json:"severity"`
	Category      string `json:"category"`
	ResourceName  string `json:"resourceName"`
	ProjectID     string `json:"projectId"`
	Source        string `json:"source"`
	Evidence      string `json:"evidence"`
	Remediation   string `json:"remediation"`
	Documentation string `json:"documentation"`
}
type Result struct {
	Provider       string    `json:"provider"`
	ProjectID      string    `json:"projectId"`
	CatalogVersion string    `json:"catalogVersion"`
	StartedAt      time.Time `json:"startedAt"`
	FinishedAt     time.Time `json:"finishedAt"`
	Summary        Summary   `json:"summary"`
	Checks         []Check   `json:"checks"`
	Findings       []Finding `json:"findings"`
}
type Connection struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenantId"`
	Provider       string `json:"provider"`
	Name           string `json:"name"`
	ProjectID      string `json:"projectId"`
	ServiceAccount string `json:"serviceAccount"`
	LastScanID     string `json:"lastScanId"`
	LastAttemptID  string `json:"lastAttemptId"`
}
type Job struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenantId"`
	ConnectionID string     `json:"connectionId"`
	State        string     `json:"state"`
	Error        string     `json:"error"`
	CreatedAt    time.Time  `json:"createdAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	Result       *Result    `json:"result"`
}
type CreateResponse struct {
	Connection Connection `json:"connection"`
	Scan       Job        `json:"scan"`
}

func validConnection(tenant, project string, c Connection) bool {
	return idPattern.MatchString(c.ID) && c.TenantID == tenant && c.ProjectID == project && c.Provider == "gcp"
}
func (c *Client) Validate(ctx context.Context, tenant, project string, credential json.RawMessage) error {
	var out struct {
		Valid   bool `json:"valid"`
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	err := c.request(ctx, tenant, "POST", "/v1/gcp/validate", map[string]any{"projectId": project, "credentials": credential}, &out)
	if err == nil && (!out.Valid || out.Project.ID != project) {
		return &Error{502, "Runtz Security did not verify the requested project"}
	}
	return err
}
func (c *Client) Connections(ctx context.Context, tenant string) ([]Connection, error) {
	var out struct {
		Connections []Connection `json:"connections"`
	}
	err := c.request(ctx, tenant, "GET", "/v1/connections", nil, &out)
	for _, conn := range out.Connections {
		if !validConnection(tenant, conn.ProjectID, conn) {
			return nil, &Error{502, "Runtz Security returned an invalid connection"}
		}
	}
	return out.Connections, err
}
func (c *Client) Create(ctx context.Context, tenant, name, project string, credential json.RawMessage) (CreateResponse, error) {
	var out CreateResponse
	err := c.request(ctx, tenant, "POST", "/v1/connections", map[string]any{"name": name, "projectId": project, "credentials": credential}, &out)
	if IsStatus(err, 409) {
		// Recover a connection left by a lost response or an interrupted engine
		// write. The exact workspace/project and the new key are verified first.
		rows, listErr := c.Connections(ctx, tenant)
		if listErr != nil {
			return out, listErr
		}
		for _, conn := range rows {
			if conn.ProjectID == project {
				if err = c.Rotate(ctx, tenant, conn.ID, credential); err != nil {
					return out, err
				}
				job, err := c.Enqueue(ctx, tenant, conn.ID)
				return CreateResponse{Connection: conn, Scan: job}, err
			}
		}
	}
	// A 503 with a saved connection means the scheduler can still enqueue its
	// initial scan. Preserve the reference so the owner can retry manually.
	if err != nil && !(IsStatus(err, 503) && validConnection(tenant, project, out.Connection)) {
		return out, err
	}
	if !validConnection(tenant, project, out.Connection) {
		return out, &Error{502, "Runtz Security returned an invalid connection"}
	}
	return out, nil
}
func connectionPath(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", &Error{400, "invalid security connection reference"}
	}
	return "/v1/connections/" + id, nil
}
func (c *Client) Connection(ctx context.Context, tenant, id string) (Connection, error) {
	path, err := connectionPath(id)
	if err != nil {
		return Connection{}, err
	}
	var out struct {
		Connection Connection `json:"connection"`
	}
	err = c.request(ctx, tenant, "GET", path, nil, &out)
	if err == nil && (out.Connection.ID != id || !validConnection(tenant, out.Connection.ProjectID, out.Connection)) {
		err = &Error{502, "Runtz Security returned an invalid connection"}
	}
	return out.Connection, err
}
func (c *Client) Enqueue(ctx context.Context, tenant, id string) (Job, error) {
	path, err := connectionPath(id)
	if err != nil {
		return Job{}, err
	}
	var out struct {
		Scan Job `json:"scan"`
	}
	err = c.request(ctx, tenant, "POST", path+"/scans", nil, &out)
	if err == nil && (out.Scan.ConnectionID != id || out.Scan.TenantID != tenant || !idPattern.MatchString(out.Scan.ID)) {
		err = &Error{502, "Runtz Security returned an invalid scan"}
	}
	return out.Scan, err
}
func (c *Client) Job(ctx context.Context, tenant, id string) (Job, error) {
	if !idPattern.MatchString(id) {
		return Job{}, &Error{400, "invalid security scan reference"}
	}
	// Decode only metadata here; finding arrays are fetched by Latest when the
	// user opens results. Compatible with existing Security API deployments.
	var out struct {
		Scan struct {
			ID           string     `json:"id"`
			TenantID     string     `json:"tenantId"`
			ConnectionID string     `json:"connectionId"`
			State        string     `json:"state"`
			Error        string     `json:"error"`
			CreatedAt    time.Time  `json:"createdAt"`
			FinishedAt   *time.Time `json:"finishedAt"`
			Result       *struct {
				Provider       string    `json:"provider"`
				ProjectID      string    `json:"projectId"`
				CatalogVersion string    `json:"catalogVersion"`
				FinishedAt     time.Time `json:"finishedAt"`
				Summary        Summary   `json:"summary"`
			} `json:"result"`
		} `json:"scan"`
	}
	err := c.request(ctx, tenant, "GET", "/v1/scans/"+id+"?view=summary", nil, &out)
	j := out.Scan
	job := Job{ID: j.ID, TenantID: j.TenantID, ConnectionID: j.ConnectionID, State: j.State, Error: j.Error, CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt}
	if j.Result != nil {
		job.Result = &Result{Provider: j.Result.Provider, ProjectID: j.Result.ProjectID, CatalogVersion: j.Result.CatalogVersion, FinishedAt: j.Result.FinishedAt, Summary: j.Result.Summary}
	}
	if err == nil && (job.ID != id || job.TenantID != tenant) {
		err = &Error{502, "Runtz Security returned an invalid scan"}
	}
	return job, err
}
func (c *Client) Latest(ctx context.Context, tenant, id string) (Job, error) {
	path, err := connectionPath(id)
	if err != nil {
		return Job{}, err
	}
	var out struct {
		Scan Job `json:"scan"`
	}
	err = c.request(ctx, tenant, "GET", path+"/results", nil, &out)
	if err == nil && (out.Scan.TenantID != tenant || out.Scan.ConnectionID != id || out.Scan.Result == nil || out.Scan.Result.Provider != "gcp") {
		err = &Error{502, "Runtz Security returned an invalid result"}
	}
	return out.Scan, err
}
func (c *Client) Rotate(ctx context.Context, tenant, id string, credential json.RawMessage) error {
	path, err := connectionPath(id)
	if err != nil {
		return err
	}
	return c.request(ctx, tenant, "PATCH", path+"/credentials", map[string]any{"credentials": credential}, nil)
}
func (c *Client) Delete(ctx context.Context, tenant, id string) error {
	path, err := connectionPath(id)
	if err != nil {
		return err
	}
	err = c.request(ctx, tenant, "DELETE", path, nil, nil)
	if IsStatus(err, 404) {
		return nil
	}
	return err
}
func (c *Client) DeleteWorkspace(ctx context.Context, tenant string) error {
	for batch := 0; batch < 100; batch++ {
		rows, err := c.Connections(ctx, tenant)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, conn := range rows {
			if err := c.Delete(ctx, tenant, conn.ID); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("cloud workspace cleanup exceeded its connection limit")
}

func ValidateConfig(endpoint, key string) error {
	if endpoint == "" && key == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" || len(key) < 32 {
		return errors.New("configure RUNTZ_SECURITY_API_URL as an HTTP(S) service origin and RUNTZ_SECURITY_API_KEY with at least 32 characters")
	}
	return nil
}
