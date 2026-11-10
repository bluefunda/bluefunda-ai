package cmd

import (
	"github.com/spf13/cobra"

	trmpb "github.com/bluefunda/bluefunda-ai/api/proto/trm"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

// completeCRID fetches live CR IDs from the BFF for use as positional arg completions.
// Returns empty on any error so the shell falls back to filename completion gracefully.
func completeCRID(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	conn, _, err := requestsConn()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.ListChangeRequests(ctx, &trmpb.ListChangeRequestsRequest{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	completions := make([]string, 0, len(resp.ChangeRequests))
	for _, cr := range resp.ChangeRequests {
		completions = append(completions, cr.Id+"\t"+cr.Description)
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// staticValues returns a completion function for a fixed set of allowed values.
func staticValues(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}
