package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
)

var rpcTimeoutMs int64

var rpcCmd = &cobra.Command{
	Use:   "rpc",
	Short: "Low-level RPC commands",
}

var rpcRequestCmd = &cobra.Command{
	Use:   "request <subject> <data>",
	Short: "Perform a request-reply on a realm-scoped subject",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		subject := args[0]
		data := []byte(args[1])

		resp, err := conn.Client.RequestReply(ctx, &pb.RequestReplyRequest{
			Subject:   subject,
			Data:      data,
			TimeoutMs: rpcTimeoutMs,
		})
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.RequestReply(ctx, &pb.RequestReplyRequest{
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

func init() {
	rpcRequestCmd.Flags().Int64Var(&rpcTimeoutMs, "timeout", 5000, "Request timeout in milliseconds")
	rpcCmd.AddCommand(rpcRequestCmd)
}
