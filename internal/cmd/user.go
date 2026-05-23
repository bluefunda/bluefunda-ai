package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var userCmd = &cobra.Command{
	Use:    "user",
	Short:  "User account operations",
	Hidden: true,
}

// --- user info ---

var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current user information",
	RunE:  runUserInfo,
}

func runUserInfo(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetUserInfo(ctx, &pb.GetUserInfoRequest{})
	if err != nil {
		return fmt.Errorf("get user info: %w", err)
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Name", resp.GetName()},
		{"Email", resp.GetEmail()},
		{"Username", resp.GetPreferredUsername()},
		{"Verified", fmt.Sprintf("%t", resp.GetEmailVerified())},
		{"Sub", resp.GetSub()},
	}
	p.Table(headers, rows)
	return nil
}

// --- user settings ---

var userSettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Show user platform settings",
	RunE:  runUserSettings,
}

func runUserSettings(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetUserSettings(ctx, &pb.GetUserSettingsRequest{})
	if err != nil {
		return fmt.Errorf("get settings: %w", err)
	}

	printer(cfg).ProtoJSON(resp)
	return nil
}

func init() {
	userCmd.AddCommand(userInfoCmd, userSettingsCmd)
}
