package commands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/pkg/output"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Manage SAP system connections",
	Long: `Add, list, switch, test, and remove SAP system connections.
Credentials are stored at ~/.abaper/systems.json (0600).`,
}

var systemAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new SAP system",
	RunE:  runSystemAdd,
}

var systemListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List configured SAP systems",
	Aliases: []string{"ls"},
	RunE:    runSystemList,
}

var systemUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active SAP system",
	Args:  cobra.ExactArgs(1),
	RunE:  runSystemUse,
}

var systemRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a SAP system",
	Aliases: []string{"rm", "delete"},
	Args:    cobra.ExactArgs(1),
	RunE:    runSystemRemove,
}

var systemTestCmd = &cobra.Command{
	Use:   "test [name]",
	Short: "Test SAP system connection (active system if no name given)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSystemTest,
}

func init() {
	systemAddCmd.Flags().String("name", "", "Display name (defaults to host)")
	systemAddCmd.Flags().String("host", "", "SAP host URL, e.g. https://host:44300 (required)")
	systemAddCmd.Flags().String("client", "100", "SAP client number")
	systemAddCmd.Flags().StringP("username", "u", "", "SAP username (required)")
	systemAddCmd.Flags().StringP("password", "p", "", "SAP password (required)")
	_ = systemAddCmd.MarkFlagRequired("host")
	_ = systemAddCmd.MarkFlagRequired("username")
	_ = systemAddCmd.MarkFlagRequired("password")

	systemCmd.AddCommand(systemAddCmd, systemListCmd, systemUseCmd, systemRemoveCmd, systemTestCmd)
}

func runSystemAdd(cmd *cobra.Command, _ []string) error {
	host, _ := cmd.Flags().GetString("host")
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	name, _ := cmd.Flags().GetString("name")
	sapClient, _ := cmd.Flags().GetString("client")

	cfg, err := config.LoadSystems()
	if err != nil {
		return err
	}

	sys := config.SAPSystem{
		Name:     name,
		Host:     host,
		Client:   sapClient,
		Username: username,
		Password: password,
	}
	id := cfg.AddSystem(sys)
	if err := config.SaveSystems(cfg); err != nil {
		return err
	}

	added := cfg.FindByNameOrID(id)
	output.OK("Added SAP system %q (%s)", added.Name, added.Host)
	if cfg.Active == id {
		fmt.Println("  Set as active system.")
	}
	return nil
}

func runSystemList(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadSystems()
	if err != nil {
		return err
	}
	if len(cfg.Systems) == 0 {
		fmt.Println("No SAP systems configured. Run: abaper system add --help")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "  \tNAME\tHOST\tCLIENT\tUSERNAME")
	for _, s := range cfg.Systems {
		active := "  "
		if s.ID == cfg.Active {
			active = "● "
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", active, s.Name, s.Host, s.Client, s.Username)
	}
	return w.Flush()
}

func runSystemUse(_ *cobra.Command, args []string) error {
	cfg, err := config.LoadSystems()
	if err != nil {
		return err
	}
	s := cfg.FindByNameOrID(args[0])
	if s == nil {
		return fmt.Errorf("system %q not found — run: abaper system list", args[0])
	}
	cfg.Active = s.ID
	if err := config.SaveSystems(cfg); err != nil {
		return err
	}
	output.OK("Active system set to %q (%s)", s.Name, s.Host)
	return nil
}

func runSystemRemove(_ *cobra.Command, args []string) error {
	cfg, err := config.LoadSystems()
	if err != nil {
		return err
	}
	s := cfg.FindByNameOrID(args[0])
	if s == nil {
		return fmt.Errorf("system %q not found — run: abaper system list", args[0])
	}
	name := s.Name
	cfg.RemoveSystem(s.ID)
	if err := config.SaveSystems(cfg); err != nil {
		return err
	}
	output.OK("Removed %q", name)
	return nil
}

func runSystemTest(_ *cobra.Command, args []string) error {
	cfg, err := config.LoadSystems()
	if err != nil {
		return err
	}

	var sys *config.SAPSystem
	if len(args) > 0 {
		sys = cfg.FindByNameOrID(args[0])
		if sys == nil {
			return fmt.Errorf("system %q not found — run: abaper system list", args[0])
		}
	} else {
		sys = cfg.GetActive()
		if sys == nil {
			return fmt.Errorf("no SAP systems configured — run: abaper system add --help")
		}
	}

	fmt.Printf("Testing connection to %s (%s)...\n", sys.Name, sys.Host)
	c, err := client.NewClient()
	if err != nil {
		return err
	}
	if err := c.SystemConnect(context.Background(), sys.Host, sys.Client, sys.Username, sys.Password); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	output.OK("Connection successful!")
	return nil
}
