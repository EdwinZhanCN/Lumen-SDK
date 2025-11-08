package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhub/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"

	"github.com/spf13/cobra"
)

// NodeCmd represents the node command
var NodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Node management commands",
	Long:  `Manage and interact with discovered nodes in the Lumen network.`,
}

var nodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all discovered nodes",
	Long:  `List all nodes that have been discovered by the Lumen Hub.`,
	RunE:  runNodeList,
}

var nodePingCmd = &cobra.Command{
	Use:   "ping [node-id]",
	Short: "Ping a node to test connectivity",
	Long:  `Send a ping to a specific node to test connectivity and latency.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runNodePing,
}

var nodeInfoCmd = &cobra.Command{
	Use:   "info [node-id]",
	Short: "Show detailed information about a node",
	Long:  `Display detailed information about a specific node, including its capabilities, load, and statistics.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runNodeInfo,
}

var nodeStatusCmd = &cobra.Command{
	Use:   "status [node-id]",
	Short: "Show real-time status of a node",
	Long:  `Display real-time status including connectivity, health, and performance metrics for a specific node.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNodeStatus,
}

func init() {
	NodeCmd.AddCommand(nodeListCmd)
	NodeCmd.AddCommand(nodePingCmd)
	NodeCmd.AddCommand(nodeInfoCmd)
	NodeCmd.AddCommand(nodeStatusCmd)
}

func runNodeList(cmd *cobra.Command, args []string) error {
	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	resp, err := client.GetNodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	outputFormat, _ := cmd.Flags().GetString("output")

	switch outputFormat {
	case "json":
		return outputNodesJSON(resp)
	case "yaml":
		return outputNodesYAML(resp)
	default:
		return outputNodesTable(resp)
	}
}

func runNodePing(cmd *cobra.Command, args []string) error {
	nodeID := args[0]

	// Get node info to check connectivity
	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

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

	// Find the target node
	var targetNode map[string]interface{}
	for _, nodeInterface := range nodesData {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		id, idOk := node["id"].(string)
		name, nameOk := node["name"].(string)
		if (idOk && id == nodeID) || (nameOk && name == nodeID) {
			targetNode = node
			break
		}
	}

	if targetNode == nil {
		return fmt.Errorf("node '%s' not found", nodeID)
	}

	// Display ping information
	address, _ := targetNode["address"].(string)
	status, _ := targetNode["status"].(string)

	fmt.Printf("PING %s (%s)\n", nodeID, address)
	fmt.Printf("Node status: %s\n", status)

	// Check last seen if available
	if lastSeenInterface, ok := targetNode["last_seen"]; ok {
		if lastSeenStr, ok := lastSeenInterface.(string); ok {
			if lastSeen, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
				timeAgo := time.Since(lastSeen).Round(time.Second)
				fmt.Printf("Last seen: %v ago\n", timeAgo)

				if timeAgo < time.Minute {
					fmt.Printf("Latency: < 1s (estimated)\n")
				} else {
					fmt.Printf("Warning: Node hasn't been seen recently\n")
				}
			}
		}
	}

	return nil
}

func runNodeInfo(cmd *cobra.Command, args []string) error {
	nodeID := args[0]

	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

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

	// Find the target node
	var targetNode map[string]interface{}
	for _, nodeInterface := range nodesData {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		id, idOk := node["id"].(string)
		name, nameOk := node["name"].(string)
		if (idOk && id == nodeID) || (nameOk && name == nodeID) {
			targetNode = node
			break
		}
	}

	if targetNode == nil {
		return fmt.Errorf("node '%s' not found", nodeID)
	}

	// Output format
	outputFormat, _ := cmd.Flags().GetString("output")

	switch outputFormat {
	case "json":
		return outputNodeJSON(targetNode)
	case "yaml":
		return outputNodeYAML(targetNode)
	default:
		return outputNodeTableSingle(targetNode)
	}
}

func outputNodesJSON(resp *rest.APIResponse) error {
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}
	if data, err := json.Marshal(dataMap); err == nil {
		fmt.Println(string(data))
	}
	return nil
}

func outputNodesYAML(resp *rest.APIResponse) error {
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}
	nodesData, ok := dataMap["nodes"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	fmt.Printf("nodes: %d\n", len(nodesData))
	return nil
}
func outputNodeJSON(node map[string]interface{}) error {
	if data, err := json.Marshal(node); err == nil {
		fmt.Println(string(data))
	}
	return nil
}

func outputNodeYAML(node map[string]interface{}) error {
	if id, ok := node["id"].(string); ok {
		fmt.Printf("id: %s\n", id)
	}
	if status, ok := node["status"].(string); ok {
		fmt.Printf("status: %s\n", status)
	}
	return nil
}

// outputNodeTable outputs a list of nodes in table format
func outputNodesTable(resp *rest.APIResponse) error {
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	nodesData, ok := dataMap["nodes"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tADDRESS\tSTATUS\tLAST SEEN\tTASKS")

	for _, nodeInterface := range nodesData {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := node["id"].(string)
		name, _ := node["name"].(string)
		address, _ := node["address"].(string)
		status, _ := node["status"].(string)

		lastSeen := "never"
		if lastSeenInterface, ok := node["last_seen"]; ok {
			if lastSeenStr, ok := lastSeenInterface.(string); ok {
				if seen, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
					lastSeen = time.Since(seen).Round(time.Second).String()
				}
			}
		}

		tasks := "0"
		if tasksInterface, ok := node["tasks"]; ok {
			if tasksList, ok := tasksInterface.([]interface{}); ok {
				tasks = fmt.Sprintf("%d", len(tasksList))
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			truncateString(id, 20),
			name,
			address,
			status,
			lastSeen,
			tasks)
	}

	return w.Flush()
}

// outputNodeTableSingle outputs a single node's information in table format
func outputNodeTableSingle(node map[string]interface{}) error {
	fmt.Printf("Node Information:\n")
	fmt.Printf("================\n")

	if id, ok := node["id"].(string); ok {
		fmt.Printf("ID:       %s\n", id)
	}
	if name, ok := node["name"].(string); ok {
		fmt.Printf("Name:     %s\n", name)
	}
	if address, ok := node["address"].(string); ok {
		fmt.Printf("Address:  %s\n", address)
	}
	if status, ok := node["status"].(string); ok {
		fmt.Printf("Status:   %s\n", status)
	}
	if runtime, ok := node["runtime"].(string); ok {
		fmt.Printf("Runtime:  %s\n", runtime)
	}
	if version, ok := node["version"].(string); ok {
		fmt.Printf("Version:  %s\n", version)
	}
	if lastSeenInterface, ok := node["last_seen"]; ok {
		if lastSeenStr, ok := lastSeenInterface.(string); ok {
			if lastSeen, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
				fmt.Printf("Last Seen: %s\n", lastSeen.Format(time.RFC3339))
			}
		}
	}

	// Display load information
	if loadInterface, ok := node["load"]; ok {
		if load, ok := loadInterface.(map[string]interface{}); ok {
			fmt.Printf("\nLoad:\n")
			if cpu, ok := load["cpu"].(float64); ok {
				fmt.Printf("  CPU:    %.1f%%\n", cpu*100)
			}
			if memory, ok := load["memory"].(float64); ok {
				fmt.Printf("  Memory: %.1f%%\n", memory*100)
			}
			if gpu, ok := load["gpu"].(float64); ok {
				fmt.Printf("  GPU:    %.1f%%\n", gpu*100)
			}
			if disk, ok := load["disk"].(float64); ok {
				fmt.Printf("  Disk:   %.1f%%\n", disk*100)
			}
		}
	}

	// Display statistics
	if statsInterface, ok := node["stats"]; ok {
		if stats, ok := statsInterface.(map[string]interface{}); ok {
			fmt.Printf("\nStatistics:\n")
			if totalRequests, ok := stats["total_requests"].(float64); ok {
				fmt.Printf("  Total Requests:      %.0f\n", totalRequests)
			}
			if successfulRequests, ok := stats["successful_requests"].(float64); ok {
				fmt.Printf("  Successful Requests: %.0f\n", successfulRequests)
			}
			if failedRequests, ok := stats["failed_requests"].(float64); ok {
				fmt.Printf("  Failed Requests:     %.0f\n", failedRequests)
			}
			if avgLatency, ok := stats["average_latency"].(float64); ok {
				fmt.Printf("  Average Latency:     %.0fms\n", avgLatency)
			}
			if lastRequestInterface, ok := stats["last_request"]; ok {
				if lastRequestStr, ok := lastRequestInterface.(string); ok {
					fmt.Printf("  Last Request:        %s\n", lastRequestStr)
				}
			}
		}
	}

	// Display tasks
	if tasksInterface, ok := node["tasks"]; ok {
		if tasksList, ok := tasksInterface.([]interface{}); ok && len(tasksList) > 0 {
			fmt.Printf("\nTasks:\n")
			for _, taskInterface := range tasksList {
				if task, ok := taskInterface.(map[string]interface{}); ok {
					if taskName, ok := task["name"].(string); ok {
						fmt.Printf("  - %s\n", taskName)
					}
				}
			}
		}
	}

	// Display models
	if modelsInterface, ok := node["models"]; ok {
		if modelsList, ok := modelsInterface.([]interface{}); ok && len(modelsList) > 0 {
			fmt.Printf("\nModels:\n")
			for _, modelInterface := range modelsList {
				if model, ok := modelInterface.(map[string]interface{}); ok {
					if modelName, ok := model["name"].(string); ok {
						if runtime, ok := model["runtime"].(string); ok {
							fmt.Printf("  - %s (%s)\n", modelName, runtime)
						} else {
							fmt.Printf("  - %s\n", modelName)
						}
					}
				}
			}
		}
	}

	return nil
}
func runNodeStatus(cmd *cobra.Command, args []string) error {
	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	// Get health check first
	healthResp, err := client.GetHealth()
	if err != nil {
		return fmt.Errorf("failed to get daemon health: %w", err)
	}

	fmt.Printf("Lumen Hub Status: %s\n", formatHealthStatus(healthResp))

	// Get node information
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

	// If specific node requested, show only that node
	if len(args) == 1 {
		nodeID := args[0]
		for _, nodeInterface := range nodesData {
			node, ok := nodeInterface.(map[string]interface{})
			if !ok {
				continue
			}

			id, idOk := node["id"].(string)
			name, nameOk := node["name"].(string)
			if (idOk && id == nodeID) || (nameOk && name == nodeID) {
				return outputNodeStatus(node)
			}
		}
		return fmt.Errorf("node '%s' not found", nodeID)
	}

	// Show status for all nodes
	if len(nodesData) == 0 {
		fmt.Printf("\nNo nodes connected\n")
		return nil
	}

	fmt.Printf("\nConnected Nodes: %d\n\n", len(nodesData))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tSTATUS\tCPU\tMEMORY\tGPU\tDISK\tREQUESTS\tLATENCY")

	for _, nodeInterface := range nodesData {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := node["id"].(string)
		name, _ := node["name"].(string)
		status, _ := node["status"].(string)

		nodeName := name
		if nodeName == "" {
			nodeName = truncateString(id, 12)
		}

		// Extract load information
		var cpu, memory, gpu, disk string
		if loadInterface, ok := node["load"]; ok {
			if load, ok := loadInterface.(map[string]interface{}); ok {
				if cpuVal, ok := load["cpu"].(float64); ok {
					cpu = fmt.Sprintf("%.0f%%", cpuVal*100)
				}
				if memVal, ok := load["memory"].(float64); ok {
					memory = fmt.Sprintf("%.0f%%", memVal*100)
				}
				if gpuVal, ok := load["gpu"].(float64); ok {
					gpu = fmt.Sprintf("%.0f%%", gpuVal*100)
				}
				if diskVal, ok := load["disk"].(float64); ok {
					disk = fmt.Sprintf("%.0f%%", diskVal*100)
				}
			}
		}

		if cpu == "" {
			cpu = "N/A"
		}
		if memory == "" {
			memory = "N/A"
		}
		if gpu == "" {
			gpu = "N/A"
		}
		if disk == "" {
			disk = "N/A"
		}

		// Extract statistics
		var requests, latency string
		if statsInterface, ok := node["stats"]; ok {
			if stats, ok := statsInterface.(map[string]interface{}); ok {
				if totalReqs, ok := stats["total_requests"].(float64); ok {
					requests = fmt.Sprintf("%.0f", totalReqs)
				}
				if avgLat, ok := stats["average_latency"].(float64); ok {
					latency = fmt.Sprintf("%.0fms", avgLat)
				}
			}
		}

		if requests == "" {
			requests = "0"
		}
		if latency == "" {
			latency = "N/A"
		}

		// Status formatting with colors
		statusIcon := getStatusIcon(status)

		fmt.Fprintf(w, "%s\t%s%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			nodeName, statusIcon, status, cpu, memory, gpu, disk, requests, latency)
	}

	return w.Flush()
}

func formatHealthStatus(resp *rest.APIResponse) string {
	if resp.Success {
		return "✅ Healthy"
	} else {
		return "❌ Unhealthy"
	}
}

func getStatusIcon(status string) string {
	switch status {
	case "active", "ready", "connected":
		return "🟢 "
	case "connecting", "pending":
		return "🟡 "
	case "error", "failed", "disconnected":
		return "🔴 "
	default:
		return "⚪ "
	}
}

func outputNodeStatus(node map[string]interface{}) error {
	name, _ := node["name"].(string)
	id, _ := node["id"].(string)
	status, _ := node["status"].(string)
	address, _ := node["address"].(string)

	nodeName := name
	if nodeName == "" {
		nodeName = id
	}

	fmt.Printf("Node Status: %s (%s)\n", nodeName, getStatusIcon(status))
	fmt.Printf("ID:         %s\n", id)
	fmt.Printf("Address:    %s\n", address)
	fmt.Printf("Status:     %s %s\n", getStatusIcon(status), status)

	// Last seen
	if lastSeenInterface, ok := node["last_seen"]; ok {
		if lastSeenStr, ok := lastSeenInterface.(string); ok {
			if lastSeen, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
				timeAgo := time.Since(lastSeen).Round(time.Second)
				fmt.Printf("Last Seen:  %v ago\n", timeAgo)
			}
		}
	}

	// Resource usage
	if loadInterface, ok := node["load"]; ok {
		if load, ok := loadInterface.(map[string]interface{}); ok {
			fmt.Printf("\nResource Usage:\n")
			if cpu, ok := load["cpu"].(float64); ok {
				fmt.Printf("  CPU:    %s%s\n", getProgressBar(cpu*100), fmt.Sprintf("%.0f%%", cpu*100))
			}
			if memory, ok := load["memory"].(float64); ok {
				fmt.Printf("  Memory: %s%s\n", getProgressBar(memory*100), fmt.Sprintf("%.0f%%", memory*100))
			}
			if gpu, ok := load["gpu"].(float64); ok {
				fmt.Printf("  GPU:    %s%s\n", getProgressBar(gpu*100), fmt.Sprintf("%.0f%%", gpu*100))
			}
			if disk, ok := load["disk"].(float64); ok {
				fmt.Printf("  Disk:   %s%s\n", getProgressBar(disk*100), fmt.Sprintf("%.0f%%", disk*100))
			}
		}
	}

	// Performance metrics
	if statsInterface, ok := node["stats"]; ok {
		if stats, ok := statsInterface.(map[string]interface{}); ok {
			fmt.Printf("\nPerformance Metrics:\n")
			var totalRequests float64
			if totalReqs, ok := stats["total_requests"].(float64); ok {
				totalRequests = totalReqs
				fmt.Printf("  Total Requests:      %.0f\n", totalRequests)
			}
			if successfulRequests, ok := stats["successful_requests"].(float64); ok {
				successRate := float64(0)
				if totalRequests > 0 {
					successRate = (successfulRequests / totalRequests) * 100
				}
				fmt.Printf("  Successful Requests: %.0f (%.1f%%)\n", successfulRequests, successRate)
			}
			if failedRequests, ok := stats["failed_requests"].(float64); ok {
				fmt.Printf("  Failed Requests:     %.0f\n", failedRequests)
			}
			if avgLatency, ok := stats["average_latency"].(float64); ok {
				fmt.Printf("  Average Latency:     %.0fms\n", avgLatency)
			}
			if lastRequestInterface, ok := stats["last_request"]; ok {
				if lastRequestStr, ok := lastRequestInterface.(string); ok {
					fmt.Printf("  Last Request:        %s\n", lastRequestStr)
				}
			}
		}
	}

	// Available services/models
	if modelsInterface, ok := node["models"]; ok {
		if modelsList, ok := modelsInterface.([]interface{}); ok && len(modelsList) > 0 {
			fmt.Printf("\nAvailable Services: %d\n", len(modelsList))
			for _, modelInterface := range modelsList {
				if model, ok := modelInterface.(map[string]interface{}); ok {
					if modelName, ok := model["name"].(string); ok {
						runtime, _ := model["runtime"].(string)
						if runtime != "" {
							fmt.Printf("  • %s (%s)\n", modelName, runtime)
						} else {
							fmt.Printf("  • %s\n", modelName)
						}
					}
				}
			}
		}
	}

	return nil
}

func getProgressBar(percentage float64) string {
	const barWidth = 20
	filled := int(percentage / 100 * barWidth)
	if filled > barWidth {
		filled = barWidth
	}

	bar := "["
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "] "

	// Color coding based on percentage
	if percentage >= 80 {
		bar = "\033[31m" + bar + "\033[0m" // Red for high usage
	} else if percentage >= 60 {
		bar = "\033[33m" + bar + "\033[0m" // Yellow for medium usage
	} else {
		bar = "\033[32m" + bar + "\033[0m" // Green for low usage
	}

	return bar
}
