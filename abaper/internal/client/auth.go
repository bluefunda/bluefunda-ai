package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bluefunda/abaper/internal/config"
)

type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type AuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func authBaseURL(realm string) string {
	return fmt.Sprintf("https://auth.bluefunda.com/realms/%s/protocol/openid-connect", realm)
}

func RequestDeviceCode(realm string) (*DeviceAuthResponse, error) {
	authURL := authBaseURL(realm)

	data := url.Values{
		"client_id": {config.ClientID},
		"scope":     {"openid"},
	}

	resp, err := http.Post(
		authURL+"/auth/device",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var authErr AuthErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&authErr)
		return nil, fmt.Errorf("device auth failed (%d): %s", resp.StatusCode, authErr.ErrorDescription)
	}

	var result DeviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode device response: %w", err)
	}
	return &result, nil
}

func PollForToken(realm, deviceCode string, interval int) (*TokenResponse, error) {
	authURL := authBaseURL(realm)
	pollInterval := time.Duration(interval) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second
	}

	for {
		data := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":   {config.ClientID},
			"device_code": {deviceCode},
		}

		resp, err := http.Post(
			authURL+"/token",
			"application/x-www-form-urlencoded",
			strings.NewReader(data.Encode()),
		)
		if err != nil {
			return nil, fmt.Errorf("poll token: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			var token TokenResponse
			err := json.NewDecoder(resp.Body).Decode(&token)
			_ = resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("decode token: %w", err)
			}
			return &token, nil
		}

		var authErr AuthErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&authErr)
		_ = resp.Body.Close()

		switch authErr.Error {
		case "authorization_pending", "slow_down":
			time.Sleep(pollInterval)
			continue
		case "expired_token":
			return nil, fmt.Errorf("device code expired, please run login again")
		case "access_denied":
			return nil, fmt.Errorf("login denied by user")
		default:
			return nil, fmt.Errorf("auth error: %s", authErr.ErrorDescription)
		}
	}
}

func RefreshAccessToken(realm, refreshToken string) (*TokenResponse, error) {
	authURL := authBaseURL(realm)

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {config.ClientID},
		"refresh_token": {refreshToken},
	}

	resp, err := http.Post(
		authURL+"/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (status %d)", resp.StatusCode)
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode refreshed token: %w", err)
	}
	return &token, nil
}
