package cmd

import (
	"fmt"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/internal/native"

	"github.com/spf13/cobra"
)

// NewStartCommand starts the already-installed background service.
func NewStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the installed Host Broker service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := native.New().Start(); err != nil {
				return fmt.Errorf("start service: %w", err)
			}
			fmt.Println("Lumen Host Broker started.")
			return nil
		},
	}
}

// NewStopCommand stops the installed background service without
// uninstalling it.
func NewStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the installed Host Broker service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := native.New().Stop(); err != nil {
				return fmt.Errorf("stop service: %w", err)
			}
			fmt.Println("Lumen Host Broker stopped.")
			return nil
		},
	}
}

// NewStatusCommand reports whether the background service is installed and
// running.
func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the Host Broker service is installed and running",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := native.New().Status()
			if err != nil {
				return fmt.Errorf("query service status: %w", err)
			}
			fmt.Printf("Installed: %v\n", st.Installed)
			fmt.Printf("Running:   %v\n", st.Running)
			if st.Detail != "" {
				fmt.Printf("Detail:    %s\n", st.Detail)
			}
			return nil
		},
	}
}
