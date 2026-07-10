package cmd

import (
	"fmt"
	"os"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/internal/native"

	"github.com/spf13/cobra"
)

// NewInstallCommand registers lumen-hostd as a background service (a
// per-user LaunchAgent on macOS, a Task Scheduler entry on Windows, a
// systemd user unit on Linux) and starts it.
func NewInstallCommand() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Host Broker as a background service",
		RunE: func(cmd *cobra.Command, args []string) error {
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable path: %w", err)
			}

			serveArgs := []string{"serve"}
			if configFile != "" {
				serveArgs = append(serveArgs, "--config", configFile)
			}

			if err := native.New().Install(execPath, serveArgs); err != nil {
				return fmt.Errorf("install service: %w", err)
			}
			fmt.Println("Lumen Host Broker installed and started.")
			return nil
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "Path to configuration file passed to the installed service")
	return cmd
}

// NewUninstallCommand removes the background service registration. It does
// not delete the lumen-hostd binary itself.
func NewUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the Host Broker background service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := native.New().Uninstall(); err != nil {
				return fmt.Errorf("uninstall service: %w", err)
			}
			fmt.Println("Lumen Host Broker service removed.")
			return nil
		},
	}
}
