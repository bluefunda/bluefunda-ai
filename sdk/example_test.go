package sdk_test

import (
	"context"
	"fmt"

	"github.com/bluefunda/bluefunda-ai/sdk"
)

func Example() {
	client := sdk.NewClient(sdk.Options{
		Model:       "auto",
		AutoApprove: true,
		MaxTurns:    5,
	})
	defer func() { _ = client.Stop() }()

	events, err := client.Send(context.Background(), "list files in current directory")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	for ev := range events {
		switch ev.Type {
		case "text":
			fmt.Print(ev.Text)
		case "tool_use":
			fmt.Printf("[tool: %s]\n", ev.Name)
		case "error":
			fmt.Printf("error: %s\n", ev.Error)
		case "result":
			fmt.Printf("\n--- done (%s) ---\n", ev.StopReason)
		}
	}
}
