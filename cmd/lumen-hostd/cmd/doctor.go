package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/internal"
	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/internal/native"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"

	"github.com/spf13/cobra"
)

// NewDoctorCommand runs local diagnostics against the Host Broker: service
// installation state, whether the Broker port is reachable, discovered
// network interfaces, discovered node count and endpoint reachability, and
// a Docker host-name hint. It does not upload logs or data anywhere — every
// check is local-only.
//
// This rollout skips token authentication (see plan §12), so there is no
// "auth token readable" check here even though the plan's original doctor
// spec includes one.
func NewDoctorCommand() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the local Host Broker installation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(configFile)
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "Path to configuration file to check against")
	return cmd
}

type doctorResult struct {
	name   string
	pass   bool
	detail string
}

func runDoctor(configFile string) error {
	cfg, err := internal.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	var results []doctorResult
	results = append(results, checkServiceInstalled())
	results = append(results, checkNetworkInterfaces())

	addr := fmt.Sprintf("%s:%d", loopbackHost(cfg.Server.REST.Host), cfg.Server.REST.Port)
	healthResult, reachable := checkBrokerPort(addr)
	results = append(results, healthResult)

	if reachable {
		results = append(results, checkDiscoveredNodes(addr)...)
	}

	results = append(results, dockerHostGuidance())

	printDoctorResults(results)
	return nil
}

func checkServiceInstalled() doctorResult {
	st, err := native.New().Status()
	if err != nil {
		return doctorResult{name: "service installation", pass: false, detail: err.Error()}
	}
	if !st.Installed {
		return doctorResult{name: "service installation", pass: false, detail: "not installed (run 'lumen-hostd install')"}
	}
	if !st.Running {
		return doctorResult{name: "service installation", pass: false, detail: "installed but not running: " + st.Detail}
	}
	return doctorResult{name: "service installation", pass: true, detail: "installed and running"}
}

func checkNetworkInterfaces() doctorResult {
	ifaces, err := net.Interfaces()
	if err != nil {
		return doctorResult{name: "network interfaces", pass: false, detail: err.Error()}
	}
	var up []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		up = append(up, iface.Name)
	}
	if len(up) == 0 {
		return doctorResult{name: "network interfaces", pass: false, detail: "no active non-loopback interfaces found"}
	}
	return doctorResult{name: "network interfaces", pass: true, detail: fmt.Sprintf("%d active: %v", len(up), up)}
}

func checkBrokerPort(addr string) (doctorResult, bool) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/v1/health", addr))
	if err != nil {
		return doctorResult{name: "broker port", pass: false, detail: fmt.Sprintf("%s unreachable: %v", addr, err)}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return doctorResult{name: "broker port", pass: false, detail: fmt.Sprintf("%s returned HTTP %d", addr, resp.StatusCode)}, false
	}
	return doctorResult{name: "broker port", pass: true, detail: addr + " reachable"}, true
}

func checkDiscoveredNodes(addr string) []doctorResult {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/v1/nodes", addr))
	if err != nil {
		return []doctorResult{{name: "discovered nodes", pass: false, detail: err.Error()}}
	}
	defer resp.Body.Close()

	var body struct {
		Nodes []*discovery.NodeInfo `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return []doctorResult{{name: "discovered nodes", pass: false, detail: "could not parse /v1/nodes response: " + err.Error()}}
	}

	results := []doctorResult{
		{name: "discovered nodes", pass: len(body.Nodes) > 0, detail: fmt.Sprintf("%d node(s)", len(body.Nodes))},
	}
	for _, n := range body.Nodes {
		results = append(results, checkNodeEndpoint(n))
	}
	return results
}

// checkNodeEndpoint is a TCP-level reachability check, not a full gRPC
// handshake: it catches the most common real-world failure (advertised
// address unreachable from this host, e.g. wrong subnet or VPN
// interference — see plan §19.1) without pulling in gRPC-specific health
// check machinery for a diagnostic command.
func checkNodeEndpoint(n *discovery.NodeInfo) doctorResult {
	name := fmt.Sprintf("node %s TCP reachability", n.ID)
	if n.Address == "" {
		return doctorResult{name: name, pass: false, detail: "no advertised address"}
	}
	conn, err := net.DialTimeout("tcp", n.Address, 2*time.Second)
	if err != nil {
		return doctorResult{name: name, pass: false, detail: fmt.Sprintf("%s: %v", n.Address, err)}
	}
	conn.Close()
	return doctorResult{name: name, pass: true, detail: n.Address + " reachable"}
}

func dockerHostGuidance() doctorResult {
	return doctorResult{
		name: "Docker host guidance",
		pass: true,
		detail: "containers on Docker Desktop (macOS/Windows) should reach this Broker " +
			"via host.docker.internal, with \"extra_hosts: [host.docker.internal:host-gateway]\" " +
			"set on Linux",
	}
}

// loopbackHost substitutes a connectable address for a bind-only host like
// "0.0.0.0" or an empty string, so doctor's own local check can dial it.
func loopbackHost(host string) string {
	if host == "" || host == "0.0.0.0" {
		return "127.0.0.1"
	}
	return host
}

func printDoctorResults(results []doctorResult) {
	for _, r := range results {
		mark := "FAIL"
		if r.pass {
			mark = "PASS"
		}
		fmt.Printf("[%s] %-28s %s\n", mark, r.name, r.detail)
	}
}
