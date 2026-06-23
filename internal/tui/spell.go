package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type spellDoneMsg struct{ err error }

func detectTTS() (bin string, prefixArgs []string, ok bool) {
	if p, err := exec.LookPath("say"); err == nil {
		return p, nil, true
	}
	if p, err := exec.LookPath("espeak"); err == nil {
		return p, []string{"-s", "140"}, true
	}
	if p, err := exec.LookPath("spd-say"); err == nil {
		return p, nil, true
	}
	return "", nil, false
}

// speakSpelling returns a tea.Cmd that pronounces the whole word using the
// first available system TTS binary.
func speakSpelling(word string) tea.Cmd {
	return func() tea.Msg {
		bin, prefix, ok := detectTTS()
		if !ok {
			return spellDoneMsg{err: fmt.Errorf("no TTS found — install espeak on Linux, say is bundled on macOS")}
		}
		payload := strings.TrimSpace(word)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		args := append(append([]string{}, prefix...), payload)
		if err := exec.CommandContext(ctx, bin, args...).Run(); err != nil {
			return spellDoneMsg{err: err}
		}
		return spellDoneMsg{err: nil}
	}
}
