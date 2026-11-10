package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

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

// OutputFormat controls how command results are rendered.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatQuiet OutputFormat = "quiet"
)

// Printer handles command output rendering. All commands write through
// a Printer so output format is consistent and testable.
type Printer struct {
	Out    io.Writer // stdout (data)
	Err    io.Writer // stderr (messages)
	Format OutputFormat
}

// DefaultPrinter returns a Printer that writes to os.Stdout/Stderr
// with table format.
func DefaultPrinter() *Printer {
	return &Printer{Out: os.Stdout, Err: os.Stderr, Format: FormatTable}
}

// Success prints a green success message to stderr.
func (p *Printer) Success(msg string) {
	if p.Format == FormatQuiet {
		return
	}
	fmt.Fprintf(p.Err, "%s %s\n", green("[OK]"), msg)
}

// Error prints a red error message to stderr.
func (p *Printer) Error(msg string) {
	fmt.Fprintf(p.Err, "%s %s\n", red("[ERROR]"), msg)
}

// Info prints a cyan info message to stderr.
func (p *Printer) Info(msg string) {
	if p.Format == FormatQuiet {
		return
	}
	fmt.Fprintf(p.Err, "%s %s\n", cyan("[INFO]"), msg)
}

// Warn prints a yellow warning message to stderr.
func (p *Printer) Warn(msg string) {
	if p.Format == FormatQuiet {
		return
	}
	fmt.Fprintf(p.Err, "%s %s\n", yellow("[WARN]"), msg)
}

// ProtoJSON serializes a protobuf message as formatted JSON to stdout.
func (p *Printer) ProtoJSON(msg proto.Message) {
	m := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false,
	}
	data, err := m.Marshal(msg)
	if err != nil {
		p.Error("marshal: " + err.Error())
		return
	}
	fmt.Fprintln(p.Out, string(data))
}

// JSON serializes any value as formatted JSON to stdout.
func (p *Printer) JSON(v any) {
	enc := json.NewEncoder(p.Out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// Table prints a table with the given headers and rows to stdout.
// In JSON mode, it serializes the rows as a list of maps.
// In quiet mode, it prints nothing.
func (p *Printer) Table(headers []string, rows [][]string) {
	switch p.Format {
	case FormatJSON:
		records := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			rec := make(map[string]string, len(headers))
			for i, h := range headers {
				if i < len(row) {
					rec[h] = row[i]
				}
			}
			records = append(records, rec)
		}
		p.JSON(records)
	case FormatQuiet:
		// Print only the first column of each row, one per line.
		for _, row := range rows {
			if len(row) > 0 {
				fmt.Fprintln(p.Out, row[0])
			}
		}
	default:
		w := tabwriter.NewWriter(p.Out, 0, 0, 2, ' ', 0)
		for i, h := range headers {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, h)
		}
		fmt.Fprintln(w)
		for i := range headers {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, "---")
		}
		fmt.Fprintln(w)
		for _, row := range rows {
			for i, col := range row {
				if i > 0 {
					fmt.Fprint(w, "\t")
				}
				fmt.Fprint(w, col)
			}
			fmt.Fprintln(w)
		}
		_ = w.Flush()
	}
}

// --- Package-level helpers for backward compatibility / convenience ---

func Success(msg string) { DefaultPrinter().Success(msg) }
func Error(msg string)   { DefaultPrinter().Error(msg) }
func Info(msg string)    { DefaultPrinter().Info(msg) }
func Warn(msg string)    { DefaultPrinter().Warn(msg) }
