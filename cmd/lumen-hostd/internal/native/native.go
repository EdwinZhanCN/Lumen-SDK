// Package native manages OS-native background service registration for
// lumen-hostd: a per-user LaunchAgent on macOS, a Task Scheduler entry on
// Windows, and a systemd user unit on Linux. It replaces the old daemon's
// manual --daemon process detachment and /tmp PID file: the OS service
// manager now owns restart and lifecycle.
package native

// Status reports whether the service is installed and/or running.
type Status struct {
	Installed bool
	Running   bool
	// Detail is a short human-readable explanation, e.g. why Status could
	// not fully determine Running.
	Detail string
}

// Installer registers, unregisters, and controls lumen-hostd as a
// background service. Each platform provides its own implementation behind
// this common interface.
type Installer interface {
	// Install registers lumen-hostd as a background service that starts
	// automatically and restarts on failure. execPath is the absolute path
	// to the lumen-hostd binary; args are passed to it verbatim (normally
	// just "serve").
	Install(execPath string, args []string) error
	// Uninstall removes the service registration. It does not delete the
	// lumen-hostd binary itself.
	Uninstall() error
	// Start starts the installed service.
	Start() error
	// Stop stops the installed service.
	Stop() error
	// Status reports whether the service is installed and running.
	Status() (Status, error)
}

// New returns the Installer for the current platform.
func New() Installer {
	return newPlatformInstaller()
}
