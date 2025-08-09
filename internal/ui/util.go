package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func tickCmd(refreshRate time.Duration) tea.Cmd {
	return tea.Tick(refreshRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
