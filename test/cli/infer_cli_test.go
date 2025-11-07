package cli_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhub/cmd/commands"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"
)

func TestInferCLI_BuildsAndSendsRequest(t *testing.T) {
	// Start a test HTTP server to capture the request sent by the CLI.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var got rest.RESTInferRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Basic assertions on received request
		if got.Service != "test_service" {
			t.Fatalf("unexpected service: %s", got.Service)
		}
		if got.Task != "test_task" {
			t.Fatalf("unexpected task: %s", got.Task)
		}
		if got.Metadata == nil || got.Metadata["k"] != "v" {
			t.Fatalf("unexpected metadata: %#v", got.Metadata)
		}
		if string(got.Payload) != "payload-bytes" {
			t.Fatalf("unexpected payload: %s", string(got.Payload))
		}

		// Respond with a simple success envelope
		resp := rest.APIResponse{
			Success:   true,
			Data:      map[string]string{"status": "ok"},
			Timestamp: "",
			RequestID: "req-1",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Prepare a temporary payload file
	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "payload.bin")
	if err := os.WriteFile(payloadPath, []byte("payload-bytes"), 0644); err != nil {
		t.Fatalf("failed to write temp payload file: %v", err)
	}

	// Configure the InferCmd flags to point to the test server
	cmd := commands.InferCmd

	// Ensure host/port flags exist on the command (they may not be global in tests)
	if cmd.Flags().Lookup("host") == nil {
		cmd.Flags().String("host", "", "API host")
	}
	if cmd.Flags().Lookup("port") == nil {
		cmd.Flags().Int("port", 0, "API port")
	}

	// Set required flags and payload
	if err := cmd.Flags().Set("service", "test_service"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}
	if err := cmd.Flags().Set("task", "test_task"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}
	if err := cmd.Flags().Set("payload-file", payloadPath); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}
	if err := cmd.Flags().Set("metadata", `{"k":"v"}`); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}
	// set output to json for deterministic test (though we don't validate output here)
	if err := cmd.Flags().Set("output", "json"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	// Parse test server address into host and port
	hostPort := ts.Listener.Addr().String()
	h, p, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("failed to parse test server addr %q: %v", hostPort, err)
	}

	if err := cmd.Flags().Set("host", h); err != nil {
		t.Fatalf("failed to set host flag: %v", err)
	}
	if err := cmd.Flags().Set("port", p); err != nil {
		// Cobra Int flag accepts string via Set; if it fails, convert to int and set via Format
		if err2 := cmd.Flags().Set("port", p); err2 != nil {
			t.Fatalf("failed to set port flag: %v (also tried fallback)", err)
		}
	}

	// Execute the command's RunE directly (no root command execution).
	if cmd.RunE == nil {
		t.Fatalf("infer command handler not set")
	}
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("infer command failed: %v", err)
	}
}
