package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server management",
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

// --- mcp user ---

var mcpUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Show user's MCP server subscriptions",
	RunE:  runMCPUser,
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

// --- mcp select ---

var mcpSelectCmd = &cobra.Command{
	Use:   "select <name>",
	Short: "Select/subscribe to an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPSelect,
}

func runMCPSelect(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.SelectMcp(ctx, &pb.SelectMcpRequest{
		McpInfo: &pb.MCPInfo{Name: args[0]},
	})
	if err != nil {
		return fmt.Errorf("select mcp: %w", err)
	}

	p := printer(cfg)
	if resp.GetError() != "" {
		return fmt.Errorf("select mcp: %s", resp.GetError())
	}
	if resp.GetSuccess() {
		p.Success(fmt.Sprintf("Selected MCP server: %s", args[0]))
	}
	return nil
}

func init() {
	mcpCmd.AddCommand(mcpListCmd, mcpUserCmd, mcpSelectCmd)
}
