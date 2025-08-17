package ui

import (
	"strings"
	"sync"
	"time"
	"weak"

	"github.com/charmbracelet/bubbles/progress"

	tea "github.com/charmbracelet/bubbletea"
)

// TODO: Try and go full event driven renders
// TODO: Get rid of final rerender flash

type ProgressTrackers struct {
	model   *Model
	program *tea.Program

	wg  sync.WaitGroup
	err error
}

func NewProgressTrackers() *ProgressTrackers {
	model := &Model{
		progressBars: []*ProgressBar{},
		refreshRate:  time.Millisecond * 50,
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
		model:    progress.New(progress.WithDefaultGradient(), progress.WithSpringOptions(50, 1)),
		index:    uint64(len(p.model.progressBars)),
		program:  weak.Make(p.program),
		state:    Unknown,
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
	print(bars.model.FilteredView(false))
	return bars.err
}

func (bars *ProgressTrackers) Finish() {
	bars.program.Send(Completed{})
}

type Model struct {
	progressBars []*ProgressBar

	refreshRate time.Duration
	completed   bool
	quitting    bool
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tickCmd(m.refreshRate)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}
	switch message := msg.(type) {

	case tea.WindowSizeMsg:
		// m.progress.Width = msg.Width - padding*2 - 4
		// if m.progress.Width > maxWidth {
		// 	m.progress.Width = maxWidth
		// }
	case Completed:
		m.completed = true
		cmds = append(cmds, tickCmd(m.refreshRate))

	case SetBarPercentage:
		if m.completed {
			break
		}
		cmds = append(cmds, tickCmd(m.refreshRate), m.progressBars[message.index].model.SetPercent(message.percentage))

	case SetTrackerText, SetTrackerContent:
		if m.completed {
			break
		}
		switch message := msg.(type) {
		case SetTrackerContent:
			m.progressBars[message.index].content = message.value
		case SetTrackerText:
			m.progressBars[message.index].text = message.value
		}

		cmds = append(cmds, tickCmd(m.refreshRate))
	case tea.ResumeMsg:
		// m.suspending = false
	case tea.KeyMsg:
		switch message.String() {
		case "ctrl+c":
			// TODO: do i need to set quitting true
			cmds = append(cmds, tea.Interrupt)
		case "ctrl+z":
			cmds = append(cmds, tea.Suspend)
		}

	case tickMsg:
		tickCommands := []tea.Cmd{tickCmd(m.refreshRate)}
		animating := false
		for _, progressBar := range m.progressBars {
			animating = progressBar.model.IsAnimating()
			if animating {
				break
			}
		}

		if !animating && m.completed {
			cmds = append(cmds, tea.Quit)
			m.quitting = true
		}

		cmds = append(cmds, tea.Sequence(tickCommands...))

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		commandBatch := make([]tea.Cmd, len(m.progressBars))
		for i, progressBar := range m.progressBars {
			progressModel, cmd := progressBar.model.Update(message)
			progressBar.model = progressModel.(progress.Model)
			commandBatch[i] = cmd
		}
		cmds = append(cmds, tea.Batch(commandBatch...))
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	return m.FilteredView(true)
}

func (m *Model) FilteredView(printNothingOnQuit bool) string {
	if printNothingOnQuit && m.quitting {
		return ""
	}
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
type SetTrackerContent SetTrackerProperty[string]
type SetTrackerText SetTrackerProperty[string]
type SetTrackerProperty[T any] struct {
	index uint64
	value T
}
