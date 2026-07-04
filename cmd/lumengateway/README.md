# Lumen Gateway CLI

A powerful command-line tool for managing Lumen Gateway daemons and interacting with distributed AI services.

## Overview

The `lumengateway` CLI provides a comprehensive interface to:
- Monitor and manage distributed AI nodes
- Execute AI inference across multiple services
- Check system health and performance metrics
- Stream results from distributed AI models

## Installation

```bash
# Build from source
go build -o lumengateway ./cmd/lumengateway

# Or use the pre-built binary
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumengateway-$(uname -s)-$(uname -m) -o lumengateway
chmod +x lumengateway
```

## Quick Start

1. **Start the daemon**:
   ```bash
   lumengatewayd --daemon --preset basic
   ```

2. **Check system status**:
   ```bash
   lumengateway status
   ```

3. **List available nodes**:
   ```bash
   lumengateway node list
   ```

4. **Run inference**:
   ```bash
   # Text embedding
   lumengateway infer --service clip --task semantic_text_embed --payload-mime text/plain --payload-file text.txt

   # Face recognition
   lumengateway infer --service insightface --task face_recognition --payload-mime image/jpeg --payload-file image.jpg
   ```

## Commands

### `infer` - AI Inference

Run AI inference requests against available services.

```bash
lumengateway infer --service <service-name> [options]
```

**Required Flags:**
- `--service <name>`: Service name written to `meta.service` (e.g., `clip`, `siglip`, `ppocr`, `insightface`)

**Optional Flags:**
- `--task <id>`: Lumen task name, e.g. `semantic_text_embed`, `semantic_image_embed`, `bioclip_classify`, `ocr`, `face_recognition`
- `--payload-mime <mime>`: Payload MIME type
- `--payload-file <path>`: Path to binary payload file (recommended for images/audio)
- `--payload-b64 <string>`: Base64-encoded payload string
- `--meta <json>`: JSON meta object (e.g., `'{"top_k":"10"}'`)
- `--correlation-id <id>`: Correlation ID for request tracing
- `--output <format>`: Output format (`table`|`json`|`yaml`, default: `table`)

**Examples:**

```bash
# Text embedding
printf "Hello world" > text.txt
lumengateway infer --service clip --task semantic_text_embed --payload-mime text/plain --payload-file text.txt

# BioCLIP classification
lumengateway infer --service clip --task bioclip_classify --payload-mime image/jpeg --payload-file photo.jpg --meta '{"top_k":"10"}' --output json

# Face recognition
lumengateway infer --service insightface --task face_recognition --payload-mime image/jpeg \
  --payload-file selfie.jpg \
  --correlation-id "user-photo-analysis-001"
```

### `node` - Node Management

Monitor and manage distributed AI nodes.

**Subcommands:**

#### `node list`
List all discovered nodes with their status.

```bash
lumengateway node list [--output table|json|yaml]
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
lumengateway node info <node-id> [--output table|json|yaml]
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
lumengateway node status

# Show specific node status
lumengateway node status <node-id>
```

**Features:**
- Real-time resource monitoring with progress bars
- Performance metrics and request statistics
- Color-coded status indicators
- Available services per node

#### `node ping <node-id>`
Test connectivity and latency to a specific node.

```bash
lumengateway node ping <node-id>
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
lumengateway status [--nodes] [--metrics] [--health] [--output table|json|yaml]
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

The Lumen Gateway supports various AI services that you can use with the `infer` command:

### Text Services
- `semantic_text_embed` via `clip` or `siglip`

### Vision Services
- `semantic_image_embed` via `clip` or `siglip`
- `bioclip_classify` via `clip`
- `ocr` via `ppocr`
- `face_recognition` via `insightface`

### Generic Usage
```bash
# List available services (you can discover this via node info)
lumengateway node info <node-with-services>

# Use any service
lumengateway infer --service <service-name> --payload-file <data>
```

## Configuration

### Environment Variables

```bash
export LUMENGATEWAY_HOST=localhost    # Daemon host (default: localhost)
export LUMENGATEWAY_PORT=5866         # Daemon port (default: 5866)
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
lumengateway node list
```

### JSON Format
Machine-readable JSON for scripting:

```bash
lumengateway node list --output json | jq '.nodes[] | select(.status == "active")'
```

### YAML Format
YAML format for configuration files:

```bash
lumengateway node status --output yaml > node_status.yaml
```

## Examples and Use Cases

### 1. Batch Image Processing
```bash
# Process all images in a directory
for img in *.jpg; do
  echo "Processing $img..."
  lumengateway infer --service insightface --task face_recognition --payload-mime image/jpeg --payload-file "$img" --output json
done
```

### 2. Text Analysis Pipeline
```bash
# Embed text chunks for similarity search
chunk1="Hello world, how are you?"
chunk2="Hi there, what's up?"

echo "$chunk1" | base64 | xargs -I {} lumengateway infer --service embedding --payload-b64 {} --correlation-id "chunk-001"
echo "$chunk2" | base64 | xargs -I {} lumengateway infer --service embedding --payload-b64 {} --correlation-id "chunk-002"
```

### 3. System Monitoring
```bash
# Real-time node monitoring
watch -n 5 'lumengateway node status'

# Health check script
#!/bin/bash
health=$(lumengateway status --health --output json | jq -r '.success')
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
best_node=$(lumengateway node status --output json | jq -r '.nodes | sort_by(.stats.average_latency) | .[0].id')

lumengateway infer --service clip --task semantic_text_embed --payload-mime text/plain --payload-file text.txt --meta '{"preferred_node":"'$best_node'"}'
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
lumengatewayd --daemon --preset basic
```

#### Node Not Found
```bash
Error: node 'invalid-node' not found
```

**Solution:** Check available nodes:
```bash
lumengateway node list
```

#### Service Not Available
```bash
Error: API error [SERVICE_NOT_FOUND]: Service 'unknown_service' not found
```

**Solution:** Check available services:
```bash
lumengateway node info <node-id>
```

### Debug Mode

Enable verbose output for debugging:

```bash
lumengateway --verbose node status
LUMENGATEWAY_HOST=localhost LUMENGATEWAY_PORT=5866 lumengateway --verbose infer --service embedding --payload-b64 "SGVsbG8="
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

NODES=$(lumengateway node list --output json | jq -r '.data.nodes | length')
ACTIVE=$(lumengateway node list --output json | jq -r '.data.nodes[] | select(.status == "active") | length')

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
    cmd = ['lumengateway', 'infer', '--service', service, '--payload-file', payload_file]
    if metadata:
        cmd.extend(['--meta', json.dumps(metadata)])

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

**Note:** This CLI connects to a `lumengatewayd` daemon. Make sure the daemon is running before using CLI commands.
