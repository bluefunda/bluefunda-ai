package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/pkg/output"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy and activate an ABAP object on the target system",
	Long: `Upload source code and activate an ABAP object. This performs
a save followed by activation, matching the workflow in abaper-editor.`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().String("type", "program", "Object type: program, class, interface")
	deployCmd.Flags().String("name", "", "Object name (required)")
	deployCmd.Flags().String("source-file", "", "Path to source file (required)")
	_ = deployCmd.MarkFlagRequired("name")
	_ = deployCmd.MarkFlagRequired("source-file")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	objectType, _ := cmd.Flags().GetString("type")
	objectName, _ := cmd.Flags().GetString("name")
	sourceFile, _ := cmd.Flags().GetString("source-file")

	objectName = strings.ToUpper(objectName)

	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}
	source := string(data)

	c, err := client.NewClient()
	if err != nil {
		return err
	}

	// Step 1: Save
	fmt.Printf("Uploading %s %s...\n", objectType, objectName)
	if err := c.CreateObject(objectName, objectType, source); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	// Step 2: Activate
	fmt.Printf("Activating %s %s...\n", objectType, objectName)
	result, err := c.Activate(objectName, objectType)
	if err != nil {
		return fmt.Errorf("activation failed: %w", err)
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	if !result.Activated {
		if outputFmt == "json" {
			output.PrintJSON(result)
		}
		return fmt.Errorf("activation failed: %s", formatActivationMessages(result.Messages))
	}

	if outputFmt == "json" {
		output.PrintJSON(result)
	} else {
		output.OK("Successfully deployed and activated %s %s.", objectType, objectName)
	}

	return nil
}

func formatActivationMessages(messages []client.ActivateMessage) string {
	if len(messages) == 0 {
		return "no details returned"
	}
	parts := make([]string, 0, len(messages))
	for _, m := range messages {
		if m.Line > 0 {
			parts = append(parts, fmt.Sprintf("[%s] line %d: %s", m.Severity, m.Line, m.Text))
		} else {
			parts = append(parts, fmt.Sprintf("[%s] %s", m.Severity, m.Text))
		}
	}
	return strings.Join(parts, "; ")
}
