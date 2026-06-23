package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/ollama"
	"github.com/andrewnageh/vocab/internal/store"
	"github.com/andrewnageh/vocab/internal/tui"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review all cards due today",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		s, err := store.Load()
		if err != nil {
			return err
		}
		due := s.Due(time.Now())
		if len(due) == 0 {
			fmt.Println("Nothing due — come back later.")
			return nil
		}
		client := ollama.New(cfg)
		return tui.RunReview(s, cfg, client, due)
	},
}

func init() {
	rootCmd.AddCommand(reviewCmd)
}
