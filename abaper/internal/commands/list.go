package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/pkg/output"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List ABAP objects or package contents",
}

var listObjectsCmd = &cobra.Command{
	Use:   "objects",
	Short: "List ABAP objects, optionally filtered by package or type",
	RunE:  runListObjects,
}

var listPackagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "List contents of an ABAP package",
	RunE:  runListPackages,
}

func init() {
	listObjectsCmd.Flags().String("package", "", "Filter by package name")
	listObjectsCmd.Flags().String("type", "", "Filter by object type")

	listPackagesCmd.Flags().String("name", "", "Package name (required)")
	_ = listPackagesCmd.MarkFlagRequired("name")

	listCmd.AddCommand(listObjectsCmd)
	listCmd.AddCommand(listPackagesCmd)
}

func runListObjects(cmd *cobra.Command, args []string) error {
	packageName, _ := cmd.Flags().GetString("package")
	objectType, _ := cmd.Flags().GetString("type")

	c, err := client.NewClient()
	if err != nil {
		return err
	}

	objects, err := c.ListObjects(packageName, objectType)
	if err != nil {
		return fmt.Errorf("list objects failed: %w", err)
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	return printObjectList(os.Stdout, objects, outputFmt, "No objects found.")
}

func runListPackages(cmd *cobra.Command, args []string) error {
	packageName, _ := cmd.Flags().GetString("name")

	c, err := client.NewClient()
	if err != nil {
		return err
	}

	objects, err := c.PackageContents(packageName)
	if err != nil {
		return fmt.Errorf("package contents failed: %w", err)
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	return printObjectList(os.Stdout, objects, outputFmt, "No objects found in package.")
}

// printObjectList renders a list of ABAP objects (as returned by the ABAPer
// API — keyed by "type" and "name") in text or JSON form.
func printObjectList(w io.Writer, objects []map[string]any, outputFmt, emptyMessage string) error {
	if outputFmt == "json" {
		output.PrintJSON(objects)
		return nil
	}

	if len(objects) == 0 {
		_, err := fmt.Fprintln(w, emptyMessage)
		return err
	}

	for _, obj := range objects {
		parts := []string{}
		if t, ok := obj["type"]; ok {
			parts = append(parts, fmt.Sprintf("%v", t))
		}
		if n, ok := obj["name"]; ok {
			parts = append(parts, fmt.Sprintf("%v", n))
		}
		if _, err := fmt.Fprintln(w, strings.Join(parts, "\t")); err != nil {
			return err
		}
	}

	return nil
}
