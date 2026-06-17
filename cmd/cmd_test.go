package cmd

import (
	"testing"
)

func TestSubcommands(t *testing.T) {
	expectedCmds := map[string]bool{
		"serve":      true,
		"add-key":    true,
		"delete-key": true,
		"test-keys":  true,
		"status":     true,
		"config":     true,
		"logs":       true,
		"version":    true,
	}

	for _, c := range rootCmd.Commands() {
		delete(expectedCmds, c.Name())
	}

	if len(expectedCmds) > 0 {
		t.Errorf("missing subcommands: %v", expectedCmds)
	}
}

func TestPersistentFlags(t *testing.T) {
	cfgFlag := rootCmd.PersistentFlags().Lookup("config")
	if cfgFlag == nil {
		t.Error("expected persistent flag --config to be defined")
	}

	portFlag := rootCmd.PersistentFlags().Lookup("port")
	if portFlag == nil {
		t.Error("expected persistent flag --port to be defined")
	}
}
