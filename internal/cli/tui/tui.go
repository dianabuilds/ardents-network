package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type tuiSection int

const (
	tuiNode tuiSection = iota
	tuiNetwork
	tuiWorkloads
	tuiData
	tuiDiagnostics
)

var tuiSections = []tuiSection{tuiNode, tuiNetwork, tuiWorkloads, tuiData, tuiDiagnostics}

type tuiSnapshot struct {
	Title     string
	Lines     []string
	UpdatedAt time.Time
}

type tuiSnapshotMsg struct {
	Section  tuiSection
	Snapshot tuiSnapshot
	Err      error
}

type tuiActionMsg struct {
	Message string
	Err     error
}

type tuiTickMsg time.Time

type tuiModel struct {
	ctx      context.Context
	app      *Command
	active   tuiSection
	width    int
	height   int
	loading  bool
	err      string
	action   string
	cache    map[tuiSection]tuiSnapshot
	interval time.Duration
}

func newTUIModel(ctx context.Context, app *Command) tuiModel {
	interval := app.ctx.Interval
	if interval <= 0 {
		interval = time.Second
	}
	return tuiModel{
		ctx:      ctx,
		app:      app,
		active:   tuiNode,
		cache:    make(map[tuiSection]tuiSnapshot),
		interval: interval,
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(m.loadSectionCmd(m.active), m.tickCmd())
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tuiSnapshotMsg:
		return m.handleSnapshot(msg)
	case tuiActionMsg:
		return m.handleAction(msg)
	case tuiTickMsg:
		return m.handleTick()
	}
	return m, nil
}

func (m tuiModel) handleSnapshot(msg tuiSnapshotMsg) (tea.Model, tea.Cmd) {
	if msg.Section != m.active {
		m.cache[msg.Section] = msg.Snapshot
		return m, nil
	}
	m.loading = false
	if msg.Err != nil {
		m.err = msg.Err.Error()
		return m, nil
	}
	m.err = ""
	m.cache[msg.Section] = msg.Snapshot
	return m, nil
}

func (m tuiModel) handleAction(msg tuiActionMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.action = ""
		m.err = msg.Err.Error()
		m.loading = false
		return m, nil
	}
	m.action = msg.Message
	m.err = ""
	m.loading = true
	return m, m.loadSectionCmd(m.active)
}

func (m tuiModel) handleTick() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.tickCmd()}
	if !m.loading {
		m.loading = true
		cmds = append(cmds, m.loadSectionCmd(m.active))
	}
	return m, tea.Batch(cmds...)
}

func (m tuiModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab", "right", "l":
		return m.navigateTUI(nextTUISection(m.active))
	case "shift+tab", "left", "h":
		return m.navigateTUI(prevTUISection(m.active))
	case "r":
		return m.navigateTUI(m.active)
	case "s", "x":
		return m.triggerAction(msg.String())
	default:
		return m, nil
	}
}

func (m tuiModel) navigateTUI(section tuiSection) (tea.Model, tea.Cmd) {
	m.active = section
	m.loading = true
	m.err = ""
	m.action = ""
	return m, m.loadSectionCmd(m.active)
}

func (m tuiModel) triggerAction(key string) (tea.Model, tea.Cmd) {
	action, ok := tuiActionForKey(m.active, key)
	if !ok {
		return m, nil
	}
	m.loading = true
	m.err = ""
	m.action = ""
	return m, m.runActionCmd(action)
}

func (m tuiModel) View() tea.View {
	var b strings.Builder
	b.WriteString(m.renderChrome())
	snapshot, ok := m.cache[m.active]
	if !ok {
		b.WriteString("no snapshot yet")
		return m.tuiView(b.String())
	}
	b.WriteString(snapshot.Title)
	b.WriteString("\n")
	if !snapshot.UpdatedAt.IsZero() {
		b.WriteString("updated: ")
		b.WriteString(formatTime(snapshot.UpdatedAt))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	for _, line := range snapshot.Lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return m.tuiView(b.String())
}

func (m tuiModel) renderChrome() string {
	var b strings.Builder
	b.WriteString("Ardents Operator Terminal\n")
	b.WriteString(m.renderTabs())
	b.WriteString("\n")
	b.WriteString("Keys: left/right or tab/shift+tab switch sections, r refresh, q quit\n")
	if hints := tuiActionHint(m.active); hints != "" {
		b.WriteString("Actions: ")
		b.WriteString(hints)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.loading {
		b.WriteString("loading current section...\n\n")
	}
	if m.err != "" {
		b.WriteString("last error: ")
		b.WriteString(m.err)
		b.WriteString("\n\n")
	}
	if m.action != "" {
		b.WriteString("last action: ")
		b.WriteString(m.action)
		b.WriteString("\n\n")
	}
	return b.String()
}

func (m tuiModel) tuiView(content string) tea.View {
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m tuiModel) renderTabs() string {
	parts := make([]string, 0, len(tuiSections))
	for _, section := range tuiSections {
		label := tuiSectionTitle(section)
		if section == m.active {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "  ")
}

func (m tuiModel) tickCmd() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg {
		return tuiTickMsg(t)
	})
}

func (m tuiModel) loadSectionCmd(section tuiSection) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := m.app.fetchTUISnapshot(m.ctx, section)
		return tuiSnapshotMsg{Section: section, Snapshot: snapshot, Err: err}
	}
}

func (m tuiModel) runActionCmd(action tuiAction) tea.Cmd {
	return func() tea.Msg {
		message, err := m.app.executeTUIAction(m.ctx, action)
		return tuiActionMsg{Message: message, Err: err}
	}
}

func nextTUISection(section tuiSection) tuiSection {
	for i, item := range tuiSections {
		if item == section {
			return tuiSections[(i+1)%len(tuiSections)]
		}
	}
	return tuiSections[0]
}

func prevTUISection(section tuiSection) tuiSection {
	for i, item := range tuiSections {
		if item == section {
			if i == 0 {
				return tuiSections[len(tuiSections)-1]
			}
			return tuiSections[i-1]
		}
	}
	return tuiSections[0]
}

func tuiSectionTitle(section tuiSection) string {
	switch section {
	case tuiNode:
		return "Node"
	case tuiNetwork:
		return "Network"
	case tuiWorkloads:
		return "Workloads"
	case tuiData:
		return "Data"
	case tuiDiagnostics:
		return "Diagnostics"
	default:
		return "Unknown"
	}
}

func tuiActionHint(section tuiSection) string {
	switch section {
	case tuiNode:
		return "s start node, x stop node"
	case tuiWorkloads:
		return ""
	default:
		return ""
	}
}
