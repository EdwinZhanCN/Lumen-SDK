package cmd

import (
	"context"
	"fmt"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/internal"
	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/service"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewServeCommand runs the Host Broker as a foreground process. Native
// service managers (launchd, systemd, Task Scheduler) invoke this directly;
// it does not detach, fork, or write a PID file — the OS service manager
// owns restart and lifecycle instead.
func NewServeCommand(build service.BuildInfo) *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Host Broker in the foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(configFile, build)
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "Path to configuration file")
	return cmd
}

func runServe(configFile string, build service.BuildInfo) error {
	cfg, err := internal.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger, err := createLogger(cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	hostdService, err := service.NewHostdService(cfg, build, logger)
	if err != nil {
		return fmt.Errorf("failed to create hostd service: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := hostdService.Start(ctx); err != nil {
		return fmt.Errorf("failed to start hostd service: %w", err)
	}

	logger.Info("Lumen Host Broker started successfully",
		zap.String("config", configFile),
		zap.String("version", build.Version))

	hostdService.WaitForShutdown()

	logger.Info("Lumen Host Broker stopped gracefully")
	return nil
}

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

	switch cfg.Output {
	case "stdout":
		zapConfig.OutputPaths = []string{"stdout"}
	case "stderr":
		zapConfig.OutputPaths = []string{"stderr"}
	default:
		zapConfig.OutputPaths = []string{cfg.Output}
	}

	return zapConfig.Build()
}
