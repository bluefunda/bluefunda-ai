package abaplint

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/bluefunda/abaper/types"
)

//go:embed default_config.json
var defaultConfig []byte

// Runner shells out to the abaplint CLI for diagnostics.
type Runner struct {
	mu          sync.Mutex
	abaplintBin string // path to abaplint binary
	workDir     string // workspace root (to look for user's abaplint.json)
}

// abaplint JSON output format
type issue struct {
	Description string   `json:"description"`
	Key         string   `json:"key"`
	File        string   `json:"file"`
	Start       position `json:"start"`
	End         position `json:"end"`
	Severity    string   `json:"severity"`
}

type position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// NewRunner creates a runner. It locates the abaplint binary and accepts
// an optional workspace root for finding user config.
func NewRunner(workDir string) (*Runner, error) {
	bin, err := exec.LookPath("abaplint")
	if err != nil {
		return nil, fmt.Errorf("abaplint not found in PATH: %w", err)
	}
	return &Runner{
		abaplintBin: bin,
		workDir:     workDir,
	}, nil
}

// Check runs abaplint on the given source and returns diagnostics.
func (r *Runner) Check(objectType, objectName, source string) (*types.SyntaxCheckResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create temp directory with abapGit structure
	tmpDir, err := os.MkdirTemp("", "abaplint-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return nil, fmt.Errorf("create src dir: %w", err)
	}

	// Write source with abapGit naming convention
	filename := abapGitFilename(objectType, objectName)
	srcFile := filepath.Join(srcDir, filename)
	if err := os.WriteFile(srcFile, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("write source: %w", err)
	}

	// Write abaplint config (use workspace config if available, else default)
	configPath := filepath.Join(tmpDir, "abaplint.json")
	if err := r.writeConfig(configPath); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Run abaplint
	cmd := exec.Command(r.abaplintBin, configPath, "--format", "json")
	cmd.Dir = tmpDir
	out, _ := cmd.CombinedOutput() // abaplint exits 1 when issues found, that's OK

	// Parse JSON output
	issues, err := parseOutput(out)
	if err != nil {
		return nil, fmt.Errorf("parse abaplint output: %w", err)
	}

	// Convert to SyntaxCheckResult
	result := &types.SyntaxCheckResult{
		ObjectName: objectName,
		ObjectType: objectType,
	}
	for _, iss := range issues {
		// abaplint uses 1-indexed rows/cols, LSP uses 0-indexed
		result.Messages = append(result.Messages, types.SyntaxCheckMessage{
			Severity: mapSeverity(iss.Severity),
			Text:     iss.Description,
			Line:     iss.Start.Row - 1,
			Column:   iss.Start.Col - 1,
			EndLine:  iss.End.Row - 1,
			EndCol:   iss.End.Col - 1,
			Code:     iss.Key,
		})
	}

	return result, nil
}

func (r *Runner) writeConfig(configPath string) error {
	// Check for user config in workspace
	if r.workDir != "" {
		for _, name := range []string{"abaplint.json", "abaplint.jsonc"} {
			userConfig := filepath.Join(r.workDir, name)
			if data, err := os.ReadFile(userConfig); err == nil {
				return os.WriteFile(configPath, data, 0o644)
			}
		}
	}
	// Use embedded default config
	return os.WriteFile(configPath, defaultConfig, 0o644)
}

func parseOutput(data []byte) ([]issue, error) {
	// abaplint outputs JSON on the last non-empty line of combined output
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "[") {
			var issues []issue
			if err := json.Unmarshal([]byte(line), &issues); err != nil {
				return nil, err
			}
			return issues, nil
		}
	}
	// No JSON output means no issues
	return nil, nil
}

func mapSeverity(s string) string {
	switch strings.ToLower(s) {
	case "error":
		return "error"
	case "warning":
		return "warning"
	case "information", "info":
		return "info"
	default:
		return "warning"
	}
}

// abapGitFilename returns the conventional abapGit filename for an object.
func abapGitFilename(objectType, objectName string) string {
	name := strings.ToLower(objectName)
	ext := document.ObjectTypeToFileExt(objectType)
	// ObjectTypeToFileExt returns ".clas.abap", ".intf.abap", etc.
	// For programs it returns ".abap", but abapGit uses ".prog.abap"
	if ext == ".abap" {
		ext = ".prog.abap"
	}
	return name + ext
}
