# Lumen Hub CLI

A powerful command-line tool for managing Lumen Hub daemons and interacting with distributed AI services.

## Overview

The `lumenhub` CLI provides a comprehensive interface to:
- Monitor and manage distributed AI nodes
- Execute AI inference across multiple services
- Check system health and performance metrics
- Stream results from distributed AI models

## Installation

```bash
# Build from source
go build -o lumenhub ./cmd/lumenhub

# Or use the pre-built binary
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-$(uname -s)-$(uname -m) -o lumenhub
chmod +x lumenhub
```

## Quick Start

1. **Start the daemon**:
   ```bash
   lumenhubd --daemon --preset basic
   ```

2. **Check system status**:
   ```bash
   lumenhub status
   ```

3. **List available nodes**:
   ```bash
   lumenhub node list
   ```

4. **Run inference**:
   ```bash
   # Text embedding
   lumenhub infer --service embedding --payload-b64 "SGVsbG8gd29ybGQ="

   # Face detection
   lumenhub infer --service face_detection --payload-file image.jpg
   ```

## Commands

### `infer` - AI Inference

Run AI inference requests against available services.

```bash
lumenhub infer --service <service-name> [options]
```

**Required Flags:**
- `--service <name>`: Service name for routing (e.g., `embedding`, `face_detection`, `classification`)

**Optional Flags:**
- `--task <id>`: Task or model identifier
- `--payload-file <path>`: Path to binary payload file (recommended for images/audio)
- `--payload-b64 <string>`: Base64-encoded payload string
- `--metadata <json>`: JSON metadata object (e.g., `'{"threshold":"0.5","max_faces":"10"}'`)
- `--correlation-id <id>`: Correlation ID for request tracing
- `--output <format>`: Output format (`table`|`json`|`yaml`, default: `table`)

**Examples:**

```bash
# Text embedding
echo "Hello world" | base64 | xargs -I {} lumenhub infer --service embedding --payload-b64 {}

# Image classification
lumenhub infer --service classification --payload-file photo.jpg --output json

# Face detection with metadata
lumenhub infer --service face_detection \
  --payload-file selfie.jpg \
  --metadata '{"threshold":"0.8","max_faces":"5"}' \
  --correlation-id "user-photo-analysis-001"

# Streaming inference
lumenhub infer --service embedding_stream --payload-file large_text.txt
```

### `node` - Node Management

Monitor and manage distributed AI nodes.

**Subcommands:**

#### `node list`
List all discovered nodes with their status.

```bash
lumenhub node list [--output table|json|yaml]
```

**Output:**
```
ID                  NAME      ADDRESS              STATUS    LAST SEEN    TASKS
node-abc123         gpu-01    192.168.1.100:5001   🟢 active  2m15s       12
node-def456         cpu-01    192.168.1.101:5001   🟢 active  1m45s       8
```

#### `node info <node-id>`
Show detailed information about a specific node.

```bash
lumenhub node info <node-id> [--output table|json|yaml]
```

**Output includes:**
- Node configuration and capabilities
- Resource usage (CPU, Memory, GPU, Disk)
- Performance statistics
- Available models and services
- Connection status and latency

#### `node status [node-id]`
Show real-time status of nodes.

```bash
# Show all nodes status
lumenhub node status

# Show specific node status
lumenhub node status <node-id>
```

**Features:**
- Real-time resource monitoring with progress bars
- Performance metrics and request statistics
- Color-coded status indicators
- Available services per node

#### `node ping <node-id>`
Test connectivity and latency to a specific node.

```bash
lumenhub node ping <node-id>
```

**Output:**
```
PING node-abc123 (192.168.1.100:5001)
Node status: 🟢 active
Last seen: 1m30s ago
Latency: < 1s (estimated)
```

### `status` - System Status

Show comprehensive system status and health.

```bash
lumenhub status [--nodes] [--metrics] [--health] [--output table|json|yaml]
```

**Flags:**
- `--nodes`: Show node information only
- `--metrics`: Show performance metrics only
- `--health`: Show health check only
- `--output`: Output format (`table`|`json`|`yaml`)

**Output includes:**
- Daemon health status
- Connected nodes summary
- System-wide metrics
- Request statistics

## Available AI Services

The Lumen Hub supports various AI services that you can use with the `infer` command:

### Text Services
- `embedding` - Text vector embedding
- `embedding_stream` - Streaming text embedding for large texts
- `classification` - Text classification
- `classification_stream` - Streaming classification

### Vision Services
- `face_detection` - Face detection in images
- `face_detection_stream` - Streaming face detection
- `face_recognition` - Face recognition
- `face_recognition_stream` - Streaming face recognition

### Generic Usage
```bash
# List available services (you can discover this via node info)
lumenhub node info <node-with-services>

# Use any service
lumenhub infer --service <service-name> --payload-file <data>
```

## Configuration

### Environment Variables

```bash
export LUMENHUB_HOST=localhost    # Daemon host (default: localhost)
export LUMENHUB_PORT=5866         # Daemon port (default: 5866)
```

### Global Flags

All commands support these global flags:

```bash
--host <hostname>        # Daemon host (default: localhost)
--port <port>            # Daemon port (default: 5866)
--output <format>        # Output format (table|json|yaml)
-v, --verbose           # Verbose output
-h, --help              # Show help
--version              # Show version
```

## Output Formats

### Table Format (Default)
Human-readable tables with headers and formatted output:

```bash
lumenhub node list
```

### JSON Format
Machine-readable JSON for scripting:

```bash
lumenhub node list --output json | jq '.nodes[] | select(.status == "active")'
```

### YAML Format
YAML format for configuration files:

```bash
lumenhub node status --output yaml > node_status.yaml
```

## Examples and Use Cases

### 1. Batch Image Processing
```bash
# Process all images in a directory
for img in *.jpg; do
  echo "Processing $img..."
  lumenhub infer --service face_detection --payload-file "$img" --output json
done
```

### 2. Text Analysis Pipeline
```bash
# Embed text chunks for similarity search
chunk1="Hello world, how are you?"
chunk2="Hi there, what's up?"

echo "$chunk1" | base64 | xargs -I {} lumenhub infer --service embedding --payload-b64 {} --correlation-id "chunk-001"
echo "$chunk2" | base64 | xargs -I {} lumenhub infer --service embedding --payload-b64 {} --correlation-id "chunk-002"
```

### 3. System Monitoring
```bash
# Real-time node monitoring
watch -n 5 'lumenhub node status'

# Health check script
#!/bin/bash
health=$(lumenhub status --health --output json | jq -r '.success')
if [ "$health" = "true" ]; then
  echo "✅ System healthy"
else
  echo "❌ System unhealthy"
  exit 1
fi
```

### 4. Load Balancing and Performance
```bash
# Check node performance before making inference
best_node=$(lumenhub node status --output json | jq -r '.nodes | sort_by(.stats.average_latency) | .[0].id')

lumenhub infer --service embedding --payload-file text.txt --metadata '{"preferred_node":"'$best_node'"}'
```

## Troubleshooting

### Common Issues

#### Connection Refused
```bash
Error: failed to get nodes: HTTP request failed: Get "http://localhost:5866/v1/nodes":
dial tcp [::1]:5866: connect: connection refused
```

**Solution:** Ensure the daemon is running:
```bash
lumenhubd --daemon --preset basic
```

#### Node Not Found
```bash
Error: node 'invalid-node' not found
```

**Solution:** Check available nodes:
```bash
lumenhub node list
```

#### Service Not Available
```bash
Error: API error [SERVICE_NOT_FOUND]: Service 'unknown_service' not found
```

**Solution:** Check available services:
```bash
lumenhub node info <node-id>
```

### Debug Mode

Enable verbose output for debugging:

```bash
lumenhub --verbose node status
LUMENHUB_HOST=localhost LUMENHUB_PORT=5866 lumenhub --verbose infer --service embedding --payload-b64 "SGVsbG8="
```

## Performance Tips

1. **Use streaming services** for large payloads (`*_stream` variants)
2. **Check node status** before inference to find optimal nodes
3. **Use correlation IDs** for request tracking in production
4. **Batch requests** when processing multiple items
5. **Monitor resource usage** to prevent node overload

## Integration Examples

### Shell Script Integration
```bash
#!/bin/bash
# health_check.sh - Check if cluster is healthy

NODES=$(lumenhub node list --output json | jq -r '.data.nodes | length')
ACTIVE=$(lumenhub node list --output json | jq -r '.data.nodes[] | select(.status == "active") | length')

echo "Total nodes: $NODES"
echo "Active nodes: $ACTIVE"

if [ "$ACTIVE" -eq 0 ]; then
  echo "❌ No active nodes found"
  exit 1
fi

echo "✅ Cluster is healthy"
```

### Python Integration
```python
import subprocess
import json

def run_inference(service, payload_file, metadata=None):
    cmd = ['lumenhub', 'infer', '--service', service, '--payload-file', payload_file]
    if metadata:
        cmd.extend(['--metadata', json.dumps(metadata)])

    result = subprocess.run(cmd, capture_output=True, text=True)
    return json.loads(result.stdout)

# Usage
result = run_inference('embedding', 'text.txt', {'model': 'multilingual'})
print(result)
```

## Support and Contributing

- **Issues:** Report bugs and request features on [GitHub Issues](https://github.com/edwinzhancn/lumen-sdk/issues)
- **Documentation:** See the [main project README](../../README.md) for architecture details
- **Contributing:** Pull requests are welcome for new features and bug fixes

---

**Note:** This CLI connects to a `lumenhubd` daemon. Make sure the daemon is running before using CLI commands.