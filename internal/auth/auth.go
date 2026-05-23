package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/config"
)

// TokenResponse holds the tokens returned by Keycloak.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// Expiry returns the absolute expiry time based on ExpiresIn seconds.
func (t *TokenResponse) Expiry() time.Time {
	return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
}

// deviceAuthResponse is the response from the device authorization endpoint.
type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// LoginWithDevice performs an OAuth2 Device Authorization Grant (RFC 8628).
// It shows a user code, opens the browser for verification, and polls
// until the user completes login.
func LoginWithDevice(domain, realm string) (*TokenResponse, error) {
	baseURL := config.AuthURL(domain, realm)

	// Step 1: Request device code
	deviceURL := baseURL + "/auth/device"
	data := url.Values{
		"client_id": {config.DefaultClientID},
		"scope":     {"openid"},
	}

	resp, err := http.Post(deviceURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("device auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var dev deviceAuthResponse
	if err := json.Unmarshal(body, &dev); err != nil {
		return nil, fmt.Errorf("parse device response: %w", err)
	}

	// Step 2: Show user code and open browser
	verifyURL := dev.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = dev.VerificationURI
	}

	fmt.Printf("\nOpen this URL in your browser:\n  %s\n\n", verifyURL)
	fmt.Printf("Enter code: %s\n\n", dev.UserCode)
	fmt.Println("Waiting for login...")

	_ = openBrowser(verifyURL)

	// Step 3: Poll for token
	interval := dev.Interval
	if interval < 5 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(dev.ExpiresIn) * time.Second)

	tokenURL := baseURL + "/token"
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)

		tok, done, err := pollToken(tokenURL, dev.DeviceCode)
		if err != nil {
			return nil, err
		}
		if done {
			return tok, nil
		}
	}

	return nil, fmt.Errorf("login timed out (code expired)")
}

// pollToken polls the token endpoint once. Returns (token, true, nil) on success,
// (nil, false, nil) if still pending, or (nil, false, err) on fatal error.
func pollToken(tokenURL, deviceCode string) (*TokenResponse, bool, error) {
	data := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {config.DefaultClientID},
		"device_code": {deviceCode},
	}

	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, false, fmt.Errorf("token poll: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var tok TokenResponse
		if err := json.Unmarshal(body, &tok); err != nil {
			return nil, false, fmt.Errorf("parse token: %w", err)
		}
		return &tok, true, nil
	}

	// Check error type
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil {
		switch errResp.Error {
		case "authorization_pending", "slow_down":
			return nil, false, nil // keep polling
		case "expired_token":
			return nil, false, fmt.Errorf("device code expired — run 'ai login' again")
		case "access_denied":
			return nil, false, fmt.Errorf("login denied by user")
		}
	}

	return nil, false, fmt.Errorf("auth failed (HTTP %d): %s", resp.StatusCode, string(body))
}

// Refresh performs a Keycloak refresh_token grant.
func Refresh(domain, realm, refreshToken string) (*TokenResponse, error) {
	tokenURL := config.AuthURL(domain, realm) + "/token"
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {config.DefaultClientID},
		"refresh_token": {refreshToken},
	}
	return postToken(tokenURL, data)
}

func postToken(tokenURL string, data url.Values) (*TokenResponse, error) {
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	return &tok, nil
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
