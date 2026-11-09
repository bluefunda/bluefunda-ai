// Package abap is the SAP ABAP domain layer shared by `bai abap` and the
// abaper binary: a client for the ABAPer gateway, the local SAP system
// registry, the ABAP tool pack for the agent, and the post-run verification
// gate. Ported from github.com/bluefunda/abaper (internal/client,
// internal/config/systems.go, internal/agentrun).
package abap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

// DefaultGatewayURL is the ABAPer gateway (abaper-gw) base URL.
const DefaultGatewayURL = "https://api.bluefunda.com"

// GatewayURL returns the gateway base URL: <PREFIX>ABAP_GATEWAY, then
// ABAPER_BASE_URL (the variable the standalone abaper CLI used), then the default.
func GatewayURL() string {
	if v := app.Getenv("ABAP_GATEWAY"); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := os.Getenv("ABAPER_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return DefaultGatewayURL
}

// Client talks to the ABAPer APIs behind abaper-gw. Every request carries the
// platform access token plus the credentials of the selected SAP system.
type Client struct {
	BaseURL    string
	Realm      string
	TokenFunc  func() (string, error)
	System     *SAPSystem // nil → requests that need SAP fail with a hint
	HTTPClient *http.Client
	// retryDelay is the backoff unit between retries; tests shorten it.
	retryDelay time.Duration
}

// NewClient returns a Client for the given gateway, realm, token source and SAP system.
func NewClient(baseURL, realm string, tokenFunc func() (string, error), sys *SAPSystem) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Realm:      realm,
		TokenFunc:  tokenFunc,
		System:     sys,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		retryDelay: time.Second,
	}
}

// apiResponse is the standard ABAPer response envelope.
type apiResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error,omitempty"`
}

// ErrNoSystem is returned when a call needs an SAP system and none is configured.
var ErrNoSystem = errors.New("no SAP system configured")

func noSystemError() error {
	return fmt.Errorf("%w — add one with `%s`", ErrNoSystem, SystemAddHint())
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	if c.System == nil {
		return nil, noSystemError()
	}
	token, err := c.TokenFunc()
	if err != nil {
		return nil, err
	}
	var payload []byte
	if body != nil {
		if payload, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.BaseURL+"/abaper"+path, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Realm", c.Realm)
		req.Header.Set("X-SAP-Host", c.System.Host)
		req.Header.Set("X-SAP-Client", c.System.Client)
		req.Header.Set("X-SAP-User", c.System.Username)
		req.Header.Set("X-SAP-Password", c.System.Password)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * c.retryDelay)
			continue
		}
		// Only retry transient conditions. Application failures (already
		// exists, syntax/activation errors, ADT locks) also come back as 5xx
		// but are deterministic, and retrying a create turns a clear error
		// into a confusing "already exists".
		if isRetryableStatus(resp.StatusCode) {
			text, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(text))
			time.Sleep(time.Duration(attempt+1) * c.retryDelay)
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("request failed after retries: %w", lastErr)
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func post[T any](c *Client, path string, body any) (*T, error) {
	resp, err := c.do(http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(text)))
	}
	var r apiResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !r.Success {
		return nil, fmt.Errorf("API error: %s", r.Error)
	}
	return &r.Data, nil
}

// ── Result types ─────────────────────────────────────────────────────────────

// ActivateMessage is one message from an activation attempt.
type ActivateMessage struct {
	Severity string `json:"severity"` // "E", "W", "I"
	Text     string `json:"text"`
	Line     int    `json:"line,omitempty"`
}

// ActivateResult is the outcome of an activation. Activated is false when SAP
// rejected it even though the HTTP call succeeded — always check it.
type ActivateResult struct {
	ObjectName string            `json:"object_name"`
	ObjectType string            `json:"object_type"`
	Activated  bool              `json:"activated"`
	Messages   []ActivateMessage `json:"messages,omitempty"`
}

// SyntaxMessage is one syntax-check finding.
type SyntaxMessage struct {
	Severity string `json:"severity"` // "error", "warning", "info", "hint"
	Text     string `json:"text"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// SyntaxCheckResult is the outcome of a syntax check.
type SyntaxCheckResult struct {
	ObjectName string          `json:"object_name"`
	ObjectType string          `json:"object_type"`
	Messages   []SyntaxMessage `json:"messages"`
}

// UnitTestResult is the outcome of an ABAP Unit run.
type UnitTestResult struct {
	ObjectName  string            `json:"object_name"`
	TotalTests  int               `json:"total_tests"`
	Passed      int               `json:"passed"`
	Failed      int               `json:"failed"`
	AllPassed   bool              `json:"all_passed"`
	TestClasses []TestClassResult `json:"test_classes"`
}

// TestClassResult holds the results of one test class.
type TestClassResult struct {
	Name    string             `json:"name"`
	Methods []TestMethodResult `json:"methods"`
}

// TestMethodResult holds the result of one test method.
type TestMethodResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "passed", "failed", "error"
	Message string `json:"message,omitempty"`
}

// ── Endpoints ────────────────────────────────────────────────────────────────

// Connect checks that the gateway can reach and log on to the SAP system.
func (c *Client) Connect() error {
	resp, err := c.do(http.MethodPost, "/api/v1/system/connect", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("connect failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(text)))
	}
	return nil
}

// Search finds objects whose name matches pattern (SAP wildcards, e.g. Z*).
func (c *Client) Search(pattern, objectType string) ([]map[string]any, error) {
	body := map[string]string{"object_name": pattern}
	if objectType != "" {
		body["object_type"] = objectType
	}
	r, err := post[struct {
		Objects []map[string]any `json:"Objects"`
	}](c, "/api/v1/objects/search", body)
	if err != nil {
		return nil, err
	}
	return r.Objects, nil
}

// Source returns an object's source code.
func (c *Client) Source(name, objectType string) (string, error) {
	r, err := post[map[string]any](c, "/api/v1/objects/get", map[string]string{
		"object_type": objectType,
		"object_name": name,
	})
	if err != nil {
		return "", err
	}
	src, _ := (*r)["source"].(string)
	return src, nil
}

// ListPackage lists the objects in a package, optionally filtered by type.
func (c *Client) ListPackage(pkg, objectType string) ([]map[string]any, error) {
	body := map[string]string{}
	if pkg != "" {
		body["package"] = pkg
	}
	if objectType != "" {
		body["object_type"] = objectType
	}
	r, err := post[[]map[string]any](c, "/api/v1/objects/list", body)
	if err != nil {
		return nil, err
	}
	return *r, nil
}

// Save creates or overwrites an object's source. The description makes the
// server take its create path, which also updates existing objects.
func (c *Client) Save(name, objectType, source string) error {
	_, err := post[map[string]any](c, "/api/v1/objects/create", map[string]string{
		"object_name": name,
		"object_type": objectType,
		"source":      source,
		"description": objectType + " " + name,
	})
	return err
}

// Activate activates an object. /api/v1/objects/activate 404s through the
// gateway; /api/v1/activate is the working route.
func (c *Client) Activate(name, objectType string) (*ActivateResult, error) {
	return post[ActivateResult](c, "/api/v1/activate", map[string]string{
		"object_name": name,
		"object_type": objectType,
	})
}

// SyntaxCheck validates source without saving it.
func (c *Client) SyntaxCheck(name, objectType, source string) (*SyntaxCheckResult, error) {
	return post[SyntaxCheckResult](c, "/api/v1/syntax-check", map[string]string{
		"object_name": name,
		"object_type": objectType,
		"source":      source,
	})
}

// UnitTests runs the object's ABAP Unit tests.
func (c *Client) UnitTests(name, objectType string) (*UnitTestResult, error) {
	return post[UnitTestResult](c, "/api/v1/unit-tests", map[string]string{
		"object_name": name,
		"object_type": objectType,
	})
}

// ActivationSummary renders activation messages on one line.
func ActivationSummary(msgs []ActivateMessage) string {
	if len(msgs) == 0 {
		return "no details returned"
	}
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m.Line > 0 {
			parts = append(parts, fmt.Sprintf("[%s] line %d: %s", m.Severity, m.Line, m.Text))
		} else {
			parts = append(parts, fmt.Sprintf("[%s] %s", m.Severity, m.Text))
		}
	}
	return strings.Join(parts, "; ")
}
