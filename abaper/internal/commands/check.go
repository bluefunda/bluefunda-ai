package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/pkg/output"
	"github.com/bluefunda/abaper/types"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Syntax-check ABAP source against the target system",
	Long: `Run the SAP syntax check on local source without saving or activating it.
Exits non-zero when the check reports errors.`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().String("type", "program", "Object type: program, class, interface")
	checkCmd.Flags().String("name", "", "Object name (required)")
	checkCmd.Flags().String("source-file", "", "Path to source file (required)")
	_ = checkCmd.MarkFlagRequired("name")
	_ = checkCmd.MarkFlagRequired("source-file")
}

func runCheck(cmd *cobra.Command, args []string) error {
	objectType, _ := cmd.Flags().GetString("type")
	objectName, _ := cmd.Flags().GetString("name")
	sourceFile, _ := cmd.Flags().GetString("source-file")

	objectName = strings.ToUpper(objectName)

	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	c, err := client.NewClient()
	if err != nil {
		return err
	}

	result, err := c.SyntaxCheck(objectName, objectType, string(data))
	if err != nil {
		return fmt.Errorf("syntax check failed: %w", err)
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	if outputFmt == "json" {
		output.PrintJSON(result)
	} else {
		printSyntaxCheck(os.Stdout, objectType, objectName, result)
	}

	if n := countSyntaxErrors(result); n > 0 {
		return fmt.Errorf("%d syntax error(s) in %s %s", n, objectType, objectName)
	}
	return nil
}

func countSyntaxErrors(result *types.SyntaxCheckResult) int {
	n := 0
	for _, m := range result.Messages {
		if m.Severity == "error" {
			n++
		}
	}
	return n
}

func printSyntaxCheck(w io.Writer, objectType, objectName string, result *types.SyntaxCheckResult) {
	if len(result.Messages) == 0 {
		_, _ = fmt.Fprintf(w, "No syntax errors in %s %s.\n", objectType, objectName)
		return
	}
	for _, m := range result.Messages {
		if m.Line > 0 {
			_, _ = fmt.Fprintf(w, "%s:%d:%d: %s: %s\n", objectName, m.Line, m.Column, m.Severity, m.Text)
		} else {
			_, _ = fmt.Fprintf(w, "%s: %s: %s\n", objectName, m.Severity, m.Text)
		}
	}
}
