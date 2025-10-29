package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhubd/internal"
	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhubd/service"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"

	"go.uber.org/zap"
)

var (
	configFile = flag.String("config", "", "Path to configuration file")
	preset     = flag.String("preset", "", "Use preset configuration (minimal|basic|lightweight|brave)")
	daemon     = flag.Bool("daemon", false, "Run as daemon process")
	version    = flag.Bool("version", false, "Show version information")
)

// Build information (populated by build flags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	flag.Parse()

	// Show version information
	if *version {
		fmt.Printf("Lumen Hub Daemon %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Built: %s\n", BuildTime)
		fmt.Printf("Go: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	// Handle daemon mode
	if *daemon {
		if err := daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to daemonize: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Load configuration
	cfg, err := internal.LoadConfig(*configFile, *preset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	logger, err := createLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Create and start service
	hubdService, err := service.NewHubdService(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create hubd service", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := hubdService.Start(ctx); err != nil {
		logger.Fatal("Failed to start hubd service", zap.Error(err))
	}

	logger.Info("Lumen Hub daemon started successfully",
		zap.String("preset", *preset),
		zap.String("config", *configFile),
		zap.String("version", "1.0.0"))

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Stop service gracefully
	if err := hubdService.Stop(); err != nil {
		logger.Error("Error stopping service", zap.Error(err))
	}

	logger.Info("Lumen Hub daemon stopped gracefully")
}

// daemonize forks the process to run as a daemon
func daemonize() error {
	// Check if we're already the child process
	if os.Getenv("LUMENHUBD_DAEMONIZED") == "1" {
		// We're the child, continue with normal execution
		os.Unsetenv("LUMENHUBD_DAEMONIZED")
		return runDaemon()
	}

	// Platform-specific daemonization
	if runtime.GOOS == "windows" {
		return daemonizeWindows()
	}
	return daemonizeUnix()
}

// daemonizeUnix handles Unix-like systems (Linux, macOS)
func daemonizeUnix() error {
	// Fork the process
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "LUMENHUBD_DAEMONIZED=1")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Parent process exits
	fmt.Printf("Lumen Hub daemon started with PID %d\n", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

// daemonizeWindows handles Windows systems
func daemonizeWindows() error {
	// On Windows, we use a simpler approach since traditional Unix daemonization
	// doesn't apply. We detach from the console and run in background.

	// Create the daemon process
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "LUMENHUBD_DAEMONIZED=1")

	// Redirect output to null
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process on Windows: %w", err)
	}

	// Parent process exits
	fmt.Printf("Lumen Hub daemon started with PID %d\n", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

// runDaemon runs the actual daemon service
func runDaemon() error {
	// Load configuration
	cfg, err := internal.LoadConfig(*configFile, *preset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		return err
	}

	// Create logger
	logger, err := createLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return err
	}
	defer logger.Sync()

	// Create and start service
	hubdService, err := service.NewHubdService(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create hubd service", zap.Error(err))
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := hubdService.Start(ctx); err != nil {
		logger.Fatal("Failed to start hubd service", zap.Error(err))
		return err
	}

	logger.Info("Lumen Hub daemon started successfully",
		zap.String("preset", *preset),
		zap.String("config", *configFile),
		zap.String("version", "1.0.0"),
		zap.Int("pid", os.Getpid()),
		zap.String("os", runtime.GOOS))

	// Create PID file
	if err := createPIDFile(); err != nil {
		logger.Warn("Failed to create PID file", zap.Error(err))
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Remove PID file
	if err := removePIDFile(); err != nil {
		logger.Warn("Failed to remove PID file", zap.Error(err))
	}

	// Stop service gracefully
	if err := hubdService.Stop(); err != nil {
		logger.Error("Error stopping service", zap.Error(err))
		return err
	}

	logger.Info("Lumen Hub daemon stopped gracefully")
	return nil
}

// createPIDFile creates a PID file for the daemon
func createPIDFile() error {
	pidFile := "/tmp/lumenhubd.pid"

	// Write PID to file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// removePIDFile removes the PID file
func removePIDFile() error {
	pidFile := "/tmp/lumenhubd.pid"

	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	return nil
}

// createLogger creates a logger instance based on the configuration
func createLogger(cfg config.LoggingConfig) (*zap.Logger, error) {
	var zapConfig zap.Config

	switch cfg.Format {
	case "json":
		zapConfig = zap.NewProductionConfig()
	case "text":
		zapConfig = zap.NewDevelopmentConfig()
	default:
		return nil, fmt.Errorf("unsupported log format: %s", cfg.Format)
	}

	// Set log level
	var zapLevel zap.AtomicLevel
	switch cfg.Level {
	case "debug":
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "fatal":
		zapLevel = zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		return nil, fmt.Errorf("unsupported log level: %s", cfg.Level)
	}

	zapConfig.Level = zapLevel

	// Set output path
	switch cfg.Output {
	case "stdout":
		zapConfig.OutputPaths = []string{"stdout"}
	case "stderr":
		zapConfig.OutputPaths = []string{"stderr"}
	default:
		// Assume it's a file path
		zapConfig.OutputPaths = []string{cfg.Output}
	}

	return zapConfig.Build()
}
