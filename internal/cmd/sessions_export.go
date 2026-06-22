package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/session"
)

var (
	exportFormat string
	exportOutput string
	exportDir    string
)

var sessionsExportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export a code session to Markdown or JSON",
	Long: `Export a local bai code session to a readable format.

The session ID prefix (first 8 characters, as shown by 'bai sessions') is accepted.

Examples:
  bai sessions export abc12345
  bai sessions export abc12345 --format json
  bai sessions export abc12345 --output session.md
`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsExport,
}

func init() {
	sessionsExportCmd.Flags().StringVar(&exportFormat, "format", "md", "Output format: md or json")
	sessionsExportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Write to file instead of stdout")
	sessionsExportCmd.Flags().StringVar(&exportDir, "dir", "", "Working directory used when the session was created (default: current dir)")

	sessionsCmd.AddCommand(sessionsExportCmd)
}

func runSessionsExport(cmd *cobra.Command, args []string) error {
	prefix := args[0]

	cwd := exportDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	info, msgs, err := resolveSession(cwd, prefix)
	if err != nil {
		return err
	}

	var out string
	switch strings.ToLower(exportFormat) {
	case "json":
		b, err := json.MarshalIndent(msgs, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		out = string(b) + "\n"
	default: // "md"
		out = renderMarkdown(info, msgs)
	}

	if exportOutput != "" {
		if err := os.WriteFile(exportOutput, []byte(out), 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Session exported to %s\n", exportOutput)
		return nil
	}
	_, err = io.WriteString(cmd.OutOrStdout(), out)
	return err
}

// resolveSession finds a session by ID prefix, loading its messages.
// Falls back to a global search across all cwd hashes if not found in cwd.
func resolveSession(cwd, prefix string) (session.Info, []session.Message, error) {
	infos, _ := session.List(cwd)
	var match *session.Info
	for i := range infos {
		if strings.HasPrefix(infos[i].ID, prefix) {
			if match != nil {
				return session.Info{}, nil, fmt.Errorf("ambiguous prefix %q — matches multiple sessions; use more characters", prefix)
			}
			m := infos[i]
			match = &m
		}
	}

	// Not found in cwd — search globally.
	if match == nil {
		all, _ := session.ListAll()
		for i := range all {
			if strings.HasPrefix(all[i].ID, prefix) {
				if match != nil {
					return session.Info{}, nil, fmt.Errorf("ambiguous prefix %q — matches multiple sessions; use more characters", prefix)
				}
				m := all[i]
				match = &m
			}
		}
	}

	if match == nil {
		return session.Info{}, nil, fmt.Errorf("no session found with ID prefix %q\n\nRun 'bai sessions' to list available sessions", prefix)
	}

	msgs, err := session.Load(match.Path)
	if err != nil {
		return session.Info{}, nil, fmt.Errorf("load session: %w", err)
	}
	return *match, msgs, nil
}

// renderMarkdown formats session messages as a readable Markdown document.
func renderMarkdown(info session.Info, msgs []session.Message) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# bai Session %s\n\n", info.ID[:min(8, len(info.ID))])
	fmt.Fprintf(&sb, "**Date:** %s  **Turns:** %d  **Directory:** %s\n\n",
		info.UpdatedAt.Format("2006-01-02 15:04"),
		info.Turns,
		info.CWD,
	)

	for _, m := range msgs {
		if m.Role == "system" {
			continue
		}
		sb.WriteString("---\n\n")
		switch m.Role {
		case "user":
			sb.WriteString("**User**\n\n")
			sb.WriteString(m.Content)
			sb.WriteString("\n\n")
		case "assistant":
			if m.Content != "" {
				sb.WriteString("**Assistant**\n\n")
				sb.WriteString(m.Content)
				sb.WriteString("\n\n")
			}
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&sb, "**Tool: %s**\n", tc.Function.Name)
				if tc.Function.Arguments != "" {
					sb.WriteString("```json\n")
					// Pretty-print arguments if they're valid JSON.
					var pretty interface{}
					if json.Unmarshal([]byte(tc.Function.Arguments), &pretty) == nil {
						if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
							sb.WriteString(string(b))
							sb.WriteString("\n")
						} else {
							sb.WriteString(tc.Function.Arguments)
							sb.WriteString("\n")
						}
					} else {
						sb.WriteString(tc.Function.Arguments)
						sb.WriteString("\n")
					}
					sb.WriteString("```\n\n")
				}
			}
		case "tool":
			sb.WriteString("**Result:**\n```\n")
			content := m.Content
			if len(content) > 2000 {
				content = content[:1997] + "..."
			}
			sb.WriteString(content)
			sb.WriteString("\n```\n\n")
		}
	}

	return sb.String()
}

