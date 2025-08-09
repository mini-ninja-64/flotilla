package ui

import (
	"strings"
	"sync"
	"time"
	"weak"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

type ProgressTrackers struct {
	model   *Model
	program *tea.Program

	wg  sync.WaitGroup
	err error
}

func NewProgressTrackers() *ProgressTrackers {
	model := &Model{
		progressBars: []*ProgressBar{},
		refreshRate:  time.Millisecond * 250,
	}
	program := tea.NewProgram(model)
	return &ProgressTrackers{
		program: program,
		model:   model,
	}
}

func (p *ProgressTrackers) AddProgressBar(title string, subtitle string) *ProgressBar {
	progressBar := &ProgressBar{
		title:    title,
		subtitle: subtitle,
		model:    progress.New(progress.WithDefaultGradient()),
		index:    uint64(len(p.model.progressBars)),
		program:  weak.Make(p.program),
	}

	p.model.progressBars = append(p.model.progressBars, progressBar)
	return progressBar
}

func (bars *ProgressTrackers) RunAsync() {
	bars.wg.Add(1)
	go func() {
		_, err := bars.program.Run()
		bars.err = err
		bars.wg.Done()
	}()
}

func (bars *ProgressTrackers) Wait() error {
	bars.wg.Wait()
	return bars.err
}

func (bars *ProgressTrackers) Finish() {
	bars.program.Send(Completed{})
}

type Model struct {
	progressBars []*ProgressBar

	refreshRate time.Duration
	completed   bool
}

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
		cmds = append(cmds, m.progressBars[msg.index].model.SetPercent(msg.percentage))
		return m, tea.Batch(cmds...)

	case SetTrackerContent:
		if m.completed {
			return m, nil
		}
		m.progressBars[msg.index].content = msg.content
		return m, tickCmd(m.refreshRate)
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
		for _, progressBar := range m.progressBars {
			animating = progressBar.model.IsAnimating()
			if animating {
				break
			}
		}

		if !animating && m.completed {
			cmds = append(cmds, tea.Quit)
			return m, tea.Sequence(cmds...)
		}

		return m, tea.Sequence(cmds...)

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		commandBatch := make([]tea.Cmd, len(m.progressBars))
		for i, progressBar := range m.progressBars {
			progressModel, cmd := progressBar.model.Update(msg)
			progressBar.model = progressModel.(progress.Model)
			commandBatch[i] = cmd
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
	for _, progressBar := range m.progressBars {
		view += progressBar.View(pad) + "\n"
	}
	return view
}

// Message types
type tickMsg time.Time
type Completed struct{}
type SetBarPercentage struct {
	index      uint64
	percentage float64
}
type SetTrackerContent struct {
	index   uint64
	content string
}
