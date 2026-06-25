package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	OllamaHost      string   `toml:"ollama_host"`
	Model           string   `toml:"model"`
	DailyWordCount  int      `toml:"daily_word_count"`
	StoryWordCount  int      `toml:"story_word_count"`
	BoxIntervalDays []int    `toml:"box_interval_days"`
}

func defaults() Config {
	return Config{
		OllamaHost:      "http://localhost:11434",
		Model:           "llama3.2",
		DailyWordCount:  3,
		StoryWordCount:  5,
		BoxIntervalDays: []int{1, 3, 7, 14, 30},
	}
}

// Dir returns ~/.config/vocab, creating it if missing.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "vocab")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func HistoryPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

func DBPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vocab.db"), nil
}

// Load reads config.toml, writing defaults on first run.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	cfg := defaults()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := Save(cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("decode %s: %w", path, err)
	}
	if cfg.OllamaHost == "" {
		cfg.OllamaHost = defaults().OllamaHost
	}
	if cfg.Model == "" {
		cfg.Model = defaults().Model
	}
	if cfg.DailyWordCount <= 0 {
		cfg.DailyWordCount = defaults().DailyWordCount
	}
	if cfg.StoryWordCount <= 0 {
		cfg.StoryWordCount = defaults().StoryWordCount
	}
	if len(cfg.BoxIntervalDays) == 0 {
		cfg.BoxIntervalDays = defaults().BoxIntervalDays
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	return enc.Encode(cfg)
}
