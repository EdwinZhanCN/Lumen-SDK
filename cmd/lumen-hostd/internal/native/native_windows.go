//go:build windows

package native

import (
	"fmt"
	"os/exec"
	"strings"
)

const taskName = "LumenHostBroker"

type windowsInstaller struct{}

func newPlatformInstaller() Installer {
	return windowsInstaller{}
}

func (windowsInstaller) Install(execPath string, args []string) error {
	tr := generateTaskRunCommand(execPath, args)
	out, err := schtasks("/Create", "/TN", taskName, "/TR", tr, "/SC", "ONLOGON", "/RL", "LIMITED", "/F")
	if err != nil {
		return fmt.Errorf("schtasks /Create: %w: %s", err, out)
	}
	return nil
}

func (windowsInstaller) Uninstall() error {
	out, err := schtasks("/Delete", "/TN", taskName, "/F")
	if err != nil {
		// A task that was never installed returns a non-zero exit; treat
		// that as success rather than an error to keep Uninstall idempotent.
		if strings.Contains(out, "cannot find") || strings.Contains(out, "does not exist") {
			return nil
		}
		return fmt.Errorf("schtasks /Delete: %w: %s", err, out)
	}
	return nil
}

func (windowsInstaller) Start() error {
	out, err := schtasks("/Run", "/TN", taskName)
	if err != nil {
		return fmt.Errorf("schtasks /Run: %w: %s", err, out)
	}
	return nil
}

func (windowsInstaller) Stop() error {
	out, err := schtasks("/End", "/TN", taskName)
	if err != nil {
		return fmt.Errorf("schtasks /End: %w: %s", err, out)
	}
	return nil
}

func (windowsInstaller) Status() (Status, error) {
	out, err := schtasks("/Query", "/TN", taskName, "/FO", "LIST", "/V")
	if err != nil {
		return Status{Detail: "not installed"}, nil
	}

	st := Status{Installed: true}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			st.Running = strings.EqualFold(status, "Running")
			st.Detail = status
			break
		}
	}
	return st, nil
}

func schtasks(args ...string) (string, error) {
	out, err := exec.Command("schtasks", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// generateTaskRunCommand is a pure function so its output can be unit tested
// without touching the filesystem or Task Scheduler. schtasks /TR takes the
// full command line as one string; the executable path is quoted so it
// survives embedded spaces (e.g. "C:\Program Files\Lumen\lumen-hostd.exe").
func generateTaskRunCommand(execPath string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, `"`+execPath+`"`)
	parts = append(parts, args...)
	return strings.Join(parts, " ")
}
