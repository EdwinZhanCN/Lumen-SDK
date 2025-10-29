package cmd

import (
	"fmt"
	"os"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhub/cmd/commands"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	output  string
	host    string
	port    int
)

// Build information (populated by build flags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lumenhub",
	Short: "Lumen Hub CLI - Manage distributed AI services",
	Long: `Lumen Hub CLI is a command-line tool for managing Lumen Hub daemons
and interacting with distributed AI services.

The CLI connects to a running lumenhubd daemon via REST API to:
- Manage and monitor nodes
- Check hub status and health
- Call AI inference services

Usage:
  lumenhub node list              # List discovered nodes
  lumenhub status                  # Show hub status
  lumenhub embed "hello world"     # Text embedding
  lumenhub detect --image img.jpg  # Object detection

	Environment Variables:
  LUMENHUB_HOST    Hub daemon host (default: localhost)
  LUMENHUB_PORT    Hub daemon port (default: 8080)`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildTime),
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(commands.NodeCmd)
	rootCmd.AddCommand(commands.StatusCmd)
	rootCmd.AddCommand(commands.InferCmd)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&output, "output", "table", "output format (table|json|yaml)")
	rootCmd.PersistentFlags().StringVar(&host, "host", "localhost", "lumenhub daemon host")
	rootCmd.PersistentFlags().IntVar(&port, "port", 8080, "lumenhub daemon port")

	// Support environment variables
	if hostEnv := os.Getenv("LUMENHUB_HOST"); hostEnv != "" {
		host = hostEnv
	}
	if portEnv := os.Getenv("LUMENHUB_PORT"); portEnv != "" {
		if p, err := parsePort(portEnv); err == nil {
			port = p
		}
	}
}

// GetServerAddr returns the full server address
func GetServerAddr() string {
	return fmt.Sprintf("%s:%d", host, port)
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	return verbose
}

// GetOutputFormat returns the output format
func GetOutputFormat() string {
	return output
}

// parsePort parses port from string
func parsePort(portStr string) (int, error) {
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 0, err
	}
	return port, nil
}

// GetHost returns the server host
func GetHost() string {
	return host
}

// GetPort returns the server port
func GetPort() int {
	return port
}
