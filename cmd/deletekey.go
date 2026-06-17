package cmd

import (
	"fmt"
	"os"

	"oapi/internal/config"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var deleteKeyCmd = &cobra.Command{
	Use:     "delete-key [key-id]",
	Aliases: []string{"remove-key", "delete"},
	Short:   "Delete a configured API key",
	Long:    `Delete an LLM provider API key from the configuration. You can pass the key ID as an argument or select it interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolveConfigPath(configPath)
		if err != nil {
			return err
		}

		cfg, err := config.LoadConfig(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: config file not found at %s.\n", path)
				os.Exit(1)
			}
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(cfg.Keys) == 0 {
			fmt.Println("No keys configured.")
			return nil
		}

		var targetID string
		if len(args) > 0 {
			targetID = args[0]
		} else {
			// Interactive select
			options := []huh.Option[string]{}
			for _, k := range cfg.Keys {
				label := fmt.Sprintf("%s (%s/%s)", k.ID, k.Provider, k.Model)
				options = append(options, huh.NewOption(label, k.ID))
			}

			err = huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Select Key to Delete").
						Options(options...).
						Value(&targetID),
				),
			).Run()
			if err != nil {
				return err
			}
		}

		// Find key
		idx := -1
		for i, k := range cfg.Keys {
			if k.ID == targetID {
				idx = i
				break
			}
		}

		if idx == -1 {
			return fmt.Errorf("key with ID '%s' not found", targetID)
		}

		// Confirm delete
		confirm := true
		if len(args) == 0 {
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("Are you sure you want to delete key '%s'?", targetID)).
						Value(&confirm),
				),
			).Run()
			if err != nil {
				return err
			}
		}

		if !confirm {
			fmt.Println("Deletion cancelled.")
			return nil
		}

		// Perform delete
		cfg.Keys = append(cfg.Keys[:idx], cfg.Keys[idx+1:]...)

		err = config.SaveConfig(path, cfg)
		if err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Successfully deleted key '%s' from configuration %s\n", targetID, path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteKeyCmd)
}
