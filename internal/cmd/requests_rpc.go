package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	trmpb "github.com/bluefunda/bluefunda-ai/api/proto/trm"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

var rpcTimeoutMs int64

// newRequestsRPCCmd builds the `rpc` command tree. A constructor (not package
// vars) because the same tree mounts under both `breq` and `bai requests`.
func newRequestsRPCCmd() *cobra.Command {
	var rpcCmd, rpcRequestCmd *cobra.Command
	rpcCmd = &cobra.Command{
		Use:   "rpc",
		Short: "Low-level RPC commands",
	}
	rpcRequestCmd = &cobra.Command{
		Use:   "request <subject> <data>",
		Short: "Perform a request-reply on a realm-scoped subject",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, cfg, err := requestsConn()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			p := printer(cfg)
			ctx, cancel := caigrpc.ContextWithTimeout()
			defer cancel()

			subject := args[0]
			data := []byte(args[1])

			resp, err := conn.Client.RequestReply(ctx, &trmpb.RequestReplyRequest{
				Subject:   subject,
				Data:      data,
				TimeoutMs: rpcTimeoutMs,
			})
			if err != nil {
				if caigrpc.IsAuthError(err) {
					if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
						return reAuthErr
					}
					resp, err = conn.Client.RequestReply(ctx, &trmpb.RequestReplyRequest{
						Subject:   subject,
						Data:      data,
						TimeoutMs: rpcTimeoutMs,
					})
				}
				if err != nil {
					return err
				}
			}

			fmt.Printf("%s\n", string(resp.Data))
			return nil
		},
	}
	rpcRequestCmd.Flags().Int64Var(&rpcTimeoutMs, "timeout", 5000, "Request timeout in milliseconds")
	rpcCmd.AddCommand(rpcRequestCmd)
	return rpcCmd
}
