// Command abaper is the ABAPer CLI: the ABAP agent and SAP tooling built on
// the bai platform core, plus the ABAPer REST server (`abaper serve`).
package main

import (
	"os"

	"github.com/bluefunda/bluefunda-ai/abaper/internal/serve"
	"github.com/bluefunda/bluefunda-ai/internal/cmd"
)

func main() {
	if err := cmd.ExecuteABAPer(serve.NewCommand(), serve.NewLSPCommand()); err != nil {
		os.Exit(1)
	}
}
