package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrewnageh/vocab/internal/tts"
)

type spellDoneMsg struct{ err error }

// speakSpelling returns a tea.Cmd that pronounces the whole word using the
// first available system TTS binary.
func speakSpelling(word string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return spellDoneMsg{err: tts.Speak(ctx, strings.TrimSpace(word))}
	}
}
