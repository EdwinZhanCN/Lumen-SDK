# Installing lumen-hostd

## System Requirements

- **OS**: Linux, macOS, or Windows
- **Network**: port 5866 available (configurable) for the Broker's HTTP/WebSocket API
- **Go 1.25+** only if building from source

## Method 1: Download a release binary (recommended)

### Linux
```bash
# AMD64
curl -fsSL https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumen-hostd-latest-linux-amd64.tar.gz | tar xz
sudo install -m 0755 lumen-hostd /usr/local/bin/

# ARM64
curl -fsSL https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumen-hostd-latest-linux-arm64.tar.gz | tar xz
sudo install -m 0755 lumen-hostd /usr/local/bin/
```

### macOS
```bash
# Apple Silicon
curl -fsSL https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumen-hostd-latest-darwin-arm64.tar.gz | tar xz
xattr -d com.apple.quarantine lumen-hostd 2>/dev/null || true
sudo install -m 0755 lumen-hostd /usr/local/bin/

# Intel
curl -fsSL https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumen-hostd-latest-darwin-amd64.tar.gz | tar xz
xattr -d com.apple.quarantine lumen-hostd 2>/dev/null || true
sudo install -m 0755 lumen-hostd /usr/local/bin/
```

macOS builds are currently unsigned and not notarized. If Gatekeeper blocks the binary after download, clear the quarantine attribute as shown above (and again on the installed copy: `sudo xattr -d com.apple.quarantine /usr/local/bin/lumen-hostd`).

### Windows
```powershell
Invoke-WebRequest -Uri "https://github.com/edwinzhancn/lumen-sdk/releases/latest/download/lumen-hostd-latest-windows-amd64.zip" -OutFile "lumen-hostd.zip"
Expand-Archive -Path "lumen-hostd.zip" -DestinationPath "."
# Move lumen-hostd.exe somewhere on PATH
```

## Method 2: Build from source

```bash
git clone https://github.com/edwinzhancn/lumen-sdk.git
cd Lumen-SDK
make build
sudo make install-local   # installs to /usr/local/bin
```

## Verification

```bash
lumen-hostd version
lumen-hostd --help
```

## Quick start

1. **Install and start as a background service** (recommended for normal use):
   ```bash
   lumen-hostd install
   ```
   This registers a per-user LaunchAgent (macOS), systemd user unit (Linux), or Task Scheduler entry (Windows) that starts `lumen-hostd serve` at login and restarts it on failure, then starts it immediately.

2. **Check status**:
   ```bash
   lumen-hostd status
   ```

3. **Diagnose discovery or reachability issues**:
   ```bash
   lumen-hostd doctor
   ```

4. **Point an application at it** — see [`configuration.md`](configuration.md) for `LUMEN_DISCOVERY_BROKER_URL` and related settings.

Alternatively, run it in the foreground without installing a service — useful in a container or for local development:
```bash
lumen-hostd serve
```

## Uninstalling

```bash
lumen-hostd uninstall          # removes the background service registration
sudo rm /usr/local/bin/lumen-hostd   # if installed via Method 1/2 above
```

## Troubleshooting

### Permission denied
```bash
sudo chmod +x /usr/local/bin/lumen-hostd
```

### Command not found
```bash
# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/bin
```

### Port already in use
```bash
# Find what's using port 5866
lsof -i :5866

# Point lumen-hostd at a different port via config file or env var —
# see configuration.md for server.rest.port / LUMEN_REST_PORT.
```

### Service won't start
```bash
lumen-hostd doctor
```
`doctor` checks service installation state, whether the Broker port is reachable, active network interfaces, discovered node count, and TCP-level reachability to each discovered node. It performs every check locally and does not upload logs or data anywhere.
