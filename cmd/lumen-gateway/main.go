package main

import (
	"context"
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
//go:embed all:frontend/dist
var assets embed.FS

// Embed the application icon for the system tray
//go:embed lumen-gateway-icon.png
var iconBytes []byte

func main() {
	// Create our gateway bridge service
	gatewayService := NewGatewayService()

	// Initialize the Wails application options
	app := application.New(application.Options{
		Name:        "Lumen Gateway",
		Description: "Lumen Distributed AI Inference Gateway",
		Services: []application.Service{
			application.NewService(gatewayService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory, // Hide from Dock, run as menu bar app
		},
	})

	// Create a frameless popover window attached to the tray
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Lumen Gateway",
		Width:           360,
		Height:          500,
		Frameless:       true, // Borderless window
		AlwaysOnTop:     true, // Keep above other windows when toggled
		Hidden:          true, // Start hidden, managed by tray
		HideOnFocusLost: true, // Auto hide when user clicks elsewhere
		HideOnEscape:    true, // Dismiss on Escape key
		URL:             "/",
	})

	// Create the system tray icon
	systray := app.SystemTray.New()
	systray.SetTemplateIcon(iconBytes)
	systray.SetTooltip("Lumen Gateway")

	// Attach the window to the tray icon
	systray.AttachWindow(mainWindow)
	systray.WindowOffset(8)
	systray.WindowDebounce(100 * time.Millisecond)

	// Set up application shutdown lifecycle to gracefully clean up
	app.OnShutdown(func() {
		if err := gatewayService.Stop(); err != nil {
			log.Printf("Error stopping Gateway core: %v", err)
		}
	})

	// Start Gateway core before app.Run()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := gatewayService.Start(ctx); err != nil {
		log.Printf("Error starting Gateway core: %v", err)
	}

	// Run the application
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
