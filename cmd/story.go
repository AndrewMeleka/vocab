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

var storyCmd = &cobra.Command{
	Use:   "story",
	Short: "Generate a micro-story using recent / due words",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		s, err := store.Load()
		if err != nil {
			return err
		}
		defer s.Close()
		pool := s.Due(time.Now())
		if len(pool) < cfg.StoryWordCount {
			pool = append(pool, s.Recent(cfg.StoryWordCount)...)
		}
		seen := map[string]bool{}
		var words []string
		for _, c := range pool {
			if seen[c.Word] {
				continue
			}
			seen[c.Word] = true
			words = append(words, c.Word)
			if len(words) >= cfg.StoryWordCount {
				break
			}
		}
		if len(words) == 0 {
			return fmt.Errorf("no words to weave into a story — add some first")
		}
		client := ollama.New(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		fmt.Printf("Weaving: %v\n\n", words)
		story, err := client.MicroStory(ctx, words)
		if err != nil {
			return err
		}
		fmt.Println(story)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(storyCmd)
}
