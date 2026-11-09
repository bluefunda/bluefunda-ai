package commands

import (
	"fmt"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/spf13/cobra"
)

var signupCmd = &cobra.Command{
	Use:   "signup",
	Short: "Open BlueFunda signup page in your browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		signupURL := "https://bluefunda.com/signup?utm_source=cli&utm_medium=command&utm_campaign=signup"
		fmt.Printf("Opening signup page: %s\n", signupURL)
		if err := openBrowser(signupURL); err != nil {
			fmt.Printf("Could not open browser. Please visit the URL above manually.\n")
		}
		go client.Track("signup_opened", nil)
		return nil
	},
}
