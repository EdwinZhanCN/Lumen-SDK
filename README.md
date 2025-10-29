# Lumen SDK

A distributed AI service platform for managing and coordinating Lumen AI inference across multiple nodes.

## Quick Start


### Package Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/edwinzhancn/lumen-sdk/pkg/client"
    "github.com/edwinzhancn/lumen-sdk/pkg/config"
)

func main() {
    // Create configuration
    cfg := config.DefaultConfig()

    // Create Lumen client
    lumenClient, err := client.NewLumenClient(cfg, nil)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    if err := lumenClient.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer lumenClient.Close()

    // Text embedding inference
    result, err := lumenClient.Embed(ctx, &client.EmbedRequest{
        Text:    "Hello, world!",
        ModelID: "text-embedding-ada-002",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Embedding dimensions: %d\n", len(result.Embedding))
    fmt.Printf("First few values: %v\n", result.Embedding[:5])
}
```

### Server Usage

**Download Release Binaries**
```bash
# Linux AMD64
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-latest-linux-amd64.tar.gz | tar xz
sudo mv lumenhubd lumenhub /usr/local/bin/

# macOS
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-latest-darwin-amd64.tar.gz | tar xz
sudo mv lumenhubd lumenhub /usr/local/bin/
```

**Build from Source**
```bash
git clone https://github.com/edwinzhancn/lumen-sdk.git
cd Lumen-SDK
make build && sudo make install-local
```

### Usage
```bash
# Start daemon
./lumenhubd --daemon --preset basic

# Use CLI
./lumenhub status
./lumenhub node list
./lumenhub infer embed "Hello world"
./lumenhub --version
```

## Architecture

- **lumenhubd**: Background daemon service (REST API, node discovery, load balancing)
- **lumenhub**: CLI client for daemon interaction

## Configuration

**Presets**: `minimal` | `basic` | `lightweight` | `brave`

```bash
./lumenhubd --preset basic     # Personal computer
./lumenhubd --config file.yaml # Custom config
```

## Development

```bash
make build          # Build binaries
make test           # Run tests
make ci             # Full CI pipeline
make release        # Create release
```

## Documentation

- [Installation Guide](docs/installation.md)
- [Configuration](docs/configuration.md)

## License

MIT
