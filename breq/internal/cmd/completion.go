package cmd

import (
	"os"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for requests.

Bash:
  source <(requests completion bash)

  # Persist across sessions (Linux):
  requests completion bash > /etc/bash_completion.d/requests
  # Persist across sessions (macOS with Homebrew):
  requests completion bash > $(brew --prefix)/etc/bash_completion.d/requests

Zsh:
  # Enable completion if not already done:
  echo "autoload -U compinit; compinit" >> ~/.zshrc

  # Persist across sessions:
  requests completion zsh > "${fpath[1]}/_requests"
  # Start a new shell for changes to take effect.

Fish:
  requests completion fish | source

  # Persist across sessions:
  requests completion fish > ~/.config/fish/completions/requests.fish

PowerShell:
  requests completion powershell | Out-String | Invoke-Expression

  # Persist across sessions — add to your PowerShell profile:
  requests completion powershell > requests.ps1 && . requests.ps1
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// completeCRID fetches live CR IDs from the BFF for use as positional arg completions.
// Returns empty on any error so the shell falls back to filename completion gracefully.
func completeCRID(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	conn, _, err := bffConn()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := trmgrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.ListChangeRequests(ctx, &pb.ListChangeRequestsRequest{})
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

func init() {
	rootCmd.AddCommand(completionCmd)

	// --output flag on root applies to all subcommands.
	_ = rootCmd.RegisterFlagCompletionFunc("output", staticValues("table", "json", "quiet"))
}
