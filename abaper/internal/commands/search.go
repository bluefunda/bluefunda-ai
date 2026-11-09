package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/pkg/output"
)

var searchCmd = &cobra.Command{
	Use:   "search <pattern>",
	Short: "Search ABAP objects by name pattern",
	Long: `Search for ABAP objects matching a name pattern.
Use * as a wildcard, e.g. ZMY* to find all objects starting with ZMY.`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringP("type", "t", "", "Filter by object type (e.g. PROG/P, CLAS/OC, INTF/OI)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	pattern := args[0]
	objectType, _ := cmd.Flags().GetString("type")

	c, err := client.NewClient()
	if err != nil {
		return err
	}

	objects, err := c.SearchObjects(pattern, objectType)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	if outputFmt == "json" {
		output.PrintJSON(objects)
		return nil
	}

	if len(objects) == 0 {
		fmt.Printf("No objects found matching %q.\n", pattern)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TYPE\tNAME\tDESCRIPTION")
	for _, obj := range objects {
		objType, _ := obj["type"].(string)
		objName, _ := obj["name"].(string)
		desc, _ := obj["description"].(string)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", objType, objName, desc)
	}
	return w.Flush()
}
