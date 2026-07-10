package agent_test

import (
	"fmt"

	"github.com/bluefunda/bluefunda-ai/sdk/agent"
)

// Example shows the minimal usage of the embedded agent runner.
// The runner connects to the bai BFF using credentials from ~/.bai/config.yaml.
func Example() {
	runner := agent.New(agent.Options{
		Model:    "auto",
		MaxTurns: 5,
		OnEvent: func(ev agent.Event) {
			switch ev.Type {
			case "text":
				fmt.Print(ev.Text)
			case "tool_use":
				fmt.Printf("[tool: %s]\n", ev.ToolName)
			case "result":
				fmt.Printf("\n--- done (%s) ---\n", ev.StopReason)
			}
		},
	})
	defer func() { _ = runner.Close() }()

	// Run requires live credentials; this example just shows creation.
	_ = runner
	fmt.Println("runner created")
	// Output: runner created
}

// ExampleRunner_WithSystemPrompt shows injecting a system prompt.
func ExampleRunner_WithSystemPrompt() {
	runner := agent.New(agent.Options{Model: "auto"}).
		WithSystemPrompt("You are a concise assistant. Answer in one sentence.")
	_ = runner
	fmt.Println("system prompt set")
	// Output: system prompt set
}

// ExampleDefaultExecute shows routing a tool call to the default executor.
func ExampleDefaultExecute() {
	res, err := agent.DefaultExecute(agent.ToolCall{
		Name:      "bash",
		Arguments: `{"command":"echo hello"}`,
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(res.Output)
	// Output: hello
}

// ExampleRunner_Continue shows that History starts empty and accumulates after
// each Run / Continue call. This example does not make live backend calls.
func ExampleRunner_Continue() {
	runner := agent.New(agent.Options{Model: "auto"})

	// History is empty before any prompts are submitted.
	fmt.Printf("history has %d messages\n", len(runner.History()))
	// Output: history has 0 messages
}
