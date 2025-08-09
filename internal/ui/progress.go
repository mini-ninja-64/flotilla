package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	trackers []ProgressTracker

	refreshRate time.Duration
	completed   bool
}

type ProgressTracker interface {
	label() string
	View() string
}

type ProgressBar struct {
	// ProgressTracker
	model *progress.Model
	text  string

	index   uint64
	program *tea.Program
}

func (progressBar ProgressBar) SetPercentage(percentage float64) error {
	if percentage > 1 {
		percentage = 1.0
	} else if percentage < 0 {
		percentage = 0
	}
	progressBar.program.Send(SetBarPercentage{
		index:      progressBar.index,
		percentage: percentage,
	})
	return nil
}

func (progressBar ProgressBar) View() string {
	return progressBar.model.View()
}

func (progressBar ProgressBar) label() string {
	return progressBar.text
}

// type Spinner struct {
// 	ProgressTracker
// 	model spinner.Model
// 	text  string
// }

// func (spinner *Spinner) label() string {
// 	return spinner.text
// }

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tickCmd(m.refreshRate)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case Completed:
		m.completed = true
		return m, tickCmd(m.refreshRate)
	case SetBarPercentage:
		if m.completed {
			return m, nil
		}
		cmds := []tea.Cmd{tickCmd(m.refreshRate)}
		progressBar := m.trackers[msg.index].(ProgressBar)
		cmds = append(cmds, progressBar.model.SetPercent(msg.percentage))
		return m, tea.Batch(cmds...)

	case tea.ResumeMsg:
		// m.suspending = false
		return m, nil
	case tea.KeyMsg:
		// TODO: is key.Matches better?
		switch msg.String() {
		case "ctrl+c":
			// m.quitting = true
			return m, tea.Interrupt
		case "ctrl+z":
			return m, tea.Suspend
		}
		return m, nil

	case tea.WindowSizeMsg:
		// m.progress.Width = msg.Width - padding*2 - 4
		// if m.progress.Width > maxWidth {
		// 	m.progress.Width = maxWidth
		// }
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tickCmd(m.refreshRate)}
		animating := false
		// for i := range len(m.progressTrackers) {
		// 	// progressTracker := m.progressTrackers[i]
		// 	// switch progressTracker.(type) {
		// 	// case ProgressBar:
		// 	// 	animating = progressTracker.
		// 	// }
		// 	if animating {
		// 		break
		// 	}
		// }

		if !animating && m.completed {
			cmds = append(cmds, tea.Quit)
			return m, tea.Sequence(cmds...)
		}

		return m, tea.Sequence(cmds...)

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		commandBatch := make([]tea.Cmd, len(m.trackers))
		for i := range m.trackers {
			progressTracker := m.trackers[i]
			switch progressTracker.(type) {
			case *ProgressBar:
				progressBar := progressTracker.(ProgressBar)
				progressModel, cmd := progressBar.model.Update(msg)
				model := progressModel.(progress.Model)
				progressBar.model = &model
				commandBatch[i] = cmd
			}
		}
		return m, tea.Batch(commandBatch...)

	default:
		return m, nil
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	pad := strings.Repeat(" ", 2)
	view := "\n"
	for _, progressBar := range m.trackers {
		view = view + pad + progressBar.View() + "\n\n"
	}
	return view
}

type tickMsg time.Time

type ProgressBars struct {
	model   *Model
	program *tea.Program
}

func (p *ProgressBars) Trackers() []ProgressTracker {
	return p.model.trackers
}

func MultiLineProgressBars(count uint64) *ProgressBars {

	progressBars := make([]ProgressTracker, count)
	for i := uint64(0); i < count; i++ {
		progressBar := progress.New(progress.WithDefaultGradient())
		progressBars[i] = ProgressBar{
			model:   &progressBar,
			index:   i,
			text:    "",
			program: nil,
		}
	}

	model := &Model{
		refreshRate: time.Millisecond * 250,
	}
	program := tea.NewProgram(model)
	progressTrackers := make([]ProgressTracker, count)
	for i := uint64(0); i < count; i++ {
		p := progress.New(progress.WithDefaultGradient())
		progressTrackers[i] = ProgressBar{
			model:   &p,
			index:   i,
			text:    "",
			program: program,
		}
	}

	model.trackers = progressTrackers

	return &ProgressBars{
		program: program,
		model:   model,
	}
}

func (bars *ProgressBars) Run() error {
	if _, err := bars.program.Run(); err != nil {
		return err
	}
	return nil
}

func tickCmd(refreshRate time.Duration) tea.Cmd {
	return tea.Tick(refreshRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (bars *ProgressBars) SetPosition(index uint64, percentage float64) error {

	if percentage > 1 {
		percentage = 1.0
	} else if percentage < 0 {
		percentage = 0
	}
	bars.program.Send(SetBarPercentage{
		index:      index,
		percentage: percentage,
	})
	return nil
}

type Completed struct{}
type SetBarPercentage struct {
	index      uint64
	percentage float64
}

func (bars *ProgressBars) Finish() {
	bars.program.Send(Completed{})
}
