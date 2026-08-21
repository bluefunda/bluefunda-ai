package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and set configuration",
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all configuration values",
	RunE:  runConfigList,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>  or  set key=value",
	Short: "Set a configuration value",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runConfigSet,
}

var configUseProfileCmd = &cobra.Command{
	Use:   "use-profile <name>",
	Short: "Set the persistent default backend profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigUseProfile,
}

func init() {
	configCmd.AddCommand(configListCmd, configGetCmd, configSetCmd, configUseProfileCmd)
}

type configKeyDef struct {
	get func(*config.Config) string
	set func(*config.Config, string)
}

var configKeys = map[string]configKeyDef{
	"model": {
		get: func(c *config.Config) string { return c.Defaults.Model },
		set: func(c *config.Config, v string) { c.Defaults.Model = v },
	},
	"output": {
		get: func(c *config.Config) string { return c.Defaults.Output },
		set: func(c *config.Config, v string) { c.Defaults.Output = v },
	},
	"endpoint": {
		get: func(c *config.Config) string { return c.BFFURL },
		set: func(c *config.Config, v string) { c.BFFURL = v },
	},
}

func runConfigList(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	p := printer(cfg)
	headers := []string{"KEY", "VALUE"}
	rows := [][]string{
		{"model", cfg.Defaults.Model},
		{"output", cfg.Defaults.Output},
		{"endpoint", cfg.BFFURL},
	}
	defaultProfile := cfg.DefaultProfile
	if defaultProfile == "" {
		defaultProfile = "(none)"
	}
	rows = append(rows, []string{"default_profile", defaultProfile})
	if names := cfg.ProfileNames(); len(names) > 0 {
		rows = append(rows, []string{"profiles", strings.Join(names, ", ")})
	}
	p.Table(headers, rows)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := strings.ToLower(args[0])
	cfg := loadConfig()
	k, ok := configKeys[key]
	if !ok {
		return fmt.Errorf("unknown key %q — valid keys: model, output, endpoint", key)
	}
	fmt.Println(k.get(cfg))
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	var key, value string
	if len(args) == 1 {
		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("usage: bai config set key=value  or  bai config set key value")
		}
		key, value = strings.ToLower(parts[0]), parts[1]
	} else {
		key, value = strings.ToLower(args[0]), args[1]
	}
	cfg := loadConfig()
	k, ok := configKeys[key]
	if !ok {
		return fmt.Errorf("unknown key %q — valid keys: model, output, endpoint", key)
	}
	k.set(cfg, value)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	printer(cfg).Success(fmt.Sprintf("Set %s = %s", key, value))
	return nil
}

func runConfigUseProfile(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	return useProfile(cfg, args[0], printer(cfg))
}

// useProfile validates name against cfg.Profiles and, if valid, sets it as
// the persistent default profile. Split out from runConfigUseProfile so it's
// testable without going through cobra/loadConfig.
func useProfile(cfg *config.Config, name string, p *ui.Printer) error {
	if _, ok := cfg.Profiles[name]; !ok {
		available := cfg.ProfileNames()
		if len(available) == 0 {
			return fmt.Errorf("no profiles are configured — add a `profiles:` block to ~/.bai/config.yaml first")
		}
		return fmt.Errorf("unknown profile %q — available: %s", name, strings.Join(available, ", "))
	}

	cfg.DefaultProfile = name
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	p.Success(fmt.Sprintf("Default profile set to %q", name))
	return nil
}
