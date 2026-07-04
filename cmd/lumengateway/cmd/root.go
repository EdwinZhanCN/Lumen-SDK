package cmd

import (
	"fmt"
	"os"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumengateway/cmd/commands"

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
	Use:   "lumengateway",
	Short: "Lumen Gateway CLI - Manage distributed AI services",
	Long: `Lumen Gateway CLI is a command-line tool for managing Lumen Gateway daemons
and interacting with distributed AI services.

The CLI connects to a running lumengatewayd daemon via REST API to:
- Manage and monitor nodes
- Check hub status and health
- Call AI inference services

Usage:
  lumengateway node list              # List discovered nodes
  lumengateway status                  # Show hub status
  lumengateway infer --service embedding "hello world"           # Text embedding
  lumengateway infer --service face_detection --payload-file img.jpg  # Face detection

	Environment Variables:
  LUMENGATEWAY_HOST    Hub daemon host (default: localhost)
  LUMENGATEWAY_PORT    Hub daemon port (default: 5866)`,
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
	rootCmd.PersistentFlags().StringVar(&host, "host", "localhost", "lumengateway daemon host")
	rootCmd.PersistentFlags().IntVar(&port, "port", 5866, "lumengateway daemon port")

	// Support environment variables
	if hostEnv := os.Getenv("LUMENGATEWAY_HOST"); hostEnv != "" {
		host = hostEnv
	}
	if portEnv := os.Getenv("LUMENGATEWAY_PORT"); portEnv != "" {
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
