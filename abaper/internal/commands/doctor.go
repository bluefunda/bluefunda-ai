package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/internal/health"
	"github.com/bluefunda/abaper/styles"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Aliases: []string{"health"},
	Short:   "Run health checks on abaper's config, auth, and SAP connections",
	RunE:    runDoctor,
}

// checkResult is one row of doctor output: ok=false,warn=false is a hard
// failure; warn=true is a non-fatal note (expiring token, no systems, etc).
type checkResult struct {
	label string
	ok    bool
	warn  bool
	info  string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var results []checkResult

	configDir := config.ConfigDirPath()
	results = append(results, checkResult{label: "Config directory", ok: configDir != "", info: configDir})

	report := health.Check(context.Background(), true)

	results = append(results, tokenCheck(report))
	results = append(results, checkResult{
		label: "Gateway reachable",
		ok:    report.GatewayReachable,
		info:  fmt.Sprintf("%s (%s)", report.GatewayStatus, report.GatewayLatency.Round(time.Millisecond)),
	})

	if len(report.Systems) == 0 {
		results = append(results, checkResult{
			label: "SAP systems",
			warn:  true,
			info:  "none configured — run: abaper system add --help",
		})
	} else {
		for _, s := range report.Systems {
			info := s.Host
			if s.Err != "" {
				info = s.Err
			}
			results = append(results, checkResult{
				label: fmt.Sprintf("SAP system %q", s.Name),
				ok:    s.Reachable,
				info:  info,
			})
		}
	}

	results = append(results, versionCheck())

	printChecks(results)
	return nil
}

func tokenCheck(r health.Report) checkResult {
	switch {
	case !r.TokenPresent:
		return checkResult{label: "Authentication", info: "not logged in — run: abaper login"}
	case !r.TokenValid:
		return checkResult{label: "Authentication", info: "token expired — run: abaper login"}
	case r.TokenExpiresIn < 10*time.Minute:
		return checkResult{label: "Authentication", warn: true, info: fmt.Sprintf("expires in %s", r.TokenExpiresIn.Round(time.Second))}
	default:
		return checkResult{label: "Authentication", ok: true, info: fmt.Sprintf("valid for %s", r.TokenExpiresIn.Round(time.Minute))}
	}
}

// versionCheck compares the running version against the latest GitHub
// release. Any failure to reach the GitHub API is a warning, not an error —
// update checks are best-effort.
func versionCheck() checkResult {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	latest, err := health.LatestRelease(ctx)
	if err != nil {
		return checkResult{label: "Version", warn: true, info: version + " (couldn't check for updates)"}
	}

	current := strings.TrimPrefix(version, "v")
	if current != "dev" && latest != current {
		return checkResult{label: "Version", warn: true, info: fmt.Sprintf("%s (%s available — run: abaper update)", current, latest)}
	}
	return checkResult{label: "Version", ok: true, info: current + " (latest)"}
}

func printChecks(results []checkResult) {
	var errs, warns int
	for _, r := range results {
		icon := lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render("✓")
		switch {
		case !r.ok && !r.warn:
			icon = lipgloss.NewStyle().Foreground(styles.ColorError).Render("✗")
			errs++
		case r.warn:
			icon = lipgloss.NewStyle().Foreground(styles.ColorWarning).Render("!")
			warns++
		}
		line := fmt.Sprintf("%s %-28s", icon, r.label)
		if r.info != "" {
			line += "  " + styles.StyleMuted.Render(r.info)
		}
		fmt.Println(line)
	}

	fmt.Println()
	switch {
	case errs > 0:
		fmt.Println(styles.StyleError.Render(fmt.Sprintf("%d error(s), %d warning(s)", errs, warns)))
	case warns > 0:
		fmt.Println(styles.StyleWarning.Render(fmt.Sprintf("%d warning(s)", warns)))
	default:
		fmt.Println(styles.StyleSuccess.Render("All checks passed"))
	}
}
