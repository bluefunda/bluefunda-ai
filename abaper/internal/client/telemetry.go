package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/bluefunda/abaper/internal/config"
)

type telemetryEvent struct {
	Event      string         `json:"event"`
	Properties map[string]any `json:"properties"`
}

// Track sends a telemetry event to the ABAPer API.
// It is fire-and-forget — errors are silently ignored.
func Track(event string, extra map[string]string) {
	cfg := config.Load()

	props := map[string]any{
		"client":   "cli",
		"platform": runtime.GOOS,
		"arch":     runtime.GOARCH,
	}
	for k, v := range extra {
		props[k] = v
	}

	body, err := json.Marshal(telemetryEvent{Event: event, Properties: props})
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", cfg.BaseURL+"/abaper/telemetry", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Include auth token if available
	tokens, err := config.LoadTokens()
	if err == nil && tokens.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
