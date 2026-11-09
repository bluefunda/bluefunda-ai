package models

// GitHubRepoRequest represents a request scoped to a GitHub repository.
type GitHubRepoRequest struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Path   string `json:"path,omitempty"`
	Branch string `json:"branch,omitempty"`
}

// GitHubBranch represents a branch returned from the GitHub API.
type GitHubBranch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
}

// GitHubContent represents a file or directory entry from the GitHub Contents API.
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	Size        int    `json:"size,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

// GitHubFileContent represents a single file's content from the GitHub Contents API.
type GitHubFileContent struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Size     int    `json:"size"`
}
