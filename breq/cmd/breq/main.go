// Command breq is the BlueRequests CLI: change requests, events, RPC and A4H
// sessions on the TRM platform, built on the bai platform core.
package main

import (
	"os"

	"github.com/bluefunda/bluefunda-ai/internal/cmd"
)

func main() {
	if err := cmd.ExecuteBREQ(); err != nil {
		os.Exit(1)
	}
}
