package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/ollama"
	"github.com/andrewnageh/vocab/internal/store"
)

type dashboardModel struct {
	store        *store.Store
	cfg          config.Config
	client       *ollama.Client
	now          time.Time
	msg          string
	due          []store.Card
	recent       []store.Card
	wantsReview  bool
}

func RunDashboard(s *store.Store, cfg config.Config, client *ollama.Client, now time.Time) error {
	m := dashboardModel{
		store:  s,
		cfg:    cfg,
		client: client,
		now:    now,
		due:    s.Due(now),
		recent: s.Recent(5),
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	if dm, ok := final.(dashboardModel); ok && dm.wantsReview {
		due := s.Due(time.Now())
		if len(due) > 0 {
			return RunReview(s, cfg, client, due)
		}
	}
	return nil
}

func (m dashboardModel) Init() tea.Cmd { return nil }

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			if len(m.due) == 0 {
				m.msg = "Nothing due."
				return m, nil
			}
			m.wantsReview = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m dashboardModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("vocab"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Ollama: %s  •  model: %s\n",
		dimStyle.Render(m.cfg.OllamaHost), dimStyle.Render(m.cfg.Model)))
	b.WriteString(fmt.Sprintf("\nDue today: %s    Total cards: %s\n",
		okStyle.Render(fmt.Sprintf("%d", len(m.due))),
		dimStyle.Render(fmt.Sprintf("%d", m.store.CountCards()))))

	if len(m.recent) > 0 {
		b.WriteString("\nRecent additions:\n")
		for _, c := range m.recent {
			flag := ""
			if c.Forgotten(m.store) {
				flag = errStyle.Render(" [forgotten]")
			}
			b.WriteString(fmt.Sprintf("  • %s  %s%s\n",
				c.Word, dimStyle.Render(fmt.Sprintf("box %d", c.Box)), flag))
		}
	}

	if m.msg != "" {
		b.WriteString("\n" + dimStyle.Render(m.msg) + "\n")
	}

	b.WriteString(helpStyle.Render("\n[r] review  •  run `vocab add <word>` / `daily` / `story`  •  [q] quit"))
	return b.String()
}
