package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/betty/agent/lib"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	toolPreviewLines  = 6
	toolPreviewChars  = 900
	inputHeight       = 5
	bannerPaddingY    = 1
	minimumBodyHeight = 8
)

type transcriptEntry struct {
	kind    string
	title   string
	content string
	time    time.Time
	isError bool
}

type agentEventMsg struct {
	event lib.Event
}

type agentDoneMsg struct {
	err error
}

type Model struct {
	session    *lib.Session
	toolNames  []string
	workspace  string
	modelName  string
	logo       string
	transcript []transcriptEntry
	input      textarea.Model
	viewport   viewport.Model
	spinner    spinner.Model
	events     chan tea.Msg
	busy       bool
	width      int
	height     int
	lastError  string
}

func NewModel(session *lib.Session, workspaceRoot, modelName string, toolNames []string, logo string) Model {
	input := textarea.New()
	input.Placeholder = "Ask the agent to inspect, edit, and verify your project..."
	input.Focus()
	input.SetHeight(inputHeight)
	input.ShowLineNumbers = false
	input.Prompt = "> "
	input.CharLimit = 0
	input.KeyMap.InsertNewline.SetEnabled(true)

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	model := Model{
		session:   session,
		toolNames: append([]string(nil), toolNames...),
		workspace: workspaceRoot,
		modelName: modelName,
		logo:      strings.TrimRight(logo, "\n"),
		input:     input,
		spinner:   spin,
		events:    make(chan tea.Msg, 256),
		transcript: []transcriptEntry{{
			kind:    "system",
			title:   "Ready",
			content: "I can inspect the project, edit files, and run local commands. Use `/help` for shortcuts.",
			time:    time.Now(),
		}},
	}

	model.viewport = viewport.New(0, 0)
	model.refreshViewport()

	return model
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.refreshViewport()
		return m, nil
	case tea.KeyMsg:
		if isViewportKey(msg.String()) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			if m.busy {
				return m, nil
			}

			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}

			if handled := m.handleLocalCommand(prompt); handled {
				m.input.Reset()
				m.refreshViewport()
				return m, nil
			}

			m.busy = true
			m.lastError = ""
			m.addEntry("user", "You", prompt, false)
			m.input.Reset()
			m.refreshViewport()
			return m, tea.Batch(m.spinner.Tick, startAgentTurn(m.session, prompt, m.events))
		case "ctrl+l":
			if m.busy {
				return m, nil
			}
			m.clearTranscript()
			m.refreshViewport()
			return m, nil
		case "ctrl+r":
			if m.busy {
				return m, nil
			}
			m.session.Reset()
			m.clearTranscript()
			m.addEntry("system", "Reset", "Conversation history cleared.", false)
			m.refreshViewport()
			return m, nil
		}
	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case agentEventMsg:
		m.applyAgentEvent(msg.event)
		m.refreshViewport()
		return m, waitForAgentEvent(m.events)
	case agentDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.lastError = msg.err.Error()
			m.addEntry("status", "Error", msg.err.Error(), true)
		} else {
			m.addEntry("status", "Done", "Turn complete.", false)
		}
		m.refreshViewport()
		return m, nil
	}

	if !m.busy {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refreshViewport()
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	banner := bannerStyle.Width(max(20, m.width-2)).Render(m.renderBanner())
	body := transcriptStyle.Width(max(20, m.width-2)).Render(m.viewport.View())
	input := inputShellStyle.Width(max(20, m.width-2)).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			inputLabelStyle.Render("Prompt"),
			m.input.View(),
		),
	)

	footerText := fmt.Sprintf("%s %s   model: %s   scroll: mouse/pgup/pgdn/home/end   send: ctrl+s   clear: ctrl+l   reset: ctrl+r   quit: ctrl+c", m.statusIndicator(), m.statusLabel(), m.modelName)
	if m.lastError != "" {
		footerText += "   error: " + m.lastError
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		banner,
		body,
		input,
		footerStyle.Width(m.width).Render(footerText),
	)
}

func (m *Model) resize() {
	innerWidth := max(20, m.width-4)
	bannerHeight := lipgloss.Height(bannerStyle.Width(innerWidth).Render(m.renderBanner()))
	inputBlockHeight := inputHeight + 4
	bodyHeight := m.height - bannerHeight - inputBlockHeight - 2
	if bodyHeight < minimumBodyHeight {
		bodyHeight = minimumBodyHeight
	}

	m.viewport.Width = innerWidth
	m.viewport.Height = bodyHeight
	m.input.SetWidth(innerWidth - 2)
	m.input.SetHeight(inputHeight)
}

func (m *Model) refreshViewport() {
	m.viewport.SetContent(m.renderTranscript())
	m.viewport.GotoBottom()
}

func (m *Model) renderBanner() string {
	title := bannerTitleStyle.Render("HUST coding agent")
	subtitle := bannerSubtitleStyle.Render("Inspect projects, edit files, and verify changes with local commands.")
	hints := bannerHintStyle.Render("Shortcuts: /help  /tools  /clear  /reset")
	credit := bannerCreditStyle.Render("from Navis Lab, SoICT with ❤️")
	workspace := bannerMetaStyle.Render("workspace: " + m.workspace)
	info := lipgloss.JoinVertical(lipgloss.Left, title, subtitle, hints, credit, workspace)

	if m.logo == "" {
		return info
	}

	logo := bannerLogoColumnStyle.Render(bannerLogoStyle.Render(m.logo))
	availableWidth := max(20, m.width-10)
	stackedThreshold := lipgloss.Width(logo) + 32
	if availableWidth <= stackedThreshold {
		return lipgloss.JoinVertical(lipgloss.Left, logo, info)
	}

	infoWidth := max(20, availableWidth-lipgloss.Width(logo)-2)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		logo,
		bannerInfoStyle.Width(infoWidth).Render(info),
	)
}

func (m *Model) renderTranscript() string {
	blocks := make([]string, 0, len(m.transcript))
	for _, entry := range m.transcript {
		blocks = append(blocks, m.renderEntry(entry))
	}
	return strings.Join(blocks, "\n\n")
}

func (m *Model) renderEntry(entry transcriptEntry) string {
	labelStyle := systemLabelStyle
	bodyStyle := systemBodyStyle
	wrapperStyle := baseEntryStyle
	label := entry.title

	switch entry.kind {
	case "user":
		labelStyle = userLabelStyle
		bodyStyle = userBodyStyle
		wrapperStyle = userEntryStyle
	case "assistant":
		labelStyle = assistantLabelStyle
		bodyStyle = assistantBodyStyle
		wrapperStyle = assistantEntryStyle
	case "tool":
		labelStyle = toolLabelStyle
		bodyStyle = toolBodyStyle
		wrapperStyle = toolEntryStyle
	case "status":
		if entry.isError {
			labelStyle = errorLabelStyle
			bodyStyle = errorBodyStyle
			wrapperStyle = errorEntryStyle
		} else {
			labelStyle = statusLabelStyle
			bodyStyle = statusBodyStyle
			wrapperStyle = statusEntryStyle
		}
	}

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render(label),
		" ",
		timestampStyle.Render(entry.time.Format("15:04:05")),
	)

	body := indentBlock(bodyStyle.Width(max(10, m.viewport.Width-2)).Render(entry.content), "  ")
	return wrapperStyle.Width(max(12, m.viewport.Width)).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, body),
	)
}

func (m *Model) addEntry(kind, title, content string, isError bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	m.transcript = append(m.transcript, transcriptEntry{
		kind:    kind,
		title:   title,
		content: content,
		time:    time.Now(),
		isError: isError,
	})
}

func (m *Model) clearTranscript() {
	m.transcript = []transcriptEntry{{
		kind:    "system",
		title:   "Ready",
		content: "Start a new prompt when you're ready.",
		time:    time.Now(),
	}}
	m.lastError = ""
}

func (m *Model) applyAgentEvent(event lib.Event) {
	switch event.Type {
	case lib.EventAssistant:
		m.addEntry("assistant", "Agent", event.Message, false)
	case lib.EventStatus:
		message := strings.TrimSpace(event.Message)
		if message != "Idle" {
			m.addEntry("status", "Status", message, false)
		}
	case lib.EventToolCall:
		content := formatToolCall(event.ToolName, event.ToolInput)
		m.addEntry("tool", "Tool", content, false)
	case lib.EventToolResult:
		content := formatToolResult(event.ToolName, event.Message)
		m.addEntry("tool", "Tool", content, event.IsError)
	}
}

func (m *Model) handleLocalCommand(prompt string) bool {
	switch strings.TrimSpace(prompt) {
	case "/help":
		m.addEntry("system", "Help", "Local commands: `/help`, `/tools`, `/clear`, `/reset`. Send prompts with `ctrl+s`.", false)
		return true
	case "/tools":
		m.addEntry("system", "Tools", strings.Join(m.toolNames, ", "), false)
		return true
	case "/clear":
		m.clearTranscript()
		return true
	case "/reset":
		m.session.Reset()
		m.clearTranscript()
		m.addEntry("system", "Reset", "Conversation history cleared.", false)
		return true
	default:
		return false
	}
}

func (m Model) statusLabel() string {
	if m.busy {
		return "working"
	}
	return "ready"
}

func (m Model) statusIndicator() string {
	if m.busy {
		return m.spinner.View()
	}
	return "*"
}

func formatToolCall(name, input string) string {
	if strings.TrimSpace(input) == "" {
		return name
	}
	return name + "\n" + previewText(input, 4, 360)
}

func formatToolResult(name, result string) string {
	preview := previewText(result, toolPreviewLines, toolPreviewChars)
	if preview == "" {
		return name + " finished"
	}
	return name + "\n" + preview
}

func previewText(content string, maxLines, maxChars int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	originalLineCount := len(strings.Split(content, "\n"))
	truncated := false
	if maxChars > 0 && len(content) > maxChars {
		content = content[:maxChars]
		truncated = true
	}

	lines := strings.Split(content, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	preview := strings.Join(lines, "\n")
	if truncated {
		remaining := originalLineCount - len(lines)
		if remaining > 0 {
			preview += fmt.Sprintf("\n... %d more line(s)", remaining)
		} else {
			preview += "\n... more"
		}
	}

	return preview
}

func indentBlock(content, prefix string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func startAgentTurn(session *lib.Session, prompt string, events chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			err := session.RunTurn(context.Background(), prompt, func(event lib.Event) {
				events <- agentEventMsg{event: event}
			})
			events <- agentDoneMsg{err: err}
		}()
		return <-events
	}
}

func waitForAgentEvent(events chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-events
	}
}

func isViewportKey(key string) bool {
	switch key {
	case "pgup", "pgdown", "home", "end", "alt+up", "alt+down":
		return true
	default:
		return false
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	bannerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("31")).
			Background(lipgloss.Color("235")).
			Padding(bannerPaddingY, 2).
			MarginBottom(1)
	bannerLogoColumnStyle = lipgloss.NewStyle().
				BorderRight(true).
				BorderForeground(lipgloss.Color("59")).
				PaddingRight(2).
				MarginRight(2)
	bannerLogoStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	bannerInfoStyle     = lipgloss.NewStyle().Align(lipgloss.Left)
	bannerTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).PaddingBottom(1)
	bannerSubtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).PaddingBottom(1)
	bannerHintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("151")).PaddingBottom(1)
	bannerCreditStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("217")).Bold(true).PaddingBottom(1)
	bannerMetaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	transcriptStyle     = lipgloss.NewStyle().Padding(0, 1).MarginBottom(1)
	baseEntryStyle      = lipgloss.NewStyle().MarginBottom(1)
	assistantEntryStyle = lipgloss.NewStyle().
				MarginBottom(1).
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.Color("214")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)
	userEntryStyle = lipgloss.NewStyle().
			MarginBottom(1).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("45")).
			Padding(0, 1)
	toolEntryStyle = lipgloss.NewStyle().
			MarginBottom(1).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	statusEntryStyle = lipgloss.NewStyle().
				MarginBottom(1).
				Foreground(lipgloss.Color("150")).
				Padding(0, 1)
	errorEntryStyle = lipgloss.NewStyle().
			MarginBottom(1).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("203")).
			Background(lipgloss.Color("52")).
			Padding(0, 1)
	inputShellStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("239")).
			Padding(0, 1).
			MarginBottom(1)
	inputLabelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	footerStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("238")).Padding(0, 1)
	userLabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45"))
	assistantLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	toolLabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	statusLabelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("120"))
	errorLabelStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	systemLabelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	userBodyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	assistantBodyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	toolBodyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("249"))
	statusBodyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("150"))
	errorBodyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	systemBodyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	timestampStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)
