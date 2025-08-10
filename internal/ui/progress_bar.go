package ui

import (
	"weak"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProgressBar struct {
	model    progress.Model
	title    string
	subtitle string
	text     string
	content  string

	index   uint64
	program weak.Pointer[tea.Program]
}

var titleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#262626ff", Dark: "#f5f5f5ff"}).
	Bold(true).
	Render

var subtitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#606060ff", Dark: "#979797ff"}).
	Bold(false).
	Render

func (progressBar *ProgressBar) View(pad string) string {
	view := pad + titleStyle(progressBar.title) + " " + subtitleStyle(progressBar.subtitle) + titleStyle(":") + "\n" +
		pad + pad + progressBar.model.View() + "\n"

	if progressBar.content != "" {
		contentStyle := lipgloss.NewStyle().
			PaddingLeft(len(pad) * 2).
			Render
		view += "\n" + contentStyle(progressBar.content)
	}
	return view
}

func (progressBar *ProgressBar) SetContent(content string) error {
	progressBar.program.Value().Send(SetTrackerContent{
		index:   progressBar.index,
		content: content,
	})
	return nil
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
