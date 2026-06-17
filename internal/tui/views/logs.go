package views

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/tui/styles"
)

type logsViewMode int

const (
	logsModeNormal logsViewMode = iota
	logsModeSearch
)

// LogLineMsg wraps log lines sent from the LogWriter subscription.
type LogLineMsg string

type LogsModel struct {
	Width    int
	Height   int
	logCh    <-chan string
	allLines []string
	filtered []string
	query    string
	paused   bool
	mode     logsViewMode
	viewport viewport.Model
	search   textinput.Model
	status   string
}

// NewLogsModel initializes a LogsModel with the subscribed log channel.
func NewLogsModel(logCh <-chan string) LogsModel {
	vp := viewport.New(0, 0)
	si := textinput.New()
	si.Placeholder = "Search logs..."
	si.CharLimit = 100
	return LogsModel{
		logCh:    logCh,
		viewport: vp,
		search:   si,
		mode:     logsModeNormal,
	}
}

// Init begins the log line subscription command loop.
func (m LogsModel) Init() tea.Cmd {
	return m.waitForLog()
}

// waitForLog waits for a log line to arrive on the channel, returning it as tea.Msg.
func (m LogsModel) waitForLog() tea.Cmd {
	return func() tea.Msg {
		if m.logCh == nil {
			// Prevent tight loops if channel isn't wired or is closed
			time.Sleep(100 * time.Millisecond)
			return LogLineMsg("")
		}
		line, ok := <-m.logCh
		if !ok {
			time.Sleep(100 * time.Millisecond)
			return LogLineMsg("")
		}
		return LogLineMsg(line)
	}
}

// appendLine adds a line to the buffer and caps it at 500 lines.
func (m *LogsModel) appendLine(line string) {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return
	}
	const maxLines = 500
	m.allLines = append(m.allLines, line)
	if len(m.allLines) > maxLines {
		m.allLines = m.allLines[len(m.allLines)-maxLines:]
	}
}

// rebuildContent filters all logs and applies style coloring based on severity tags.
func (m *LogsModel) rebuildContent() {
	m.filtered = nil
	for _, line := range m.allLines {
		// Perform case-insensitive search if query is non-empty
		if m.query == "" || strings.Contains(strings.ToLower(line), strings.ToLower(m.query)) {
			var styledLine string
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "failed") {
				styledLine = lipgloss.NewStyle().Foreground(styles.ColorError).Render(line)
			} else if strings.Contains(lowerLine, "rate limited") || strings.Contains(lowerLine, "429") || strings.Contains(lowerLine, "cooling") {
				styledLine = lipgloss.NewStyle().Foreground(styles.ColorCooling).Render(line)
			} else if strings.Contains(lowerLine, "starting") || strings.Contains(lowerLine, "started") {
				styledLine = lipgloss.NewStyle().Foreground(styles.ColorActive).Render(line)
			} else {
				styledLine = lipgloss.NewStyle().Foreground(styles.ColorText).Render(line)
			}
			m.filtered = append(m.filtered, styledLine)
		}
	}

	m.viewport.SetContent(strings.Join(m.filtered, "\n"))
	if !m.paused {
		m.viewport.GotoBottom()
	}
}

// exportToFile writes the current full logs buffer to a file in the workspace directory.
func (m *LogsModel) exportToFile() (string, error) {
	filename := fmt.Sprintf("oapi-logs-%s.txt", time.Now().Format("20060102-150405"))
	content := strings.Join(m.allLines, "\n")
	err := os.WriteFile(filename, []byte(content), 0644)
	return filename, err
}

// Update processes Bubble Tea messages and updates internal state.
func (m LogsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		
		// Sizing calculations for viewport inside the h - 10 outer panel:
		// Height margin accounts for tab headers, border separators, search inputs and footers.
		vpHeight := m.Height - 16
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Width = m.Width - 8
		m.viewport.Height = vpHeight
		m.rebuildContent()
		return m, nil

	case LogLineMsg:
		if string(msg) != "" {
			m.appendLine(string(msg))
			m.rebuildContent()
		}
		// Continue log listening subscription loop
		return m, m.waitForLog()

	case tea.KeyMsg:
		// Clear status messaging on any interaction
		m.status = ""

		switch m.mode {
		case logsModeSearch:
			switch msg.String() {
			case "esc":
				m.query = ""
				m.mode = logsModeNormal
				m.search.Blur()
				m.rebuildContent()
				return m, nil
			case "enter":
				m.query = m.search.Value()
				m.mode = logsModeNormal
				m.search.Blur()
				m.rebuildContent()
				return m, nil
			default:
				m.search, cmd = m.search.Update(msg)
				return m, cmd
			}

		case logsModeNormal:
			switch msg.String() {
			case "p":
				m.paused = !m.paused
				if !m.paused {
					m.viewport.GotoBottom()
				}
				m.rebuildContent()
				return m, nil
			case "/":
				m.mode = logsModeSearch
				m.search.SetValue(m.query)
				m.search.Focus()
				return m, textinput.Blink
			case "e":
				filename, err := m.exportToFile()
				if err != nil {
					m.status = fmt.Sprintf("Export failed: %v", err)
				} else {
					m.status = fmt.Sprintf("Exported to %s", filename)
				}
				return m, nil
			default:
				// Delegate normal scrolling commands directly to viewport model
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}
	}

	return m, nil
}

// View renders the visual layout of the Logs View.
func (m LogsModel) View() string {
	w := m.Width
	if w < 80 {
		w = 80
	}
	h := m.Height
	if h < 24 {
		h = 24
	}

	// 1. Status Indicator
	var statusBadge string
	if m.paused {
		statusBadge = styles.CoolingStyle.Render("⏸ PAUSED")
	} else {
		statusBadge = styles.ActiveStyle.Render("● LIVE")
	}

	headerText := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.HeaderStyle.Render("Proxy Logs  "),
		statusBadge,
		styles.MutedStyle.Render(fmt.Sprintf("  [%d/500 lines]", len(m.allLines))),
	)

	// 2. Viewport Output
	viewportView := m.viewport.View()

	// 3. Search Field / Status Row
	var searchBar string
	if m.mode == logsModeSearch {
		searchBar = lipgloss.JoinHorizontal(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(styles.ColorAccent).Bold(true).Render("Search: "),
			m.search.View(),
		)
	}

	var statusText string
	if m.status != "" {
		statusText = styles.CoolingStyle.Render(m.status)
	} else if m.query != "" {
		statusText = styles.MutedStyle.Render(fmt.Sprintf("Active Filter: %q (%d matching)", m.query, len(m.filtered)))
	}

	// 4. Keyboard Shortcuts Footer
	var footerText string
	if m.mode == logsModeSearch {
		footerText = "[enter] apply filter  │  [esc] clear & back"
	} else {
		footerText = "[p] pause/resume  │  [/] search  │  [e] export buffer  │  [↑/↓/pgup/pgdn] scroll"
	}
	footer := styles.HelpStyle.Render(footerText)

	// Build the visual stack inside the container
	dividerLine := lipgloss.NewStyle().Foreground(styles.ColorBorder).Render(strings.Repeat("-", w-8))
	sections := []string{
		headerText,
		dividerLine,
		viewportView,
		dividerLine,
	}
	if searchBar != "" {
		sections = append(sections, searchBar)
	}
	if statusText != "" {
		sections = append(sections, statusText)
	}
	sections = append(sections, footer)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return styles.PanelStyle.
		Width(w - 6).
		Height(h - 10).
		Render(content)
}
