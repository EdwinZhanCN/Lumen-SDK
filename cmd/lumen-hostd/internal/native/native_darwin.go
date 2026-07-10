//go:build darwin

package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const launchAgentLabel = "com.edwinzhan.lumen-host"

type darwinInstaller struct{}

func newPlatformInstaller() Installer {
	return darwinInstaller{}
}

func (darwinInstaller) Install(execPath string, args []string) error {
	logDir, err := logDirectory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}

	plist := generatePlist(launchAgentLabel, execPath, args,
		filepath.Join(logDir, "lumen-hostd.log"),
		filepath.Join(logDir, "lumen-hostd.err.log"))
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write LaunchAgent plist: %w", err)
	}

	if out, err := exec.Command("launchctl", "load", "-w", path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinInstaller) Uninstall() error {
	path, err := plistPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		// Best-effort: unload before removing the plist. A LaunchAgent that
		// is already stopped returns a non-zero exit here, which is fine.
		_, _ = exec.Command("launchctl", "unload", "-w", path).CombinedOutput()
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove LaunchAgent plist: %w", err)
	}
	return nil
}

func (darwinInstaller) Start() error {
	if out, err := exec.Command("launchctl", "start", launchAgentLabel).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl start: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinInstaller) Stop() error {
	if out, err := exec.Command("launchctl", "stop", launchAgentLabel).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl stop: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinInstaller) Status() (Status, error) {
	path, err := plistPath()
	if err != nil {
		return Status{}, err
	}

	var st Status
	if _, statErr := os.Stat(path); statErr == nil {
		st.Installed = true
	} else if !os.IsNotExist(statErr) {
		return Status{}, fmt.Errorf("stat LaunchAgent plist: %w", statErr)
	}

	if !st.Installed {
		st.Detail = "not installed"
		return st, nil
	}

	out, err := exec.Command("launchctl", "list", launchAgentLabel).CombinedOutput()
	if err != nil {
		st.Detail = "installed but not loaded"
		return st, nil
	}
	// `launchctl list <label>` prints a plist-like block including "PID" =
	// <n> when running, or "PID" = 0 / no PID key when loaded but not
	// currently running (e.g. between crash and KeepAlive restart).
	st.Running = strings.Contains(string(out), "\"PID\"")
	st.Detail = "loaded"
	return st, nil
}

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func logDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Logs", "Lumen"), nil
}

// generatePlist is a pure function so its output can be unit tested without
// touching the filesystem or launchd.
func generatePlist(label, execPath string, args []string, stdoutPath, stderrPath string) string {
	var argsXML strings.Builder
	argsXML.WriteString("        <string>" + xmlEscape(execPath) + "</string>\n")
	for _, a := range args {
		argsXML.WriteString("        <string>" + xmlEscape(a) + "</string>\n")
	}

	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>` + xmlEscape(label) + `</string>
    <key>ProgramArguments</key>
    <array>
` + argsXML.String() + `    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>` + xmlEscape(stdoutPath) + `</string>
    <key>StandardErrorPath</key>
    <string>` + xmlEscape(stderrPath) + `</string>
</dict>
</plist>
`
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
