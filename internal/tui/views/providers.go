package views

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/tui/styles"
)

type ProvidersModel struct {
	Width  int
	Height int
}

func NewProvidersModel() ProvidersModel {
	return ProvidersModel{}
}

func (m ProvidersModel) Init() tea.Cmd {
	return nil
}

func (m ProvidersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m ProvidersModel) View() string {
	content := lipgloss.NewStyle().
		Foreground(styles.ColorText).
		Render("Providers View Placeholder (T-014)")

	return styles.PanelStyle.
		Width(m.Width - 6).
		Height(m.Height - 10).
		Render(content)
}
