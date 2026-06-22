package sdk

// Event represents a streaming event from a bai subprocess.
type Event struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	Name       string `json:"name,omitempty"`
	Input      string `json:"input,omitempty"`
	Status     string `json:"status,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

// Options configures a bai subprocess session.
type Options struct {
	// BinaryPath is the path to the bai binary. Defaults to "bai" (found in PATH).
	BinaryPath string

	// Model overrides the default model.
	Model string

	// WorkDir sets the working directory for the subprocess.
	WorkDir string

	// MaxTurns limits agentic loop iterations.
	MaxTurns int

	// AutoApprove auto-approves all tool calls.
	AutoApprove bool

	// Env sets additional environment variables for the subprocess.
	Env []string
}
