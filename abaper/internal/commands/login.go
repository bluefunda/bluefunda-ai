package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/pkg/output"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in (delegates to bai login — shared auth for AI features)",
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.ClearTokens(); err != nil {
			return fmt.Errorf("logout: %w", err)
		}
		printLoginSuccess("Logged out successfully.")
		go client.Track("logout", nil)
		return nil
	},
}

// runLogin delegates to `bai login` so that abaper and bai share a single
// auth flow and token store. abaper ai code already uses the bai SDK, so
// a single login is sufficient for all abaper commands that call the AI backend.
func runLogin(cmd *cobra.Command, args []string) error {
	bai, err := exec.LookPath("bai")
	if err != nil {
		return fmt.Errorf("bai not found in PATH — install bai to authenticate: %w", err)
	}
	c := exec.Command(bai, "login")
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func printLoginSuccess(msg string) {
	fmt.Println()
	output.OK(msg)
}
