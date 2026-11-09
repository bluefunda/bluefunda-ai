package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bluefunda/abaper/rest/models"
	"go.uber.org/zap"
)

const githubAPIBase = "https://api.github.com"

var githubHTTPClient = &http.Client{Timeout: 30 * time.Second}

// resolveGitHubToken returns the user's OAuth token from the request header,
// falling back to the server-level GH_PAT env var.
func resolveGitHubToken(r *http.Request) string {
	if token := r.Header.Get("X-GitHub-Token"); token != "" {
		return token
	}
	return os.Getenv("GH_PAT")
}

// githubRequest makes a request to the GitHub API with the given token.
func githubRequest(method, path, token string) ([]byte, int, error) {
	reqURL := githubAPIBase + path

	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "abaper/1.0")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// githubOAuthCallbackHandler exchanges an OAuth code for an access token.
// POST /api/v1/github/oauth/callback
func (rs *RestServer) githubOAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		rs.sendError(w, "code is required", http.StatusBadRequest)
		return
	}

	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		rs.sendError(w, "GitHub OAuth not configured", http.StatusServiceUnavailable)
		return
	}

	rs.logger.Info("Exchanging GitHub OAuth code for token")

	// Exchange code for access token
	tokenURL := "https://github.com/login/oauth/access_token"
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {req.Code},
	}

	tokenReq, err := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		rs.sendError(w, "Failed to create token request", http.StatusInternalServerError)
		return
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("Accept", "application/json")

	resp, err := githubHTTPClient.Do(tokenReq)
	if err != nil {
		rs.sendError(w, "Failed to exchange code: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		rs.sendError(w, "Failed to parse token response", http.StatusInternalServerError)
		return
	}

	if tokenResp.Error != "" {
		rs.sendError(w, tokenResp.ErrorDesc, http.StatusBadRequest)
		return
	}

	// Fetch GitHub user info to return alongside token
	userBody, status, err := githubRequest("GET", "/user", tokenResp.AccessToken)
	var username string
	if err == nil && status == http.StatusOK {
		var user struct {
			Login string `json:"login"`
		}
		if json.Unmarshal(userBody, &user) == nil {
			username = user.Login
		}
	}

	rs.sendSuccess(w, map[string]string{
		"access_token": tokenResp.AccessToken,
		"token_type":   tokenResp.TokenType,
		"scope":        tokenResp.Scope,
		"username":     username,
	})
}

// githubUserHandler returns the authenticated GitHub user's info.
// POST /api/v1/github/user
func (rs *RestServer) githubUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := resolveGitHubToken(r)
	if token == "" {
		rs.sendError(w, "Not authenticated with GitHub", http.StatusUnauthorized)
		return
	}

	body, status, err := githubRequest("GET", "/user", token)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		rs.sendError(w, fmt.Sprintf("GitHub API error %d: %s", status, string(body)), status)
		return
	}

	var user struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(body, &user); err != nil {
		rs.sendError(w, "Failed to parse GitHub response", http.StatusInternalServerError)
		return
	}

	rs.sendSuccess(w, user)
}

// githubBranchesHandler lists branches for a repository.
// POST /api/v1/github/branches
func (rs *RestServer) githubBranchesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.GitHubRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Repo == "" {
		rs.sendError(w, "owner and repo are required", http.StatusBadRequest)
		return
	}

	token := resolveGitHubToken(r)

	rs.logger.Info("Listing GitHub branches",
		zap.String("owner", req.Owner),
		zap.String("repo", req.Repo))

	path := fmt.Sprintf("/repos/%s/%s/branches?per_page=100",
		url.PathEscape(req.Owner),
		url.PathEscape(req.Repo))

	body, status, err := githubRequest("GET", path, token)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		rs.sendError(w, fmt.Sprintf("GitHub API error %d: %s", status, string(body)), status)
		return
	}

	var branches []models.GitHubBranch
	if err := json.Unmarshal(body, &branches); err != nil {
		rs.sendError(w, "Failed to parse GitHub response", http.StatusInternalServerError)
		return
	}

	rs.sendSuccess(w, branches)
}

// githubTreeHandler lists directory contents for a repository path.
// POST /api/v1/github/tree
func (rs *RestServer) githubTreeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.GitHubRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Repo == "" {
		rs.sendError(w, "owner and repo are required", http.StatusBadRequest)
		return
	}

	token := resolveGitHubToken(r)

	rs.logger.Info("Listing GitHub directory",
		zap.String("owner", req.Owner),
		zap.String("repo", req.Repo),
		zap.String("path", req.Path),
		zap.String("branch", req.Branch))

	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s",
		url.PathEscape(req.Owner),
		url.PathEscape(req.Repo),
		req.Path)

	if req.Branch != "" {
		apiPath += "?ref=" + url.QueryEscape(req.Branch)
	}

	body, status, err := githubRequest("GET", apiPath, token)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		rs.sendError(w, fmt.Sprintf("GitHub API error %d: %s", status, string(body)), status)
		return
	}

	var contents []models.GitHubContent
	if err := json.Unmarshal(body, &contents); err != nil {
		rs.sendError(w, "Failed to parse GitHub response", http.StatusInternalServerError)
		return
	}

	rs.sendSuccess(w, contents)
}

// githubFileHandler retrieves a single file's content from a repository.
// POST /api/v1/github/file
func (rs *RestServer) githubFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.GitHubRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Repo == "" || req.Path == "" {
		rs.sendError(w, "owner, repo, and path are required", http.StatusBadRequest)
		return
	}

	token := resolveGitHubToken(r)

	rs.logger.Info("Fetching GitHub file",
		zap.String("owner", req.Owner),
		zap.String("repo", req.Repo),
		zap.String("path", req.Path),
		zap.String("branch", req.Branch))

	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s",
		url.PathEscape(req.Owner),
		url.PathEscape(req.Repo),
		req.Path)

	if req.Branch != "" {
		apiPath += "?ref=" + url.QueryEscape(req.Branch)
	}

	body, status, err := githubRequest("GET", apiPath, token)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		rs.sendError(w, fmt.Sprintf("GitHub API error %d: %s", status, string(body)), status)
		return
	}

	// Parse the single-file response
	var raw struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		Size     int    `json:"size"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		rs.sendError(w, "Failed to parse GitHub response", http.StatusInternalServerError)
		return
	}

	if raw.Type != "file" {
		rs.sendError(w, "Path is not a file", http.StatusBadRequest)
		return
	}

	// Decode base64 content
	decoded := raw.Content
	if raw.Encoding == "base64" {
		decodedBytes, err := base64.StdEncoding.DecodeString(raw.Content)
		if err != nil {
			rs.sendError(w, "Failed to decode file content", http.StatusInternalServerError)
			return
		}
		decoded = string(decodedBytes)
	}

	result := models.GitHubFileContent{
		Name:     raw.Name,
		Path:     raw.Path,
		Content:  decoded,
		Encoding: "utf-8",
		Size:     raw.Size,
	}

	rs.sendSuccess(w, result)
}
