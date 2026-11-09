// Package health provides a single source of truth for abaper's
// login/auth state and SAP system connectivity, shared by `abaper status`,
// `abaper doctor`, and the bare TUI's status bar.
package health

import (
	"context"
	"sync"
	"time"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/internal/config"
)

// systemCheckTimeout bounds each individual SAP system connectivity check so
// a single unreachable system can't stall a status report.
const systemCheckTimeout = 5 * time.Second

// SystemStatus is the connectivity result for one configured SAP system.
type SystemStatus struct {
	Name      string
	Host      string
	Active    bool
	Reachable bool
	Err       string
	Latency   time.Duration
}

// Report is a point-in-time snapshot of abaper's auth and connectivity state.
type Report struct {
	TokenPresent     bool
	TokenValid       bool
	TokenExpiresIn   time.Duration
	GatewayReachable bool
	GatewayLatency   time.Duration
	GatewayStatus    string
	Systems          []SystemStatus
}

// Check builds a Report. When all is false, only the active SAP system is
// checked (the fast path used by the TUI's status bar); when true, every
// configured system is checked concurrently (used by `status`/`doctor` and
// the TUI's `/status` slash command).
func Check(ctx context.Context, all bool) Report {
	var report Report

	tokens, err := config.LoadTokens()
	report.TokenPresent = err == nil && tokens != nil && tokens.AccessToken != ""
	if report.TokenPresent {
		report.TokenExpiresIn = time.Until(time.UnixMilli(tokens.ExpiresAt))
		report.TokenValid = report.TokenExpiresIn > 0
	}

	if !report.TokenPresent {
		return report
	}

	c, err := client.NewClient()
	if err != nil {
		return report
	}

	start := time.Now()
	if hc, err := c.HealthCheck(); err == nil {
		report.GatewayReachable = true
		report.GatewayStatus = hc["status"]
	}
	report.GatewayLatency = time.Since(start)

	sysCfg, err := config.LoadSystems()
	if err != nil || len(sysCfg.Systems) == 0 {
		return report
	}

	systems := sysCfg.Systems
	if !all {
		active := sysCfg.GetActive()
		if active == nil {
			return report
		}
		systems = []config.SAPSystem{*active}
	}

	report.Systems = checkSystems(ctx, c, systems, sysCfg.Active)
	return report
}

func checkSystems(ctx context.Context, c *client.Client, systems []config.SAPSystem, activeID string) []SystemStatus {
	results := make([]SystemStatus, len(systems))
	var wg sync.WaitGroup
	for i, sys := range systems {
		wg.Add(1)
		go func(i int, sys config.SAPSystem) {
			defer wg.Done()
			results[i] = checkOneSystem(ctx, c, sys, sys.ID == activeID)
		}(i, sys)
	}
	wg.Wait()
	return results
}

func checkOneSystem(ctx context.Context, c *client.Client, sys config.SAPSystem, active bool) SystemStatus {
	status := SystemStatus{Name: sys.Name, Host: sys.Host, Active: active}

	cctx, cancel := context.WithTimeout(ctx, systemCheckTimeout)
	defer cancel()

	start := time.Now()
	err := c.SystemConnect(cctx, sys.Host, sys.Client, sys.Username, sys.Password)
	status.Latency = time.Since(start)
	if err != nil {
		status.Err = err.Error()
		return status
	}
	status.Reachable = true
	return status
}
