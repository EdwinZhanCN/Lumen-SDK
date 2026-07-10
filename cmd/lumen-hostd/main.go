package main

import (
	"fmt"
	"os"

	hostdcmd "github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/cmd"
	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/service"

	"github.com/spf13/cobra"
)

// Build information, populated by -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	build := service.BuildInfo{Version: Version, Commit: Commit, BuildTime: BuildTime}

	root := &cobra.Command{
		Use:   "lumen-hostd",
		Short: "Lumen Host Broker: discovers Lumen inference nodes on the LAN and republishes them for applications that cannot perform local-network discovery themselves (e.g. inside Docker Desktop).",
	}

	root.AddCommand(
		hostdcmd.NewServeCommand(build),
		hostdcmd.NewVersionCommand(build),
		hostdcmd.NewInstallCommand(),
		hostdcmd.NewUninstallCommand(),
		hostdcmd.NewStartCommand(),
		hostdcmd.NewStopCommand(),
		hostdcmd.NewStatusCommand(),
		hostdcmd.NewDoctorCommand(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
