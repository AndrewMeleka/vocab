package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/leitner"
	"github.com/andrewnageh/vocab/internal/ollama"
	"github.com/andrewnageh/vocab/internal/store"
)

type reviewModel struct {
	store    *store.Store
	cfg      config.Config
	client   *ollama.Client
	queue    []store.Card
	idx      int
	revealed bool
	loading  bool
	speaking bool
	flash    string
	done     int
}

func RunReview(s *store.Store, cfg config.Config, client *ollama.Client, due []store.Card) error {
	m := reviewModel{store: s, cfg: cfg, client: client, queue: due}
	_, err := tea.NewProgram(m).Run()
	return err
}

type examplesMsg struct {
	word string
	ex   []string
	err  error
}

func fetchExamples(client *ollama.Client, c store.Card) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ex, err := client.MoreExamples(ctx, c.Word, c.Definition, c.Examples)
		return examplesMsg{word: c.Word, ex: ex, err: err}
	}
}

func (m reviewModel) Init() tea.Cmd { return nil }

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.idx >= len(m.queue) {
			return m, tea.Quit
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			_ = m.store.Save()
			return m, tea.Quit
		case " ", "enter":
			m.revealed = true
		case "j":
			if m.revealed {
				m.applyResult(store.ResultKnew)
				m.flash = okStyle.Render("✓ knew")
			}
		case "f":
			if m.revealed {
				m.applyResult(store.ResultForgot)
				m.flash = errStyle.Render("✗ forgot")
			}
		case "e":
			if m.revealed && !m.loading {
				m.loading = true
				return m, fetchExamples(m.client, m.queue[m.idx])
			}
		case "s":
			if !m.speaking {
				m.speaking = true
				return m, speakSpelling(m.queue[m.idx].Word)
			}
		}
	case spellDoneMsg:
		m.speaking = false
		if msg.err != nil {
			m.flash = errStyle.Render(fmt.Sprintf("spell: %v", msg.err))
		}
	case examplesMsg:
		m.loading = false
		if msg.err != nil {
			m.flash = errStyle.Render(fmt.Sprintf("examples: %v", msg.err))
			return m, nil
		}
		card := &m.queue[m.idx]
		card.Examples = append(card.Examples, msg.ex...)
		if err := m.store.AddExamples(card.WordID, msg.ex); err != nil {
			m.flash = errStyle.Render(fmt.Sprintf("save examples: %v", err))
		}
	}
	return m, nil
}

func (m *reviewModel) applyResult(r store.ReviewResult) {
	card := m.queue[m.idx]
	updated, ev := leitner.Apply(card, r, time.Now(), m.cfg.BoxIntervalDays)
	if err := m.store.Upsert(updated); err != nil {
		m.flash = errStyle.Render(fmt.Sprintf("save: %v", err))
	}
	if err := m.store.RecordReview(updated.ID, ev); err != nil {
		m.flash = errStyle.Render(fmt.Sprintf("review log: %v", err))
	}
	m.idx++
	m.revealed = false
	m.done++
}

func (m reviewModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Review  (%d / %d)", m.done, len(m.queue))))
	b.WriteString("\n")

	if m.idx >= len(m.queue) {
		b.WriteString(okStyle.Render("Session complete. Press any key to exit.\n"))
		return b.String()
	}

	card := m.queue[m.idx]
	b.WriteString(wordStyle.Render(card.Word))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("box %d  •  streak %d", card.Box, card.CorrectStreak)))
	b.WriteString("\n\n")

	if !m.revealed {
		b.WriteString(dimStyle.Render("(press space / enter to reveal)"))
	} else {
		b.WriteString(defStyle.Render(card.Definition))
		b.WriteString("\n\n")
		for _, ex := range card.Examples {
			b.WriteString(exampleStyle.Render("• " + ex))
			b.WriteString("\n")
		}
		if m.loading {
			b.WriteString(dimStyle.Render("\nfetching more examples…"))
		}
	}

	if m.flash != "" {
		b.WriteString("\n" + m.flash)
	}

	help := "[space] reveal  •  [j] knew  •  [f] forgot  •  [e] more examples  •  [s] spell  •  [q] quit"
	b.WriteString(helpStyle.Render("\n" + help))
	return b.String()
}
