package views

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/tui/styles"
)

type LogsModel struct {
	Width  int
	Height int
}

func NewLogsModel() LogsModel {
	return LogsModel{}
}

func (m LogsModel) Init() tea.Cmd {
	return nil
}

func (m LogsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m LogsModel) View() string {
	content := lipgloss.NewStyle().
		Foreground(styles.ColorText).
		Render("Logs View Placeholder (T-017)")

	return styles.PanelStyle.
		Width(m.Width - 6).
		Height(m.Height - 10).
		Render(content)
}
