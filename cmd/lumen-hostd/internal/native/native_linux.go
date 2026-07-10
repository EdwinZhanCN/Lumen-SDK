//go:build linux

package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemdUnitName = "lumen-hostd.service"

type linuxInstaller struct{}

func newPlatformInstaller() Installer {
	return linuxInstaller{}
}

func (linuxInstaller) Install(execPath string, args []string) error {
	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create systemd user directory: %w", err)
	}

	unit := generateUnit(execPath, args)
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	if out, err := systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w: %s", err, out)
	}
	if out, err := systemctl("enable", "--now", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl enable: %w: %s", err, out)
	}
	return nil
}

func (linuxInstaller) Uninstall() error {
	// Best-effort: disable before removing the unit file. A unit that is
	// already stopped/disabled returns a non-zero exit here, which is fine.
	_, _ = systemctl("disable", "--now", systemdUnitName)

	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove systemd unit: %w", err)
	}

	if out, err := systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w: %s", err, out)
	}
	return nil
}

func (linuxInstaller) Start() error {
	if out, err := systemctl("start", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl start: %w: %s", err, out)
	}
	return nil
}

func (linuxInstaller) Stop() error {
	if out, err := systemctl("stop", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl stop: %w: %s", err, out)
	}
	return nil
}

func (linuxInstaller) Status() (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}

	var st Status
	if _, statErr := os.Stat(path); statErr == nil {
		st.Installed = true
	} else if !os.IsNotExist(statErr) {
		return Status{}, fmt.Errorf("stat systemd unit: %w", statErr)
	}

	if !st.Installed {
		st.Detail = "not installed"
		return st, nil
	}

	out, _ := systemctl("is-active", systemdUnitName)
	st.Running = strings.TrimSpace(out) == "active"
	st.Detail = strings.TrimSpace(out)
	return st, nil
}

func systemctl(args ...string) (string, error) {
	full := append([]string{"--user"}, args...)
	out, err := exec.Command("systemctl", full...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func unitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName), nil
}

// generateUnit is a pure function so its output can be unit tested without
// touching the filesystem or systemd.
func generateUnit(execPath string, args []string) string {
	cmdLine := execPath
	for _, a := range args {
		cmdLine += " " + a
	}

	return `[Unit]
Description=Lumen Host Broker
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=` + cmdLine + `
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`
}
