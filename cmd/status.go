package cmd

import (
	"fmt"
	"os"
	"time"

	"oapi/internal/config"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print proxy server status",
	Long:  `Print statistics and key pool health status, including uptime, requests, and cooling state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolveConfigPath(configPath)
		if err != nil {
			return err
		}

		cfg, err := config.LoadConfig(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: config file not found at %s. Please run 'oapi add-key' or use the TUI to configure.\n", path)
				os.Exit(1)
			}
			return fmt.Errorf("failed to load config: %w", err)
		}

		stateMgr := config.NewStateManager(path)
		if err := stateMgr.LoadState(); err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		state := stateMgr.GetState()

		fmt.Println("OAPI Proxy Server Status:")
		fmt.Printf("  Uptime Start: %v\n", state.UptimeStart.Format(time.RFC1123))
		fmt.Printf("  Uptime:       %v\n", time.Since(state.UptimeStart).Round(time.Second))
		fmt.Printf("  Total Requests Today: %d\n", state.TotalRequestsToday)
		fmt.Println("\nConfigured Keys:")

		if len(cfg.Keys) == 0 {
			fmt.Println("  (No keys configured)")
			return nil
		}

		for _, k := range cfg.Keys {
			maskedKey := config.MaskAPIKey(k.APIKey)
			keyState, ok := state.Keys[k.ID]

			statusStr := k.Status
			var coolingInfo string
			if ok {
				if keyState.CoolingUntil != nil && keyState.CoolingUntil.After(time.Now()) {
					statusStr = "cooling"
					coolingInfo = fmt.Sprintf(" (cooling until %s)", keyState.CoolingUntil.Format("15:04:05"))
				}
				fmt.Printf("  - ID: %s\n", k.ID)
				fmt.Printf("    Provider:    %s\n", k.Provider)
				fmt.Printf("    Model:       %s\n", k.Model)
				fmt.Printf("    API Key:     %s\n", maskedKey)
				fmt.Printf("    Status:      %s%s\n", statusStr, coolingInfo)
				fmt.Printf("    Reqs Today:  %d\n", keyState.RequestsToday)
				fmt.Printf("    Reqs Minute: %d\n", keyState.RequestsThisMinute)
				if !keyState.LastUsed.IsZero() {
					fmt.Printf("    Last Used:   %s\n", keyState.LastUsed.Format("15:04:05"))
				} else {
					fmt.Println("    Last Used:   never")
				}
			} else {
				fmt.Printf("  - ID: %s\n", k.ID)
				fmt.Printf("    Provider:    %s\n", k.Provider)
				fmt.Printf("    Model:       %s\n", k.Model)
				fmt.Printf("    API Key:     %s\n", maskedKey)
				fmt.Printf("    Status:      %s (untested/unused)\n", statusStr)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
