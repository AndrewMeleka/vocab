package cmd

import (
	"time"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/ollama"
	"github.com/andrewnageh/vocab/internal/store"
	"github.com/andrewnageh/vocab/internal/tui"
)

func runDashboard() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := store.Load()
	if err != nil {
		return err
	}
	client := ollama.New(cfg)
	return tui.RunDashboard(s, cfg, client, time.Now())
}
