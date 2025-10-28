package main

import (
	"fmt"
	"os"

	"Lumen-SDK/cmd/lumenhub/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
