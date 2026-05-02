package terminal

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justjack1521/mevpatch/internal/gui"
	"github.com/justjack1521/mevpatch/internal/patch"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	stylePrimary   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleSecondary = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleLabel     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(12)
	styleLogNew    = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	styleLogOld    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleError     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	styleDone      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleDivider   = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
)

const maxLogLines = 6

// ── Messages ──────────────────────────────────────────────────────────────────

type msgDone struct{}

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	app     string
	version patch.Version
	updates <-chan gui.PatchUpdate

	primary   string
	secondary string
	errText   string
	finished  bool

	primaryBar   progress.Model
	secondaryBar progress.Model
	spin         spinner.Model

	primaryValue   float64
	secondaryValue float64

	log []string // recent events, newest last
}

func newModel(app string, version patch.Version, updates <-chan gui.PatchUpdate) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	return model{
		app:          app,
		version:      version,
		updates:      updates,
		primaryBar:   progress.New(progress.WithDefaultGradient(), progress.WithWidth(52)),
		secondaryBar: progress.New(progress.WithScaledGradient("#3B82F6", "#10B981"), progress.WithWidth(52)),
		spin:         sp,
		primary:      "Starting...",
	}
}

func (m *model) addLog(line string) {
	m.log = append(m.log, line)
	if len(m.log) > maxLogLines {
		m.log = m.log[len(m.log)-maxLogLines:]
	}
}

func waitForUpdate(ch <-chan gui.PatchUpdate) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-ch
		if !ok {
			return msgDone{}
		}
		return update
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, waitForUpdate(m.updates))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case gui.StatusUpdate:
		if msg.Primary != "" {
			if m.primary != "" && m.primary != "Starting..." {
				m.addLog("✓ " + m.primary)
			}
			m.primary = msg.Primary
		}
		if msg.Secondary != "" {
			m.secondary = msg.Secondary
		}
		return m, waitForUpdate(m.updates)

	case gui.ProgressUpdate:
		if msg.ProgressUpdateType == gui.ProgressUpdateTypePrimary {
			if msg.Reset {
				m.primaryValue = 0
			} else {
				m.primaryValue = clamp(m.primaryValue + float64(msg.Value))
			}
		} else {
			if msg.Reset {
				m.secondaryValue = 0
			} else {
				m.secondaryValue = clamp(m.secondaryValue + float64(msg.Value))
			}
		}
		return m, tea.Batch(
			m.primaryBar.SetPercent(m.primaryValue),
			m.secondaryBar.SetPercent(m.secondaryValue),
			waitForUpdate(m.updates),
		)

	case gui.ErrorUpdate:
		m.errText = formatError(msg.Value)
		m.finished = true
		return m, tea.Batch(waitForUpdate(m.updates), tea.Quit)

	case msgDone:
		m.addLog("✓ " + m.primary)
		m.primary = "Up to date!"
		m.finished = true
		return m, tea.Quit

	case progress.FrameMsg:
		pm, pc := m.primaryBar.Update(msg)
		m.primaryBar = pm.(progress.Model)
		sm, sc := m.secondaryBar.Update(msg)
		m.secondaryBar = sm.(progress.Model)
		return m, tea.Batch(pc, sc)
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString("\n")

	// ── Title ─────────────────────────────────────────────────────────────
	title := fmt.Sprintf("  Mevius Patcher  •  %s", strings.ToUpper(m.app))
	b.WriteString(styleTitle.Render(title))
	b.WriteString("\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", 64)))
	b.WriteString("\n\n")

	// ── Current status ────────────────────────────────────────────────────
	if m.finished && m.errText == "" {
		b.WriteString("  " + styleDone.Render("✓  "+m.primary))
	} else if m.errText != "" {
		b.WriteString("  " + styleError.Render("✗  "+m.primary))
	} else {
		b.WriteString("  " + m.spin.View() + " " + stylePrimary.Render(m.primary))
	}
	b.WriteString("\n")

	if m.secondary != "" {
		b.WriteString("  " + styleLabel.Render("  ") + styleSecondary.Render(m.secondary))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// ── Progress bars ─────────────────────────────────────────────────────
	b.WriteString("  " + styleLabel.Render("Overall") + m.primaryBar.View())
	b.WriteString("\n")
	b.WriteString("  " + styleLabel.Render("Files") + m.secondaryBar.View())
	b.WriteString("\n")

	// ── Log ───────────────────────────────────────────────────────────────
	if len(m.log) > 0 {
		b.WriteString("\n")
		b.WriteString(styleDivider.Render(strings.Repeat("─", 64)))
		b.WriteString("\n")
		for i, line := range m.log {
			if i == len(m.log)-1 {
				b.WriteString("  " + styleLogNew.Render(line))
			} else {
				b.WriteString("  " + styleLogOld.Render(line))
			}
			b.WriteString("\n")
		}
	}

	// ── Error ─────────────────────────────────────────────────────────────
	if m.errText != "" {
		b.WriteString("\n")
		b.WriteString("  " + styleError.Render("Error: "+m.errText))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// ── Window ────────────────────────────────────────────────────────────────────

type Window struct {
	app     string
	version patch.Version
	updates <-chan gui.PatchUpdate
}

func NewWindow(app string, version patch.Version, updates <-chan gui.PatchUpdate) *Window {
	return &Window{app: app, version: version, updates: updates}
}

func (w *Window) Build() {
	p := tea.NewProgram(newModel(w.app, w.version, w.updates), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running UI: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func formatError(err error) string {
	var parts []string
	for err != nil {
		msg := err.Error()
		if i := strings.Index(msg, ": "); i != -1 {
			msg = msg[:i]
		}
		runes := []rune(msg)
		for i, r := range runes {
			if unicode.IsLetter(r) {
				runes[i] = unicode.ToUpper(r)
				break
			}
		}
		parts = append(parts, string(runes))
		err = errors.Unwrap(err)
	}
	return strings.Join(parts, ": ")
}
