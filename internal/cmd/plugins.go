package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/plugins"
)

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage bai plugins",
}

var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List loaded plugins",
	RunE:  runPluginsList,
}

func init() {
	pluginsCmd.AddCommand(pluginsListCmd)
}

func runPluginsList(_ *cobra.Command, _ []string) error {
	m := plugins.NewManager(".")
	all := m.All()
	if len(all) == 0 {
		fmt.Println("No plugins loaded.")
		fmt.Println()
		fmt.Println("Add plugins to .bai/plugins/<name>/plugin.yaml or run `bai init` for a scaffold.")
		return nil
	}
	fmt.Printf("%-20s  %-8s  %-40s  %s\n", "NAME", "APPROVAL", "DESCRIPTION", "SOURCE")
	fmt.Printf("%-20s  %-8s  %-40s  %s\n", "----", "--------", "-----------", "------")
	for _, p := range all {
		approval := p.Manifest.Approval
		if approval == "" {
			approval = "auto"
		}
		desc := p.Manifest.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Printf("%-20s  %-8s  %-40s  %s\n",
			p.Manifest.Name, approval, desc, p.SourcePath)
	}
	return nil
}
