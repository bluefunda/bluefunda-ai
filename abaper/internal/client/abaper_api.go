package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/types"
)

// APIResponse is the standard response envelope from ABAPer APIs.
type APIResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error,omitempty"`
}

// Client communicates with ABAPer APIs exposed via abaper-gw.
type Client struct {
	BaseURL     string
	Token       string
	Realm       string
	HTTPClient  *http.Client
	SAPHost     string
	SAPClient   string
	SAPUser     string
	SAPPassword string
}

// NewClient creates a Client from the current config and stored tokens.
func NewClient() (*Client, error) {
	cfg := config.Load()

	tokens, err := config.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("not logged in — run 'abaper login' first")
	}

	// Refresh if expired
	if time.Now().UnixMilli() >= tokens.ExpiresAt {
		refreshed, err := RefreshAccessToken(cfg.Realm, tokens.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("session expired — run 'abaper login' again: %w", err)
		}
		tokens = &config.Tokens{
			AccessToken:  refreshed.AccessToken,
			RefreshToken: refreshed.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(refreshed.ExpiresIn) * time.Second).UnixMilli(),
		}
		if err := config.SaveTokens(tokens); err != nil {
			return nil, fmt.Errorf("save refreshed tokens: %w", err)
		}
	}

	c := &Client{
		BaseURL: cfg.BaseURL,
		Token:   tokens.AccessToken,
		Realm:   cfg.Realm,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	if sysCfg, err := config.LoadSystems(); err == nil {
		if active := sysCfg.GetActive(); active != nil {
			c.SAPHost = active.Host
			c.SAPClient = active.Client
			c.SAPUser = active.Username
			c.SAPPassword = active.Password
		}
	}

	return c, nil
}

// Do sends an HTTP request with auth headers, retry logic, and structured error handling.
func (c *Client) Do(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.BaseURL+"/abaper"+path, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("X-Realm", c.Realm)
		if c.SAPHost != "" {
			req.Header.Set("X-SAP-Host", c.SAPHost)
			req.Header.Set("X-SAP-Client", c.SAPClient)
			req.Header.Set("X-SAP-User", c.SAPUser)
			req.Header.Set("X-SAP-Password", c.SAPPassword)
		}

		// Reset body reader for retries
		if body != nil {
			data, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(data)
			req.Body = io.NopCloser(bodyReader)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		// Only retry genuinely transient conditions. Most application-level
		// failures (already exists, syntax/activation errors, ADT lock
		// errors) also come back as 5xx, and are deterministic — retrying
		// them just wastes time and, for non-idempotent creates, can turn a
		// clear error into a confusing "already exists" on the next attempt.
		if isRetryableStatus(resp.StatusCode) {
			text, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(text))
			time.Sleep(time.Duration(attempt+1) * time.Second)
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
	default:
		return false
	}
}

// Post sends a POST request and decodes the response into the target type.
func Post[T any](c *Client, path string, body any) (*T, error) {
	resp, err := c.Do(http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(text))
	}

	var apiResp APIResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	return &apiResp.Data, nil
}

// Get sends a GET request and decodes the response.
func Get[T any](c *Client, path string) (*T, error) {
	resp, err := c.Do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(text))
	}

	var apiResp APIResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &apiResp.Data, nil
}

// HealthCheck calls the health endpoint.
func (c *Client) HealthCheck() (map[string]string, error) {
	resp, err := c.Do(http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SystemConnect tests SAP system connectivity.
func (c *Client) SystemConnect(ctx context.Context, sapHost, sapClient, sapUser, sapPassword string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/abaper/api/v1/system/connect", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-Realm", c.Realm)
	req.Header.Set("X-SAP-Host", sapHost)
	req.Header.Set("X-SAP-Client", sapClient)
	req.Header.Set("X-SAP-User", sapUser)
	req.Header.Set("X-SAP-Password", sapPassword)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("connect failed (%d): %s", resp.StatusCode, string(text))
	}

	return nil
}

// SearchObjects searches for ABAP objects by pattern.
func (c *Client) SearchObjects(pattern, objectType string) ([]map[string]any, error) {
	body := map[string]string{"object_name": pattern}
	if objectType != "" {
		body["object_type"] = objectType
	}

	type SearchResult struct {
		Objects []map[string]any `json:"Objects"`
	}

	result, err := Post[SearchResult](c, "/api/v1/objects/search", body)
	if err != nil {
		return nil, err
	}
	return result.Objects, nil
}

// GetObject retrieves an ABAP object's source code.
func (c *Client) GetObject(objectType, objectName string) (*map[string]any, error) {
	body := map[string]string{
		"object_type": objectType,
		"object_name": objectName,
	}
	return Post[map[string]any](c, "/api/v1/objects/get", body)
}

// CreateObject creates a new ABAP object with source code. A non-empty
// description is required so the server routes this through its create
// path rather than its save/update path (see rest/server createObjectHandler).
func (c *Client) CreateObject(objectName, objectType, source string) error {
	body := map[string]string{
		"object_name": objectName,
		"object_type": objectType,
		"source":      source,
		"description": fmt.Sprintf("%s %s", objectType, objectName),
	}
	_, err := Post[map[string]any](c, "/api/v1/objects/create", body)
	return err
}

// ActivateMessage is a single message returned by an activation attempt.
type ActivateMessage struct {
	Severity string `json:"severity"` // "E", "W", "I"
	Text     string `json:"text"`
	Line     int    `json:"line,omitempty"`
}

// ActivateResult is the outcome of an activation attempt. Activated is false
// when SAP rejected activation (syntax/semantic errors, cancelled edits,
// etc.) even though the HTTP call itself succeeded — callers must check it,
// not just the error return.
type ActivateResult struct {
	ObjectName string            `json:"object_name"`
	ObjectType string            `json:"object_type"`
	Activated  bool              `json:"activated"`
	Messages   []ActivateMessage `json:"messages,omitempty"`
}

// Activate activates an ABAP object. Note: the working route is
// /api/v1/activate — /api/v1/objects/activate 404s through the gateway.
func (c *Client) Activate(objectName, objectType string) (*ActivateResult, error) {
	body := map[string]string{
		"object_name": objectName,
		"object_type": objectType,
	}
	return Post[ActivateResult](c, "/api/v1/activate", body)
}

// SyntaxCheck runs syntax validation on source code.
func (c *Client) SyntaxCheck(objectName, objectType, source string) (*types.SyntaxCheckResult, error) {
	body := map[string]string{
		"object_name": objectName,
		"object_type": objectType,
		"source":      source,
	}
	return Post[types.SyntaxCheckResult](c, "/api/v1/syntax-check", body)
}

// FormatCode formats ABAP source code.
func (c *Client) FormatCode(source string) (string, error) {
	body := map[string]string{"source": source}
	type FormatResult struct {
		Source string `json:"source"`
	}
	result, err := Post[FormatResult](c, "/api/v1/format", body)
	if err != nil {
		return "", err
	}
	return result.Source, nil
}

// TransportInfo retrieves transport request information.
func (c *Client) TransportInfo() (*map[string]any, error) {
	return Post[map[string]any](c, "/api/v1/transports/info", nil)
}

// CreateTransport creates a transport request.
func (c *Client) CreateTransport(description, targetPackage string) (string, error) {
	body := map[string]string{
		"description": description,
		"package":     targetPackage,
	}
	type Result struct {
		Transport string `json:"transport"`
	}
	result, err := Post[Result](c, "/api/v1/transports/create", body)
	if err != nil {
		return "", err
	}
	return result.Transport, nil
}

// GatewayVersion retrieves the gateway version.
func (c *Client) GatewayVersion() (map[string]string, error) {
	resp, err := c.Do(http.MethodGet, "/version", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListObjects lists ABAP objects in a package, optionally filtered by type.
func (c *Client) ListObjects(packageName, objectType string) ([]map[string]any, error) {
	body := map[string]string{}
	if packageName != "" {
		body["package"] = packageName
	}
	if objectType != "" {
		body["object_type"] = objectType
	}

	result, err := Post[[]map[string]any](c, "/api/v1/objects/list", body)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// RunUnitTests executes ABAP unit tests for an object.
func (c *Client) RunUnitTests(objectName, objectType string) (*types.UnitTestResult, error) {
	body := map[string]string{
		"object_name": objectName,
		"object_type": objectType,
	}
	return Post[types.UnitTestResult](c, "/api/v1/unit-tests", body)
}

// Completion retrieves code completion suggestions.
func (c *Client) Completion(objectName, objectType, source string, line, column int) (*map[string]any, error) {
	body := map[string]any{
		"object_name": objectName,
		"object_type": objectType,
		"source":      source,
		"line":        line,
		"column":      column,
	}
	return Post[map[string]any](c, "/api/v1/completion", body)
}

// Navigation retrieves code navigation (go-to-definition) results.
func (c *Client) Navigation(objectName, objectType, source string, line, column int) (*map[string]any, error) {
	body := map[string]any{
		"object_name": objectName,
		"object_type": objectType,
		"source":      source,
		"line":        line,
		"column":      column,
	}
	return Post[map[string]any](c, "/api/v1/navigation", body)
}

// PackageContents lists the contents of a package.
func (c *Client) PackageContents(packageName string) ([]map[string]any, error) {
	body := map[string]string{
		"object_type": "package",
		"object_name": packageName,
	}

	type PkgResult struct {
		Objects []map[string]any `json:"objects"`
	}

	result, err := Post[PkgResult](c, "/api/v1/objects/get", body)
	if err != nil {
		return nil, err
	}
	return result.Objects, nil
}

// ChatTitle generates a title for a chat session.
func (c *Client) ChatTitle(chatID, prompt string) (string, error) {
	body := map[string]string{"prompt": prompt}
	type TitleResult struct {
		GeneratedTitle string `json:"generatedTitle"`
		Title          string `json:"title"`
	}
	result, err := Post[TitleResult](c, "/ai/chats/"+chatID+"/title", body)
	if err != nil {
		return "", err
	}
	if result.GeneratedTitle != "" {
		return result.GeneratedTitle, nil
	}
	return result.Title, nil
}
