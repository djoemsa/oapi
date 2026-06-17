package cmd

import (
	"fmt"
	"os"

	"oapi/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print current config path and contents",
	Long:  `Print the resolved file path of oapi.yaml and dump its contents with API keys masked.`,
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

		fmt.Printf("Config File Path: %s\n\n", path)

		// Create a copy of the config with masked keys for display
		displayCfg := *cfg
		displayCfg.Keys = make([]config.KeyConfig, len(cfg.Keys))
		for i, k := range cfg.Keys {
			displayCfg.Keys[i] = k
			displayCfg.Keys[i].APIKey = config.MaskAPIKey(k.APIKey)
		}

		yamlBytes, err := yaml.Marshal(displayCfg)
		if err != nil {
			return fmt.Errorf("failed to marshal config for display: %w", err)
		}

		fmt.Println(string(yamlBytes))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
