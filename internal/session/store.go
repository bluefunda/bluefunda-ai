// Package session persists bai code agentic loop history to disk so sessions
// can be resumed after a crash or deliberate exit.
//
// Files are stored at ~/.bai/sessions/<cwd-hash>/<session-id>.jsonl
// Each line is a JSON-encoded codeMessage (role/content/tool_calls/tool_call_id).
package session

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Message mirrors cmd.codeMessage but is exported for storage.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall mirrors cmd.codeToolCall.
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function FuncCall `json:"function"`
}

// FuncCall mirrors cmd.codeFuncCall.
type FuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Info describes a persisted session for listing.
type Info struct {
	ID        string
	Path      string
	CWD       string
	UpdatedAt time.Time
	Turns     int
	LastMsg   string
}

// Dir returns the per-cwd session directory, creating it if needed.
func Dir(cwd string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", md5.Sum([]byte(cwd)))[:8]
	dir := filepath.Join(home, ".bai", "sessions", hash)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// Path returns the full path for a session file.
func Path(cwd, sessionID string) (string, error) {
	dir, err := Dir(cwd)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionID+".jsonl"), nil
}

// Save writes messages atomically to path.
func Save(path string, messages []Message) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bai-session-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	w := bufio.NewWriter(tmp)
	for _, m := range messages {
		line, err := json.Marshal(m)
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return err
		}
		_, _ = w.Write(line)
		_ = w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// Load reads messages from a session file.
func Load(path string) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var msgs []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for scanner.Scan() {
		var m Message
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, scanner.Err()
}

// List returns sessions for the given cwd directory, newest first.
func List(cwd string) ([]Info, error) {
	dir, err := Dir(cwd)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var infos []Info
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		id := e.Name()[:len(e.Name())-6]
		path := filepath.Join(dir, e.Name())
		fi, _ := e.Info()
		msgs, _ := Load(path)
		info := Info{
			ID:    id,
			Path:  path,
			CWD:   cwd,
			Turns: len(msgs),
		}
		if fi != nil {
			info.UpdatedAt = fi.ModTime()
		}
		// last non-system message content snippet
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role != "system" && msgs[i].Content != "" {
				c := msgs[i].Content
				if len(c) > 60 {
					c = c[:57] + "..."
				}
				info.LastMsg = c
				break
			}
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt.After(infos[j].UpdatedAt)
	})
	return infos, nil
}
