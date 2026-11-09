package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/pkg/output"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate ABAP objects on the target system",
	Long: `Create ABAP objects (programs, classes, interfaces, etc.) on the
connected SAP system via ABAPer APIs.`,
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().String("type", "program", "Object type: program, class, interface")
	generateCmd.Flags().String("name", "", "Object name (required)")
	generateCmd.Flags().String("source-file", "", "Path to source file (reads from file instead of stdin)")
	_ = generateCmd.MarkFlagRequired("name")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	objectType, _ := cmd.Flags().GetString("type")
	objectName, _ := cmd.Flags().GetString("name")
	sourceFile, _ := cmd.Flags().GetString("source-file")

	objectName = strings.ToUpper(objectName)

	var source string
	if sourceFile != "" {
		data, err := os.ReadFile(sourceFile)
		if err != nil {
			return fmt.Errorf("read source file: %w", err)
		}
		source = string(data)
	} else {
		source = defaultSource(objectType, objectName)
	}

	c, err := client.NewClient()
	if err != nil {
		return err
	}

	fmt.Printf("Creating %s %s...\n", objectType, objectName)
	if err := c.CreateObject(objectName, objectType, source); err != nil {
		return fmt.Errorf("generate failed: %w", err)
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	if outputFmt == "json" {
		output.PrintJSON(map[string]string{
			"status": "created",
			"type":   objectType,
			"name":   objectName,
		})
	} else {
		output.OK("Successfully created %s %s.", objectType, objectName)
	}

	return nil
}

func defaultSource(objectType, name string) string {
	switch objectType {
	case "class":
		return fmt.Sprintf("CLASS %s DEFINITION PUBLIC.\nENDCLASS.\n\nCLASS %s IMPLEMENTATION.\nENDCLASS.", name, name)
	case "interface":
		return fmt.Sprintf("INTERFACE %s PUBLIC.\nENDINTERFACE.", name)
	default:
		return fmt.Sprintf("REPORT %s.", name)
	}
}
