package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"oapi/internal/config"
	"oapi/internal/registry"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var addKeyCmd = &cobra.Command{
	Use:   "add-key",
	Short: "Add an API key interactively",
	Long:  `Interactively add a new LLM provider API key to the configuration using a terminal form.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolveConfigPath(configPath)
		if err != nil {
			return err
		}

		// Load config. If it doesn't exist, create a default config
		cfg, err := config.LoadConfig(path)
		if err != nil {
			if os.IsNotExist(err) {
				cfg = config.DefaultConfig()
			} else {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		// 1. Prompt for Provider
		var providerChoice string
		providerOptions := []huh.Option[string]{}
		var providerKeys []string
		for k := range registry.Providers {
			providerKeys = append(providerKeys, k)
		}
		sort.Strings(providerKeys)
		for _, pk := range providerKeys {
			providerOptions = append(providerOptions, huh.NewOption(pk, pk))
		}
		providerOptions = append(providerOptions, huh.NewOption("custom provider", "custom"))

		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select LLM Provider").
					Options(providerOptions...).
					Value(&providerChoice),
			),
		).Run()
		if err != nil {
			return err
		}

		var provider string
		if providerChoice == "custom" {
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Enter custom provider ID").
						Placeholder("e.g. anthropic").
						Value(&provider).
						Validate(func(str string) error {
							if strings.TrimSpace(str) == "" {
								return fmt.Errorf("provider cannot be empty")
							}
							return nil
						}),
				),
			).Run()
			if err != nil {
				return err
			}
			provider = strings.TrimSpace(strings.ToLower(provider))
		} else {
			provider = providerChoice
		}

		// 2. Prompt for Model
		var model string
		recommended := registry.RecommendedModels[provider]
		if len(recommended) > 0 {
			var modelChoice string
			modelOptions := []huh.Option[string]{}
			for _, m := range recommended {
				modelOptions = append(modelOptions, huh.NewOption(m, m))
			}
			modelOptions = append(modelOptions, huh.NewOption("enter custom model name", "custom"))

			err = huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Select Model").
						Options(modelOptions...).
						Value(&modelChoice),
				),
			).Run()
			if err != nil {
				return err
			}

			if modelChoice == "custom" {
				err = huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Enter custom model name").
							Value(&model).
							Validate(func(str string) error {
								if strings.TrimSpace(str) == "" {
									return fmt.Errorf("model cannot be empty")
								}
								return nil
							}),
					),
				).Run()
				if err != nil {
					return err
				}
				model = strings.TrimSpace(model)
			} else {
				model = modelChoice
			}
		} else {
			// Custom or no recommended models
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Enter model name").
						Value(&model).
						Validate(func(str string) error {
							if strings.TrimSpace(str) == "" {
								return fmt.Errorf("model cannot be empty")
							}
							return nil
						}),
				),
			).Run()
			if err != nil {
				return err
			}
			model = strings.TrimSpace(model)
		}

		// 3. Prompt for API Key
		var apiKey string
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter API Key").
					Password(true).
					Value(&apiKey).
					Validate(func(str string) error {
						if strings.TrimSpace(str) == "" {
							return fmt.Errorf("API key cannot be empty")
						}
						return nil
					}),
			),
		).Run()
		if err != nil {
			return err
		}
		apiKey = strings.TrimSpace(apiKey)

		// 4. Resolve default limits
		defaultRPM := 0
		defaultRPD := 0
		if provInfo, ok := registry.Providers[provider]; ok {
			defaultRPM = provInfo.DefaultRPM
			defaultRPD = provInfo.DefaultRPD
		}

		var rpmStr string
		var rpdStr string
		var tpmStr string
		var maxTokensStr string

		if defaultRPM > 0 {
			rpmStr = strconv.Itoa(defaultRPM)
		}
		if defaultRPD > 0 {
			rpdStr = strconv.Itoa(defaultRPD)
		}

		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("RPM Limit (Requests Per Minute)").
					Placeholder("leave blank for no limit").
					Value(&rpmStr),
				huh.NewInput().
					Title("RPD Limit (Requests Per Day)").
					Placeholder("leave blank for no limit").
					Value(&rpdStr),
				huh.NewInput().
					Title("TPM Limit (Tokens Per Minute - Optional)").
					Placeholder("leave blank for no limit").
					Value(&tpmStr),
				huh.NewInput().
					Title("Max Completion Tokens Override (Optional)").
					Placeholder("leave blank for default").
					Value(&maxTokensStr),
			),
		).Run()
		if err != nil {
			return err
		}

		rpm, _ := strconv.Atoi(rpmStr)
		rpd, _ := strconv.Atoi(rpdStr)
		tpm, _ := strconv.Atoi(tpmStr)
		maxTokens, _ := strconv.Atoi(maxTokensStr)

		// Generate ID
		providerCount := 0
		for _, k := range cfg.Keys {
			if k.Provider == provider {
				providerCount++
			}
		}
		keyID := fmt.Sprintf("%s_%d", provider, providerCount+1)

		keyConfig := config.KeyConfig{
			ID:                  keyID,
			Provider:            provider,
			Model:               model,
			APIKey:              apiKey,
			RPMLimit:            rpm,
			RPDLimit:            rpd,
			TPMLimit:            tpm,
			MaxCompletionTokens: maxTokens,
			Status:              "active",
		}

		cfg.Keys = append(cfg.Keys, keyConfig)

		err = config.SaveConfig(path, cfg)
		if err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Successfully added key %s to config %s\n", keyID, path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addKeyCmd)
}
