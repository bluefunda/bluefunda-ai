package abap

import (
	"fmt"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// SlashCommands returns the ABAP commands for the chat TUI.
func SlashCommands(env Env) []tui.SlashCommand {
	nameType := func(args string) (string, string, error) {
		f := strings.Fields(args)
		if len(f) != 2 {
			return "", "", fmt.Errorf("usage: <NAME> <type>  (type: %s)", strings.Join(ObjectTypes, ", "))
		}
		a := objectArgs{Name: strings.ToUpper(f[0]), Type: strings.ToLower(f[1])}
		return a.Name, a.Type, a.requireObject()
	}
	return []tui.SlashCommand{
		{
			Name:        "/system",
			Description: "List SAP systems, or switch the active one  /system <name>",
			Run:         func(args string) (string, error) { return systemCommand(strings.TrimSpace(args)) },
		},
		{
			Name:        "/search",
			Description: "Search SAP objects  /search ZCL_ORDER* [CLAS/OC]",
			Run: func(args string) (string, error) {
				f := strings.Fields(args)
				if len(f) == 0 || len(f) > 2 {
					return "", fmt.Errorf("usage: /search <PATTERN> [type]")
				}
				objType := ""
				if len(f) == 2 {
					objType = f[1]
				}
				c, err := env.NewClient()
				if err != nil {
					return "", err
				}
				objs, err := c.Search(strings.ToUpper(f[0]), objType)
				if err != nil {
					return "", err
				}
				return formatRows(objs, 50), nil
			},
		},
		{
			Name:        "/source",
			Description: "Show an object's source from SAP  /source ZCL_FOO class",
			Run: func(args string) (string, error) {
				name, objType, err := nameType(args)
				if err != nil {
					return "", err
				}
				c, err := env.NewClient()
				if err != nil {
					return "", err
				}
				src, err := c.Source(name, objType)
				if err != nil {
					return "", err
				}
				if src == "" {
					return fmt.Sprintf("No source found for %s (%s).", name, objType), nil
				}
				return fmt.Sprintf("%s (%s) — %d lines\n\n%s", name, objType, strings.Count(src, "\n")+1, src), nil
			},
		},
		{
			Name:        "/activate",
			Description: "Activate a saved object  /activate ZCL_FOO class",
			Run: func(args string) (string, error) {
				name, objType, err := nameType(args)
				if err != nil {
					return "", err
				}
				c, err := env.NewClient()
				if err != nil {
					return "", err
				}
				res, err := c.Activate(name, objType)
				if err != nil {
					return "", err
				}
				if !res.Activated {
					return "", fmt.Errorf("not activated: %s", ActivationSummary(res.Messages))
				}
				return fmt.Sprintf("Activated %s %s.", objType, name), nil
			},
		},
		{
			Name:        "/test",
			Description: "Run ABAP Unit tests  /test ZCL_FOO class",
			Run: func(args string) (string, error) {
				name, objType, err := nameType(args)
				if err != nil {
					return "", err
				}
				c, err := env.NewClient()
				if err != nil {
					return "", err
				}
				res, err := c.UnitTests(name, objType)
				if err != nil {
					return "", err
				}
				return unitTestReport(res), nil
			},
		},
		{
			Name:        "/verify",
			Description: "Syntax-check, activate and unit-test every object deployed this session",
			Run: func(string) (string, error) {
				if env.Tracker == nil || len(env.Tracker.Objects()) == 0 {
					return "No SAP objects deployed in this session yet.", nil
				}
				c, err := env.NewClient()
				if err != nil {
					return "", err
				}
				return Report(Verify(c, env.Tracker.Objects())), nil
			},
		},
	}
}

func systemCommand(name string) (string, error) {
	reg, err := LoadSystems()
	if err != nil {
		return "", err
	}
	if len(reg.Systems) == 0 {
		return fmt.Sprintf("No SAP systems configured — add one with `%s`.", SystemAddHint()), nil
	}
	if name == "" {
		return ListSystems(reg), nil
	}
	sys := reg.Find(name)
	if sys == nil {
		return "", fmt.Errorf("no system named %q\n%s", name, ListSystems(reg))
	}
	reg.Active = sys.ID
	if err := reg.Save(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Active SAP system: %s (%s, client %s)", sys.Name, sys.Host, sys.Client), nil
}

// ListSystems renders the registry with the active system marked.
func ListSystems(reg *Systems) string {
	active := reg.ActiveSystem()
	var b strings.Builder
	for _, s := range reg.Systems {
		mark := " "
		if active != nil && s.ID == active.ID {
			mark = "*"
		}
		fmt.Fprintf(&b, "%s %-16s %-32s client %-4s user %s\n", mark, s.Name, s.Host, s.Client, s.Username)
	}
	return strings.TrimRight(b.String(), "\n")
}
