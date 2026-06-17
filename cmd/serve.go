package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"oapi/internal/config"
	"oapi/internal/proxy"
	"oapi/internal/rotation"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the proxy server in headless mode",
	Long:  `Start the proxy server in headless (daemon) mode, routing requests and managing keys without the TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolveConfigPath(configPath)
		if err != nil {
			return fmt.Errorf("failed to resolve config path: %w", err)
		}

		// Ensure config directory exists
		configDir := filepath.Dir(path)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Setup log file
		logPath := filepath.Join(configDir, "oapi.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", logPath, err)
		}
		defer logFile.Close()

		// Log to both stderr and log file
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))

		// Load config
		cfg, err := config.LoadConfig(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: config file not found at %s. Please run 'oapi add-key' or launch the TUI to configure.\n", path)
				os.Exit(1)
			}
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Port override
		if portOverride > 0 {
			cfg.Server.Port = portOverride
		}

		// NotifyContext to catch interrupt signals
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Load state
		stateMgr := config.NewStateManager(path)
		if err := stateMgr.LoadState(); err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		// Initialize key pool and rotation engine
		pool := rotation.NewKeyPool(ctx, cfg, path, stateMgr)
		engine := rotation.NewRotationEngine(pool)

		// Create and start proxy server
		srv := proxy.NewServer(cfg, path, stateMgr, pool, engine)
		if err := srv.Start(ctx); err != nil {
			return fmt.Errorf("failed to start proxy server: %w", err)
		}

		log.Println("Headless proxy server started. Press Ctrl+C to stop.")

		// Block until context is canceled (SIGINT/SIGTERM)
		<-ctx.Done()
		cancel() // Release signal resources

		log.Println("Stopping proxy server...")
		<-srv.Stopped() // Wait for internal server cleanup and Stop() execution to finish

		log.Println("Proxy server gracefully stopped.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
