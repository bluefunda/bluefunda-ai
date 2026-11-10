// Package requests is the BlueRequests (TRM change and release management)
// domain shared by the breq binary and `bai requests`: the TRM BFF
// connection and status colouring. The dashboard lives in requests/tui and
// the A4H session model in requests/a4h.
package requests

import (
	"strings"

	"github.com/fatih/color"

	trmpb "github.com/bluefunda/bluefunda-ai/api/proto/trm"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

// Conn is an authenticated connection to the TRM BFF. Client is the TRM
// service client; it shadows the embedded connection's bai BFF client.
type Conn struct {
	*caigrpc.Conn
	Client trmpb.BFFServiceClient
}

// Wrap returns a TRM connection over an established core connection.
func Wrap(c *caigrpc.Conn) *Conn {
	return &Conn{Conn: c, Client: trmpb.NewBFFServiceClient(c.ClientConn())}
}

var (
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	cyan    = color.New(color.FgCyan).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
	blue    = color.New(color.FgBlue).SprintFunc()
	hiRed   = color.New(color.FgHiRed, color.Bold).SprintFunc()
)

// SeverityColor returns a terminal-colored severity string.
func SeverityColor(s string) string {
	switch s {
	case "low":
		return yellow(s)
	case "medium":
		return color.New(color.FgYellow, color.Bold).Sprint(s)
	case "high":
		return red(s)
	case "critical":
		return hiRed(s)
	default:
		return s
	}
}

// StatusColor returns a terminal-colored status string.
func StatusColor(s string) string {
	switch strings.ToLower(s) {
	case "planned":
		return cyan(s)
	case "inprogress", "in_progress":
		return yellow(s)
	case "completed", "imported":
		return green(s)
	case "qa", "toc_importing_to_qa", "toc_imported_to_qa":
		return magenta(s)
	case "uat", "uat_approved":
		return blue(s)
	case "release_management", "toc_release_initiated", "toc_released",
		"transport_release_from_dev", "transport_released",
		"import_to_prod", "schedule_import", "import_is_scheduled":
		return color.New(color.FgHiMagenta).Sprint(s)
	default:
		return s
	}
}
