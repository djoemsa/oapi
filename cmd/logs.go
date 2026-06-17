package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"oapi/internal/config"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail proxy server logs",
	Long:  `Print existing logs and tail new entries in real time from oapi.log.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolveConfigPath(configPath)
		if err != nil {
			return err
		}

		// Ensure config exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: config file not found at %s. Please run 'oapi add-key' or use the TUI to configure.\n", path)
			os.Exit(1)
		}

		configDir := filepath.Dir(path)
		logPath := filepath.Join(configDir, "oapi.log")

		// If log file doesn't exist yet, wait for it
		for {
			if _, err := os.Stat(logPath); err == nil {
				break
			}
			fmt.Println("Waiting for log file to be created...")
			time.Sleep(2 * time.Second)
		}

		file, err := os.Open(logPath)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		// Copy existing contents to stdout
		_, err = io.Copy(os.Stdout, file)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to read log file: %w", err)
		}

		info, err := file.Stat()
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to get log file info: %w", err)
		}
		offset := info.Size()
		file.Close()

		// Polling loop to tail new logs
		for {
			curInfo, err := os.Stat(logPath)
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if curInfo.Size() < offset {
				// File was truncated/rotated, reset offset to start
				offset = 0
			}

			if curInfo.Size() > offset {
				f, err := os.Open(logPath)
				if err == nil {
					_, err = f.Seek(offset, io.SeekStart)
					if err == nil {
						_, err = io.Copy(os.Stdout, f)
						if err == nil {
							offset = curInfo.Size()
						}
					}
					f.Close()
				}
			}

			time.Sleep(500 * time.Millisecond)
		}
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
