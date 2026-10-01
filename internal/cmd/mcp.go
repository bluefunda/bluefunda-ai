package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage tool integrations",
}

// --- mcp list ---

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available MCP servers",
	RunE:  runMCPList,
}

func runMCPList(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetMcpInfo(ctx, &pb.GetMcpInfoRequest{})
	if err != nil {
		return fmt.Errorf("get mcp info: %w", err)
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"ID", "NAME", "TYPE", "AVAILABLE", "DESCRIPTION"}
	rows := make([][]string, 0, len(resp.GetMcpServers()))
	for _, s := range resp.GetMcpServers() {
		rows = append(rows, []string{
			fmt.Sprintf("%d", s.GetServerId()),
			s.GetName(),
			s.GetType(),
			strconv.FormatBool(s.GetIsAvailable()),
			truncate(s.GetShortDescription(), 50),
		})
	}
	p.Table(headers, rows)
	return nil
}

// --- mcp add ---

var mcpAddCmd = &cobra.Command{
	Use:   "add <name> [name...]",
	Short: "Activate one or more MCP server integrations",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMCPAdd,
}

func runMCPAdd(cmd *cobra.Command, args []string) error {
	return runMCPSelect(args, true, "Activated")
}

// --- mcp remove ---

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove <name> [name...]",
	Short: "Deactivate one or more MCP server integrations",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMCPRemove,
}

func runMCPRemove(cmd *cobra.Command, args []string) error {
	return runMCPSelect(args, false, "Removed")
}

// mcpSelectResult is the outcome of toggling one server's subscription.
type mcpSelectResult struct {
	name    string
	success bool
	err     error
}

// selectMcpServers toggles subscribe for each name via selectFn, continuing
// past individual failures so one bad name doesn't block the rest. Extracted
// from runMCPSelect so the aggregation/continue-on-error behavior is
// independently testable without a gRPC connection.
func selectMcpServers(names []string, subscribe bool, selectFn func(name string, subscribe bool) error) []mcpSelectResult {
	results := make([]mcpSelectResult, 0, len(names))
	for _, name := range names {
		err := selectFn(name, subscribe)
		results = append(results, mcpSelectResult{name: name, success: err == nil, err: err})
	}
	return results
}

// runMCPSelect activates or deactivates each named MCP server integration.
// Each name is an independent SelectMcp call (the backend already models a
// user's subscriptions as a set — repeated calls toggle individual members
// rather than replacing the whole set), so selecting multiple servers in one
// invocation is just multiple calls, same as a multi-select UI would issue.
func runMCPSelect(names []string, subscribe bool, verb string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := printer(cfg)
	results := selectMcpServers(names, subscribe, func(name string, sub bool) error {
		ctx, cancel := caigrpc.ContextWithTimeout()
		defer cancel()
		resp, err := conn.Client.SelectMcp(ctx, &pb.SelectMcpRequest{
			McpInfo: &pb.MCPInfo{Name: name, Subscribe: sub},
		})
		if err != nil {
			return err
		}
		if resp.GetError() != "" {
			return fmt.Errorf("%s", resp.GetError())
		}
		if !resp.GetSuccess() {
			return fmt.Errorf("server did not confirm success")
		}
		return nil
	})

	var failed []string
	for _, r := range results {
		if r.success {
			p.Success(fmt.Sprintf("%s MCP server: %s", verb, r.name))
		} else {
			p.Error(fmt.Sprintf("%s failed for %s: %v", verb, r.name, r.err))
			failed = append(failed, r.name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d server(s) failed: %s", len(failed), len(names), strings.Join(failed, ", "))
	}
	return nil
}

// --- mcp user (hidden, backward compat) ---

var mcpUserCmd = &cobra.Command{
	Use:    "user",
	Short:  "Show user's MCP subscriptions",
	Hidden: true,
	RunE:   runMCPUser,
}

func runMCPUser(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetMcpForUser(ctx, &pb.GetMcpForUserRequest{})
	if err != nil {
		return fmt.Errorf("get mcp for user: %w", err)
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"ID", "NAME", "TYPE", "SUBSCRIBED"}
	rows := make([][]string, 0, len(resp.GetMcpServers()))
	for _, s := range resp.GetMcpServers() {
		rows = append(rows, []string{
			fmt.Sprintf("%d", s.GetServerId()),
			s.GetName(),
			s.GetType(),
			strconv.FormatBool(s.GetSubscribe()),
		})
	}
	p.Table(headers, rows)
	return nil
}

// --- mcp select (hidden alias for mcp add) ---

var mcpSelectCmd = &cobra.Command{
	Use:    "select <name> [name...]",
	Short:  "Select one or more MCP servers (use `mcp add` instead)",
	Args:   cobra.MinimumNArgs(1),
	Hidden: true,
	RunE:   runMCPAdd,
}

func init() {
	mcpCmd.AddCommand(mcpListCmd, mcpAddCmd, mcpRemoveCmd, mcpUserCmd, mcpSelectCmd)
}
