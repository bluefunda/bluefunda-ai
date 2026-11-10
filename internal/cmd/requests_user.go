package cmd

import (
	"github.com/spf13/cobra"

	trmpb "github.com/bluefunda/bluefunda-ai/api/proto/trm"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// newRequestsUserCmd builds the `user` command tree. A constructor (not package
// vars) because the same tree mounts under both `breq` and `bai requests`.
func newRequestsUserCmd() *cobra.Command {
	var userCmd, userInfoCmd *cobra.Command
	userCmd = &cobra.Command{
		Use:   "user",
		Short: "User commands",
	}
	userInfoCmd = &cobra.Command{
		Use:   "info",
		Short: "Show current user info",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, cfg, err := requestsConn()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			p := printer(cfg)
			ctx, cancel := caigrpc.ContextWithTimeout()
			defer cancel()

			resp, err := conn.Client.GetUserInfo(ctx, &trmpb.GetUserInfoRequest{})
			if err != nil {
				if caigrpc.IsAuthError(err) {
					if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
						return reAuthErr
					}
					resp, err = conn.Client.GetUserInfo(ctx, &trmpb.GetUserInfoRequest{})
				}
				if err != nil {
					return err
				}
			}

			p.Table(
				[]string{"FIELD", "VALUE"},
				[][]string{
					{"sub", resp.Sub},
					{"name", resp.Name},
					{"email", resp.Email},
					{"username", resp.PreferredUsername},
					{"email_verified", boolStr(resp.EmailVerified)},
				},
			)
			return nil
		},
	}
	userCmd.AddCommand(userInfoCmd)
	return userCmd
}
