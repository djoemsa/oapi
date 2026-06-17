package cmd

import (
	"fmt"
	"os"

	"oapi/internal/config"
	"oapi/internal/testutil"

	"github.com/spf13/cobra"
)

var testKeysCmd = &cobra.Command{
	Use:   "test-keys",
	Short: "Test all configured API keys",
	Long:  `Run a connection test on each configured API key in the configuration to check if it's active.`,
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

		if len(cfg.Keys) == 0 {
			fmt.Println("No keys configured.")
			return nil
		}

		client := testutil.DefaultHTTPClient()
		fmt.Printf("Testing %d configured keys...\n", len(cfg.Keys))

		for _, key := range cfg.Keys {
			maskedKey := config.MaskAPIKey(key.APIKey)
			fmt.Printf("Key: %s [%s/%s] (key: %s) -> ", key.ID, key.Provider, key.Model, maskedKey)
			status, err := testutil.ProbeKey(client, key)
			if err != nil {
				fmt.Printf("FAIL: %v (Status marked: %s)\n", err, status)
			} else {
				fmt.Printf("SUCCESS (Status marked: %s)\n", status)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testKeysCmd)
}
