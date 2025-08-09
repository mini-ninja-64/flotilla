package ui

import (
	"weak"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProgressBar struct {
	model progress.Model
	title string
	text  string

	index   uint64
	program weak.Pointer[tea.Program]
}

var titleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#262626ff", Dark: "#f5f5f5ff"}).
	Bold(true).
	Render

func (progressBar *ProgressBar) View(pad string) string {
	view := pad + titleStyle(progressBar.title) + ":\n" +
		pad + pad + progressBar.model.View() + "\n"
	return view
}

func (progressBar *ProgressBar) SetPercentage(percentage float64) error {
	if percentage > 1 {
		percentage = 1.0
	} else if percentage < 0 {
		percentage = 0
	}
	progressBar.program.Value().Send(SetBarPercentage{
		index:      progressBar.index,
		percentage: percentage,
	})
	return nil
}
