package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/ollama"
	"github.com/andrewnageh/vocab/internal/store"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show config + DB stats + verify Ollama connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		path, _ := config.Path()
		dbPath, _ := config.DBPath()
		fmt.Printf("Config file:   %s\n", path)
		fmt.Printf("Database file: %s\n", dbPath)
		fmt.Printf("Ollama host:   %s\n", cfg.OllamaHost)
		fmt.Printf("Model:         %s\n", cfg.Model)
		fmt.Printf("Suggest count: %d\n", cfg.SuggestWordCount)
		fmt.Printf("Story count:   %d\n", cfg.StoryWordCount)
		fmt.Printf("Box intervals: %v days\n", cfg.BoxIntervalDays)

		s, err := store.Load()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer s.Close()
		fmt.Printf("Dictionary:    %d words\n", s.CountWords())
		fmt.Printf("Your collection:     %d cards\n", s.CountCards())

		fmt.Print("\nPinging Ollama… ")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := ollama.New(cfg).Ping(ctx); err != nil {
			fmt.Println("FAIL")
			return err
		}
		fmt.Println("ok")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
