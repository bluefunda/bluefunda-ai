package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	trmpb "github.com/bluefunda/bluefunda-ai/api/proto/trm"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

// newRequestsEventsCmd builds the `events` command tree. A constructor (not package
// vars) because the same tree mounts under both `breq` and `bai requests`.
func newRequestsEventsCmd() *cobra.Command {
	var eventsCmd, eventsSubscribeCmd, eventsPublishCmd *cobra.Command
	eventsCmd = &cobra.Command{
		Use:   "events",
		Short: "Event commands (subscribe, publish)",
	}
	eventsSubscribeCmd = &cobra.Command{
		Use:   "subscribe [pattern]",
		Short: "Subscribe to realm-scoped events (streams until Ctrl-C)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, cfg, err := requestsConn()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			p := printer(cfg)

			pattern := ">"
			if len(args) > 0 {
				pattern = args[0]
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sig
				cancel()
			}()

			stream, err := conn.Client.SubscribeEvents(ctx, &trmpb.SubscribeEventsRequest{
				SubjectPattern: pattern,
			})
			if err != nil {
				return fmt.Errorf("subscribe: %w", err)
			}

			p.Info(fmt.Sprintf("Subscribed to pattern: %s (Ctrl-C to stop)", pattern))

			for {
				event, err := stream.Recv()
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return fmt.Errorf("stream: %w", err)
				}
				fmt.Printf("[%s] %s: %s\n", event.Timestamp, event.Subject, string(event.Data))
			}
		},
	}
	eventsPublishCmd = &cobra.Command{
		Use:   "publish <subject> <data>",
		Short: "Publish an event to a realm-scoped subject",
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

			_, err = conn.Client.PublishEvent(ctx, &trmpb.PublishEventRequest{
				Subject: subject,
				Data:    data,
			})
			if err != nil {
				if caigrpc.IsAuthError(err) {
					if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
						return reAuthErr
					}
					_, err = conn.Client.PublishEvent(ctx, &trmpb.PublishEventRequest{
						Subject: subject,
						Data:    data,
					})
				}
				if err != nil {
					return err
				}
			}

			p.Success(fmt.Sprintf("Published to %s", subject))
			return nil
		},
	}
	eventsCmd.AddCommand(eventsSubscribeCmd)
	eventsCmd.AddCommand(eventsPublishCmd)
	return eventsCmd
}
