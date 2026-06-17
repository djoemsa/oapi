package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"oapi/internal/config"
	"oapi/internal/rotation"
	"oapi/internal/tui"

	"github.com/spf13/cobra"
)

var (
	configPath   string
	portOverride int
)

var rootCmd = &cobra.Command{
	Use:   "oapi",
	Short: "oapi is a local reverse proxy with key pool management",
	Long:  `oapi is an OpenAI-compatible reverse proxy that rotates API keys and tracks rate limits.`,
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

		// For TUI, log only to the log file to avoid corrupting the screen
		log.SetOutput(logFile)

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

		// Run TUI
		err = tui.Run(tui.TUIConfig{
			Cfg:        cfg,
			ConfigPath: path,
			StateMgr:   stateMgr,
			Pool:       pool,
			Engine:     engine,
			Ctx:        ctx,
			Cancel:     cancel,
		})
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "path to config file (oapi.yaml)")
	rootCmd.PersistentFlags().IntVar(&portOverride, "port", 0, "override proxy server port")
}

