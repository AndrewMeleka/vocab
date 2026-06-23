package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andrewnageh/vocab/internal/store"
)

var resetYes bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete every card from your collection (dictionary kept)",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Load()
		if err != nil {
			return err
		}
		defer s.Close()
		count := s.CountCards()
		if count == 0 {
			fmt.Println("Collection is already empty.")
			return nil
		}
		if !resetYes {
			fmt.Printf("This will delete %d card(s) and their review history.\n", count)
			fmt.Print("Type 'yes' to confirm: ")
			r := bufio.NewReader(os.Stdin)
			line, _ := r.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(line)) != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}
		n, err := s.ResetCards()
		if err != nil {
			return err
		}
		fmt.Printf("Deleted %d card(s).\n", n)
		return nil
	},
}

func init() {
	resetCmd.Flags().BoolVar(&resetYes, "yes", false, "skip interactive confirmation")
	rootCmd.AddCommand(resetCmd)
}
