package views

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/tui/styles"
)

type RoutesModel struct {
	Width  int
	Height int
}

func NewRoutesModel() RoutesModel {
	return RoutesModel{}
}

func (m RoutesModel) Init() tea.Cmd {
	return nil
}

func (m RoutesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m RoutesModel) View() string {
	content := lipgloss.NewStyle().
		Foreground(styles.ColorText).
		Render("Routes View Placeholder (T-016)")

	return styles.PanelStyle.
		Width(m.Width - 6).
		Height(m.Height - 10).
		Render(content)
}
