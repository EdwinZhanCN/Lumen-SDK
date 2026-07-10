package cmd

import (
	"fmt"
	"runtime"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/service"

	"github.com/spf13/cobra"
)

// NewVersionCommand prints build version information.
func NewVersionCommand(build service.BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Lumen Host Broker %s\n", build.Version)
			fmt.Printf("Commit: %s\n", build.Commit)
			fmt.Printf("Built: %s\n", build.BuildTime)
			fmt.Printf("Go: %s\n", runtime.Version())
			fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
