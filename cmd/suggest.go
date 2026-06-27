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
	suggestAdd   bool
	suggestTopic string
	suggestCount int
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest new words to study, anytime (from the local dictionary, or AI by topic)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if suggestCount > 0 {
			cfg.SuggestWordCount = suggestCount
		}
		s, err := store.Load()
		if err != nil {
			return err
		}
		defer s.Close()
		client := ollama.New(cfg)

		return printSuggestions(cmd.Context(), s, client, cfg)
	},
}

// suggestion is a display-ready word, either sampled from the local dictionary
// (wordID set) or produced by the AI for a topic (wordID 0 until defined).
type suggestion struct {
	word   string
	desc   string
	wordID int64
}

func printSuggestions(ctx context.Context, s *store.Store, client *ollama.Client, cfg config.Config) error {
	var (
		suggestions []suggestion
		heading     string
	)

	if suggestTopic != "" {
		fmt.Printf("Asking %s for words about %q…\n", cfg.Model, suggestTopic)

		// Seed the exclusion set with words already in the collection so we never
		// suggest a word the user already studies. On error, fall back to empty.
		excluded := map[string]bool{}
		if known, err := s.CollectionWords(); err != nil {
			fmt.Printf("  %s\n", searchWarnStyle.Render(fmt.Sprintf("could not read collection: %v", err)))
		} else {
			for _, w := range known {
				excluded[w] = true
			}
		}

		target := cfg.SuggestWordCount
		// Ask in bounded batches rather than for the whole remainder at once: a
		// single huge request truncates and the model repeats itself, so smaller
		// rounds top up the list more reliably.
		const (
			maxRounds = 20
			batchSize = 40
			maxStalls = 3
		)
		broaden := false
		stalls := 0
		for round := 0; round < maxRounds; round++ {
			need := target - len(suggestions)
			if need <= 0 {
				break
			}
			ask := need
			if ask > batchSize {
				ask = batchSize
			}
			picks, err := suggestTopicRound(ctx, client, suggestTopic, ask, excludedKeys(excluded), broaden)
			if err != nil {
				return fmt.Errorf("suggest topic words: %w", err)
			}
			before := len(suggestions)
			for _, p := range picks {
				norm := strings.ToLower(strings.TrimSpace(p.Word))
				if norm == "" || excluded[norm] {
					continue
				}
				excluded[norm] = true
				suggestions = append(suggestions, suggestion{word: p.Word, desc: p.Hint})
			}
			if len(suggestions) == before {
				// No new words this round. First widen the relevance bar to pull in
				// adjacent vocabulary; only give up if broadening also dries up.
				stalls++
				if !broaden {
					broaden = true
					stalls = 0
				} else if stalls >= maxStalls {
					break
				}
			} else {
				stalls = 0
			}
		}
		if len(suggestions) < target {
			fmt.Printf("  %s\n", searchWarnStyle.Render(fmt.Sprintf(
				"got %d of %d — %q seems exhausted of distinct new words.",
				len(suggestions), target, suggestTopic)))
		}
		heading = fmt.Sprintf("Suggested words for %q (%d):", suggestTopic, len(suggestions))
	} else {
		words, err := s.SampleNewWords(cfg.SuggestWordCount)
		if err != nil {
			return fmt.Errorf("sample new words: %w", err)
		}
		for _, w := range words {
			suggestions = append(suggestions, suggestion{word: w.Name, desc: w.Definition, wordID: w.ID})
		}
		heading = fmt.Sprintf("Suggested new words (%d):", len(suggestions))
	}

	if len(suggestions) == 0 {
		fmt.Println(searchWarnStyle.Render("No new words to suggest — the dictionary is empty or you've added everything!"))
		return nil
	}

	fmt.Println(searchHeadingStyle.Render(heading))
	for i, sg := range suggestions {
		fmt.Printf("  %d. %s — %s\n", i+1, searchWordStyle.Render(sg.word), truncate(sg.desc, 70))
	}

	if !suggestAdd {
		fmt.Println("\n" + searchWarnStyle.Render("Run with --add to add all suggestions to your collection."))
		return nil
	}

	added := 0
	for _, sg := range suggestions {
		wordID := sg.wordID
		if wordID == 0 {
			id, err := ensureWord(ctx, client, cfg, s, sg.word)
			if err != nil {
				fmt.Printf("  %s\n", searchWarnStyle.Render(fmt.Sprintf("skip %s: %v", sg.word, err)))
				continue
			}
			wordID = id
		}
		// suggestTopic is "" for local-dictionary suggestions, which stores NULL.
		if _, err := s.CreateTopicCard(wordID, time.Now(), suggestTopic); err != nil {
			fmt.Printf("  %s\n", searchWarnStyle.Render(fmt.Sprintf("skip %s: %v", sg.word, err)))
			continue
		}
		added++
	}
	fmt.Printf("\n%s\n", searchHeadingStyle.Render(fmt.Sprintf("Added %d new cards.", added)))
	return nil
}

// suggestTopicRound runs a single SuggestTopic call under its own timeout so the
// top-up loop can issue multiple rounds without leaking contexts.
func suggestTopicRound(parentCtx context.Context, client *ollama.Client, topic string, n int, exclude []string, broaden bool) ([]ollama.Suggestion, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()
	return client.SuggestTopic(ctx, topic, n, exclude, broaden)
}

// excludedKeys returns the keys of set as a slice for prompt construction.
func excludedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// ensureWord returns the dictionary id for word, AI-defining and inserting it
// (with examples) when it is not already in the local dictionary.
func ensureWord(parentCtx context.Context, client *ollama.Client, cfg config.Config, s *store.Store, word string) (int64, error) {
	if w, err := s.FindWord(word); err != nil {
		return 0, err
	} else if w != nil {
		return w.ID, nil
	}
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()
	def, err := client.Define(ctx, word)
	if err != nil {
		return 0, fmt.Errorf("define: %w", err)
	}
	id, err := s.InsertWord(word, def.Definition, "")
	if err != nil {
		return 0, err
	}
	if err := s.AddExamples(id, def.Examples); err != nil {
		return 0, err
	}
	return id, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func init() {
	suggestCmd.Flags().BoolVar(&suggestAdd, "add", false, "add all suggestions to your collection")
	suggestCmd.Flags().StringVar(&suggestTopic, "topic", "", "use AI to suggest words about a topic (e.g. --topic kitchen)")
	suggestCmd.Flags().IntVar(&suggestCount, "count", 0, "how many words to suggest (default from config, 3)")
	rootCmd.AddCommand(suggestCmd)
}
