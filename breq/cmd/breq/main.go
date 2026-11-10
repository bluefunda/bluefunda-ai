package main

import (
	"os"

	"github.com/bluefunda/bluerequests/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
