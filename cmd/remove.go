package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andrewnageh/vocab/internal/store"
)

var removeCmd = &cobra.Command{
	Use:     "remove <word>",
	Aliases: []string{"rm"},
	Short:   "Remove a single card from your collection (dictionary entry kept)",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		word := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
		s, err := store.Load()
		if err != nil {
			return err
		}
		defer s.Close()
		deleted, err := s.DeleteCardByWord(word)
		if err != nil {
			return err
		}
		if !deleted {
			return fmt.Errorf("%q is not in your collection", word)
		}
		fmt.Printf("Removed %q from your collection.\n", word)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
