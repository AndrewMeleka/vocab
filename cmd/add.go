package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/ollama"
	"github.com/andrewnageh/vocab/internal/store"
)

var addCmd = &cobra.Command{
	Use:   "add <word>",
	Short: "Add a word to your collection (looked up in local dictionary first, AI fallback)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		word := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		s, err := store.Load()
		if err != nil {
			return err
		}
		defer s.Close()

		existingWord, err := s.FindWord(word)
		if err != nil {
			return err
		}

		if existingWord != nil {
			cardID, err := s.CreateCard(existingWord.ID, time.Now())
			if err != nil {
				return err
			}
			fmt.Printf("Added %q from local dictionary (card id %d)\n  %s\n", existingWord.Name, cardID, existingWord.Definition)
			return nil
		}

		client := ollama.New(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		fmt.Printf("Word not in local dictionary — validating via %s…\n", cfg.Model)
		v, err := client.ValidateEnglishWord(ctx, word)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		if !v.Valid {
			return fmt.Errorf("%q is not in the English dictionary — refusing to add", word)
		}

		fmt.Printf("Fetching definition + examples…\n")
		def, err := client.Define(ctx, word)
		if err != nil {
			return fmt.Errorf("define: %w", err)
		}

		wordID, err := s.InsertWord(word, def.Definition, v.Type)
		if err != nil {
			return err
		}
		if err := s.AddExamples(wordID, def.Examples); err != nil {
			return err
		}
		if _, err := s.CreateCard(wordID, time.Now()); err != nil {
			return err
		}

		fmt.Printf("\nAdded %q\n  %s\n", word, def.Definition)
		for _, ex := range def.Examples {
			fmt.Printf("  • %s\n", ex)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
