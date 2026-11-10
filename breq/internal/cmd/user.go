package cmd

import (
	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User commands",
}

var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current user info",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		resp, err := conn.Client.GetUserInfo(ctx, &pb.GetUserInfoRequest{})
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.GetUserInfo(ctx, &pb.GetUserInfoRequest{})
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

func init() {
	userCmd.AddCommand(userInfoCmd)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
