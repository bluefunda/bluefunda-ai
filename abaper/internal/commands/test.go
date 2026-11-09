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

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run ABAP unit tests for an object",
	Long:  `Run ABAP Unit tests for an object. Exits non-zero when any test fails.`,
	RunE:  runTest,
}

func init() {
	testCmd.Flags().String("type", "class", "Object type: class, program")
	testCmd.Flags().String("name", "", "Object name (required)")
	_ = testCmd.MarkFlagRequired("name")
}

func runTest(cmd *cobra.Command, args []string) error {
	objectType, _ := cmd.Flags().GetString("type")
	objectName, _ := cmd.Flags().GetString("name")

	objectName = strings.ToUpper(objectName)

	c, err := client.NewClient()
	if err != nil {
		return err
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	if outputFmt != "json" {
		fmt.Printf("Running unit tests for %s %s...\n", objectType, objectName)
	}
	result, err := c.RunUnitTests(objectName, objectType)
	if err != nil {
		return fmt.Errorf("unit tests failed: %w", err)
	}

	if outputFmt == "json" {
		output.PrintJSON(result)
	} else {
		printUnitTests(os.Stdout, result)
	}

	if result.Failed > 0 {
		return fmt.Errorf("%d of %d unit test(s) failed", result.Failed, result.TotalTests)
	}
	return nil
}

func printUnitTests(w io.Writer, result *types.UnitTestResult) {
	if result.TotalTests == 0 {
		_, _ = fmt.Fprintln(w, "No unit tests found.")
		return
	}
	for _, tc := range result.TestClasses {
		for _, m := range tc.Methods {
			line := fmt.Sprintf("  %-6s %s->%s", strings.ToUpper(m.Status), tc.Name, m.Name)
			if m.Message != "" {
				line += ": " + m.Message
			}
			_, _ = fmt.Fprintln(w, line)
		}
	}
	_, _ = fmt.Fprintf(w, "%d passed, %d failed, %d total\n", result.Passed, result.Failed, result.TotalTests)
}
