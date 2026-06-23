package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	wordStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")).
			MarginBottom(1)

	defStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC"))

	exampleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88AAFF")).
			MarginLeft(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			MarginTop(1)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B"))

	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6BFF95"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
)
