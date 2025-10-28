package commands

import "github.com/spf13/cobra"

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Helper functions to get host and port from command
func getHostFromCmd(cmd *cobra.Command) string {
	if host, _ := cmd.Flags().GetString("host"); host != "" {
		return host
	}
	return "localhost"
}

func getPortFromCmd(cmd *cobra.Command) int {
	if port, _ := cmd.Flags().GetInt("port"); port != 0 {
		return port
	}
	return 8080
}
