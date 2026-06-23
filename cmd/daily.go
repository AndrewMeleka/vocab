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

var (
	dailyAccept bool
	dailyLevel  string
)

var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Show word of the day + suggest new words from the local dictionary",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if dailyLevel != "" {
			levels, err := parseLevels(dailyLevel)
			if err != nil {
				return err
			}
			cfg.Level = levels
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("persist level: %w", err)
			}
		}
		s, err := store.Load()
		if err != nil {
			return err
		}
		defer s.Close()
		client := ollama.New(cfg)

		printWordOfDay(s, client)
		return printSuggestions(s, client, cfg)
	},
}

func printWordOfDay(s *store.Store, client *ollama.Client) {
	pick := s.RandomDueCard(time.Now())
	if pick == nil {
		recent := s.Recent(1)
		if len(recent) == 0 {
			fmt.Println("No cards yet — add some with `vocab add <word>`.")
			fmt.Println()
			return
		}
		pick = &recent[0]
	}
	fmt.Printf("Word of the day: %s\n", pick.Word)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	wod, err := client.PickWordOfDay(ctx, []string{pick.Word})
	if err == nil && strings.EqualFold(wod.Word, pick.Word) && wod.Reason != "" {
		fmt.Printf("  — %s\n\n", wod.Reason)
		return
	}
	fmt.Printf("  %s\n\n", pick.Definition)
}

func printSuggestions(s *store.Store, client *ollama.Client, cfg config.Config) error {
	words, fellBack, err := s.SampleNewWords(cfg.DailyWordCount, cfg.Level)
	if err != nil {
		return fmt.Errorf("sample new words: %w", err)
	}
	if len(words) == 0 {
		fmt.Println("No new words to suggest — the dictionary is empty or you've added everything!")
		return nil
	}
	label := strings.Join(cfg.Level, ",")
	if fellBack {
		fmt.Printf("(no words tagged at level %s yet — falling back to any level)\n", label)
		label = "any"
	}
	fmt.Printf("Suggested new words (%d) at level %s:\n", len(words), label)
	for i, w := range words {
		fmt.Printf("  %d. %s — %s\n", i+1, w.Name, truncate(w.Definition, 70))
	}

	if !dailyAccept {
		fmt.Println("\nRun with --accept to add all suggestions to your collection.")
		return nil
	}
	added := 0
	for _, w := range words {
		if _, err := s.CreateCard(w.ID, time.Now()); err != nil {
			fmt.Printf("  skip %s: %v\n", w.Name, err)
			continue
		}
		added++
	}
	fmt.Printf("\nAdded %d new cards.\n", added)
	_ = client // reserved for future on-demand enrichment of examples
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func init() {
	dailyCmd.Flags().BoolVar(&dailyAccept, "accept", false, "add all suggestions to your collection")
	dailyCmd.Flags().StringVar(&dailyLevel, "level", "", "CEFR level(s) for suggestions, comma-separated (a1,a2,b1,b2,c1,c2). Persisted to config.toml.")
	rootCmd.AddCommand(dailyCmd)
}

// parseLevels splits a comma-separated CEFR list, validates, lowercases, dedupes.
func parseLevels(raw string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		lv := strings.ToLower(strings.TrimSpace(part))
		if lv == "" {
			continue
		}
		if !config.ValidLevels[lv] {
			return nil, fmt.Errorf("invalid level %q (allowed: a1, a2, b1, b2, c1, c2)", lv)
		}
		if seen[lv] {
			continue
		}
		seen[lv] = true
		out = append(out, lv)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid levels in %q", raw)
	}
	return out, nil
}
