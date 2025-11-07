# Installation Guide

## System Requirements

- **Go**: 1.21 or later
- **OS**: Linux, macOS, or Windows
- **Memory**: Minimum 512MB RAM
- **Disk**: 100MB free space
- **Network**: Port 8080 available for REST API

## Installation Methods

### Method 1: Download Release Binaries (Recommended)

#### Linux
```bash
# AMD64
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-latest-linux-amd64.tar.gz | tar xz
sudo mv lumenhubd lumenhub /usr/local/bin/

# ARM64
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-latest-linux-arm64.tar.gz | tar xz
sudo mv lumenhubd lumenhub /usr/local/bin/
```

#### macOS
```bash
# Intel (AMD64)
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-latest-darwin-amd64.tar.gz | tar xz
sudo mv lumenhubd lumenhub /usr/local/bin/

# Apple Silicon (ARM64)
curl -L https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-latest-darwin-arm64.tar.gz | tar xz
sudo mv lumenhubd lumenhub /usr/local/bin/
```

#### Windows
```powershell
# Download and extract
Invoke-WebRequest -Uri "https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumenhub-latest-windows-amd64.zip" -OutFile "lumenhub.zip"
Expand-Archive -Path "lumenhub.zip" -DestinationPath "."
# Move to PATH or add to PATH
```

### Method 2: Build from Source

#### Prerequisites
```bash
# Install Go (if not already installed)
# macOS
brew install go

# Linux (Ubuntu/Debian)
sudo apt update && sudo apt install golang-go

# Linux (CentOS/RHEL)
sudo yum install golang
```

#### Build Instructions
```bash
# Clone repository
git clone https://github.com/edwinzhancn/lumen-sdk.git
cd Lumen-SDK

# Build binaries
make build

# Install to system PATH
sudo make install-local
```

## Verification

After installation, verify the binaries:

```bash
# Check versions
lumenhub --version
lumenhubd --version

# Test help commands
lumenhub --help
lumenhubd --help
```

## Quick Start

1. **Start the daemon**:
   ```bash
   lumenhubd --daemon --preset basic
   ```

2. **Verify installation**:
   ```bash
   lumenhub status
   ```

3. **Test functionality**:
   ```bash
   lumenhub node list
   lumenhub infer --service embedding --payload-b64 "SGVsbG8sIHdvcmxkIQ=="
   ```

## Troubleshooting

### Common Issues

#### Permission Denied
```bash
# Fix permissions
sudo chmod +x /usr/local/bin/lumenhub*
```

#### Command Not Found
```bash
# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/bin

# Or create symlinks
sudo ln -s /usr/local/bin/lumenhub /usr/local/bin/lumenhubd
```

#### Port Already in Use
```bash
# Check what's using port 8080
lsof -i :8080

# Use different port
lumenhubd --preset basic
# Then update CLI
lumenhub --port 8081 status
```
