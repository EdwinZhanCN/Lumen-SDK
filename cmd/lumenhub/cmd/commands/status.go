package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhub/internal"

	"github.com/spf13/cobra"
)

// StatusCmd represents the status command
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show hub and node status",
	Long:  `Display the current status of the Lumen Hub and all discovered nodes.`,
	RunE:  runStatus,
}

func init() {
	// Add command-level flags
	StatusCmd.Flags().Bool("nodes", true, "show node information")
	StatusCmd.Flags().Bool("metrics", true, "show metrics information")
	StatusCmd.Flags().Bool("health", true, "show health information")
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Get flags
	showNodes, _ := cmd.Flags().GetBool("nodes")
	showMetrics, _ := cmd.Flags().GetBool("metrics")
	showHealth, _ := cmd.Flags().GetBool("health")

	// Create API client
	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	// Get output format
	outputFormat, _ := cmd.Flags().GetString("output")

	// Display hub status
	fmt.Printf("Lumen Hub Status\n")
	fmt.Printf("================\n")
	fmt.Printf("Server: %s\n", getHostFromCmd(cmd)+":"+fmt.Sprintf("%d", getPortFromCmd(cmd)))
	fmt.Printf("Version: 1.0.0\n")
	fmt.Printf("Status: Connected\n")

	if showHealth {
		fmt.Printf("\nHealth:\n")
		if err := displayHealth(client, outputFormat); err != nil {
			fmt.Printf("  Error: %v\n", err)
		}
	}

	if showMetrics {
		fmt.Printf("\nMetrics:\n")
		if err := displayMetrics(client, outputFormat); err != nil {
			fmt.Printf("  Error: %v\n", err)
		}
	}

	if showNodes {
		fmt.Printf("\nNodes:\n")
		if err := displayNodeSummary(client, outputFormat); err != nil {
			fmt.Printf("  Error: %v\n", err)
		}
	}

	return nil
}

func displayHealth(client *internal.APIClient, outputFormat string) error {
	resp, err := client.GetHealth()
	if err != nil {
		return fmt.Errorf("failed to get health: %w", err)
	}

	switch outputFormat {
	case "json":
		if data, err := json.Marshal(resp.Data); err == nil {
			fmt.Printf("  %s\n", string(data))
		}
		return nil
	case "yaml":
		fmt.Printf("  status: healthy\n")
		fmt.Printf("  timestamp: %s\n", resp.Timestamp)
		return nil
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Status:\t%s\n", "healthy")
		fmt.Fprintf(w, "  Timestamp:\t%s\n", resp.Timestamp)
		if reqID := resp.RequestID; reqID != "" {
			fmt.Fprintf(w, "  Request ID:\t%s\n", reqID)
		}
		return w.Flush()
	}
}

func displayMetrics(client *internal.APIClient, outputFormat string) error {
	resp, err := client.GetMetrics()
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}

	switch outputFormat {
	case "json":
		if data, err := json.Marshal(resp.Data); err == nil {
			fmt.Printf("  %s\n", string(data))
		}
		return nil
	case "yaml":
		fmt.Printf("  metrics:\n")
		fmt.Printf("    available: true\n")
		return nil
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Available:\ttrue\n")
		fmt.Fprintf(w, "  Timestamp:\t%s\n", resp.Timestamp)
		if reqID := resp.RequestID; reqID != "" {
			fmt.Fprintf(w, "  Request ID:\t%s\n", reqID)
		}
		return w.Flush()
	}
}

func displayNodeSummary(client *internal.APIClient, outputFormat string) error {
	resp, err := client.GetNodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	nodesData, ok := dataMap["nodes"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	if !ok {
		return fmt.Errorf("invalid response format")
	}

	if len(nodesData) == 0 {
		fmt.Printf("  No nodes discovered\n")
		return nil
	}

	switch outputFormat {
	case "json":
		if data, err := json.Marshal(map[string]interface{}{
			"total": len(nodesData),
			"nodes": nodesData,
		}); err == nil {
			fmt.Printf("  %s\n", string(data))
		}
		return nil
	case "yaml":
		fmt.Printf("  total: %d\n", len(nodesData))
		return nil
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  ID\tSTATUS\tLAST SEEN\tTASKS")

		for _, nodeInterface := range nodesData {
			node, ok := nodeInterface.(map[string]interface{})
			if !ok {
				continue
			}

			id, _ := node["id"].(string)
			status, _ := node["status"].(string)
			taskCount := 0

			// Count tasks
			if tasksInterface, ok := node["tasks"]; ok {
				if tasksList, ok := tasksInterface.([]interface{}); ok {
					taskCount = len(tasksList)
				}
			}

			lastSeen := "never"
			if lastSeenInterface, ok := node["last_seen"]; ok {
				if lastSeenStr, ok := lastSeenInterface.(string); ok {
					if seen, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
						lastSeen = time.Since(seen).Round(time.Second).String()
					}
				}
			}

			fmt.Fprintf(w, "  %s\t%s\t%s\t%d\n",
				truncateString(id, 20),
				status,
				lastSeen,
				taskCount)
		}

		return w.Flush()
	}
}
