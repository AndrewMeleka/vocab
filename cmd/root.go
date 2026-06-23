package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vocab",
	Short: "AI-powered spaced-repetition vocab CLI",
	Long: `vocab is a terminal vocabulary trainer.

It uses a Leitner spaced-repetition algorithm to schedule reviews and an Ollama
LLM as both dictionary and tutor (definitions, examples, word of the day,
micro-stories).

Run with no arguments to open the interactive dashboard.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDashboard()
	},
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
