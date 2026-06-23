package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/andrewnageh/vocab/internal/store"
)

var listBox int

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List cards (optionally filter by Leitner box)",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Load()
		if err != nil {
			return err
		}
		defer s.Close()
		cards := s.All()
		now := time.Now()
		shown := 0
		for _, c := range cards {
			if listBox >= 0 && c.Box != listBox {
				continue
			}
			status := "  "
			if !c.NextDueAt.After(now) {
				status = "★ "
			}
			flag := ""
			if c.Forgotten(s) {
				flag = " [forgotten]"
			}
			fmt.Printf("%sbox %d  %-20s  due %s%s\n",
				status, c.Box, c.Word, c.NextDueAt.Format("2006-01-02"), flag)
			shown++
		}
		fmt.Printf("\n%d cards.\n", shown)
		return nil
	},
}

func init() {
	listCmd.Flags().IntVar(&listBox, "box", -1, "filter by Leitner box (0..4)")
	rootCmd.AddCommand(listCmd)
}

