package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/ollama"
	"github.com/andrewnageh/vocab/internal/store"
)

var (
	searchWordStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700"))
	searchTypeStyle    = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#A78BFA"))
	searchHeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#88AAFF"))
	searchWarnStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFA500"))
)

var searchCmd = &cobra.Command{
	Use:   "search <word> [word...]",
	Short: "Look up one or more words; AI-add them to dictionary + collection if missing",
	Args:  cobra.MinimumNArgs(1),
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

		for i, arg := range args {
			word := strings.ToLower(strings.TrimSpace(arg))
			if word == "" {
				continue
			}
			if i > 0 {
				fmt.Println()
			}
			if err := searchWord(cmd.Context(), cfg, s, word); err != nil {
				fmt.Println(searchWarnStyle.Render(fmt.Sprintf("✗ %q: %v", word, err)))
			}
		}
		return nil
	},
}

func searchWord(parentCtx context.Context, cfg config.Config, s *store.Store, word string) error {
	w, err := s.FindWord(word)
	if err != nil {
		return err
	}
	if w != nil {
		examples, err := s.Examples(w.ID)
		if err != nil {
			return err
		}
		if len(examples) == 0 {
			client := ollama.New(cfg)
			ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
			defer cancel()
			fmt.Printf("No stored examples — generating via %s…\n", cfg.Model)
			ex, err := client.MoreExamples(ctx, w.Name, w.Definition, nil)
			if err != nil {
				return fmt.Errorf("generate examples: %w", err)
			}
			if len(ex) > 0 {
				if err := s.AddExamples(w.ID, ex); err != nil {
					return err
				}
				examples = ex
			}
		}

		card, _ := s.Find(word)
		fmt.Print(searchWordStyle.Render(w.Name))
		if w.Type != "" {
			fmt.Print("  " + searchTypeStyle.Render("("+w.Type+")"))
		}
		fmt.Println()
		fmt.Printf("  %s\n", w.Definition)
		if len(examples) > 0 {
			fmt.Println("\n" + searchHeadingStyle.Render("Examples:"))
			for _, ex := range examples {
				fmt.Printf("  • %s\n", ex)
			}
		}
		if card != nil {
			fmt.Printf("\n✓ In your collection — box %d, due %s\n",
				card.Box, card.NextDueAt.Format("2006-01-02"))
		} else {
			fmt.Println("\n" + searchWarnStyle.Render("✗ Not in your collection — run `vocab add "+w.Name+"` to study it."))
		}
		return nil
	}

	// Not in the dictionary at all — AI gate, then add to both tables.
	client := ollama.New(cfg)
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()

	fmt.Printf("Not in local dictionary — validating via %s…\n", cfg.Model)
	v, err := client.ValidateEnglishWord(ctx, word)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	if !v.Valid {
		return fmt.Errorf("%q is not in the English dictionary", word)
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
	fmt.Printf("\nAdded %q to dictionary and collection.\n  %s\n", word, def.Definition)
	for _, ex := range def.Examples {
		fmt.Printf("  • %s\n", ex)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
