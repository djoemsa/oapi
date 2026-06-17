package tui

import (
	"context"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/config"
	"oapi/internal/rotation"
	"oapi/internal/tui/styles"
	"oapi/internal/tui/views"
)

type TUIConfig struct {
	Cfg        *config.Config
	ConfigPath string
	StateMgr   *config.StateManager
	Pool       *rotation.KeyPool
	Engine     *rotation.RotationEngine
	Ctx        context.Context
	Cancel     context.CancelFunc
	LogCh      <-chan string
}

type AppModel struct {
	activeTab int
	width     int
	height    int
	dashboard views.DashboardModel
	providers views.ProvidersModel
	routes    views.RoutesModel
	logs      views.LogsModel
	tuiCfg    TUIConfig
}

func New(tuiCfg TUIConfig) AppModel {
	return AppModel{
		tuiCfg:    tuiCfg,
		dashboard: views.NewDashboardModel(tuiCfg.Ctx, tuiCfg.Cfg, tuiCfg.ConfigPath, tuiCfg.StateMgr, tuiCfg.Pool, tuiCfg.Engine),
		providers: views.NewProvidersModel(tuiCfg.Ctx, tuiCfg.Cfg, tuiCfg.ConfigPath, tuiCfg.StateMgr, tuiCfg.Pool),
		routes:    views.NewRoutesModel(tuiCfg.Cfg, tuiCfg.ConfigPath),
		logs:      views.NewLogsModel(tuiCfg.LogCh),
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.dashboard.Init(),
		m.logs.Init(),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global key events first
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// ctrl+c always exits the app
		if keyMsg.String() == "ctrl+c" {
			if m.tuiCfg.Cancel != nil {
				m.tuiCfg.Cancel()
			}
			m.routes.FlushIfDirty()
			return m, tea.Quit
		}

		// Other global commands should only execute when we are not in an input form
		isEditing := false
		if m.activeTab == 1 && m.providers.IsEditing() {
			isEditing = true
		} else if m.activeTab == 2 && m.routes.IsEditing() {
			isEditing = true
		}

		if !isEditing {
			switch keyMsg.String() {
			case "q":
				if m.tuiCfg.Cancel != nil {
					m.tuiCfg.Cancel()
				}
				m.routes.FlushIfDirty()
				return m, tea.Quit
			case "tab", "]":
				m.activeTab = (m.activeTab + 1) % 4
				return m, nil
			case "shift+tab", "[":
				m.activeTab = (m.activeTab - 1 + 4) % 4
				return m, nil
			}
		}
	}

	// Propagate window size to all models
	if wMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wMsg.Width
		m.height = wMsg.Height

		// Propagate sizing to sub-models
		var cmd tea.Cmd
		var newDashboard tea.Model
		newDashboard, cmd = m.dashboard.Update(wMsg)
		m.dashboard = newDashboard.(views.DashboardModel)
		cmds = append(cmds, cmd)

		var newProviders tea.Model
		newProviders, cmd = m.providers.Update(wMsg)
		m.providers = newProviders.(views.ProvidersModel)
		cmds = append(cmds, cmd)

		var newRoutes tea.Model
		newRoutes, cmd = m.routes.Update(wMsg)
		m.routes = newRoutes.(views.RoutesModel)
		cmds = append(cmds, cmd)

		var newLogs tea.Model
		newLogs, cmd = m.logs.Update(wMsg)
		m.logs = newLogs.(views.LogsModel)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)
	}

	// Propagate specific server/tick events to dashboard always
	switch msg.(type) {
	case views.TickMsg, views.SrvStartedMsg, views.SrvStoppedMsg:
		var cmd tea.Cmd
		var newDashboard tea.Model
		newDashboard, cmd = m.dashboard.Update(msg)
		m.dashboard = newDashboard.(views.DashboardModel)
		return m, cmd
	}

	// Otherwise, delegate message to active tab
	var cmd tea.Cmd
	switch m.activeTab {
	case 0:
		var newDashboard tea.Model
		newDashboard, cmd = m.dashboard.Update(msg)
		m.dashboard = newDashboard.(views.DashboardModel)
	case 1:
		var newProviders tea.Model
		newProviders, cmd = m.providers.Update(msg)
		m.providers = newProviders.(views.ProvidersModel)
	case 2:
		var newRoutes tea.Model
		newRoutes, cmd = m.routes.Update(msg)
		m.routes = newRoutes.(views.RoutesModel)
	case 3:
		var newLogs tea.Model
		newLogs, cmd = m.logs.Update(msg)
		m.logs = newLogs.(views.LogsModel)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	// Render tabs
	var tabViews []string
	for i, tabName := range []string{"Dashboard", "Providers", "Routes", "Logs"} {
		if i == m.activeTab {
			tabViews = append(tabViews, styles.ActiveTabStyle.Render(tabName))
		} else {
			tabViews = append(tabViews, styles.InactiveTabStyle.Render(tabName))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	tabBar = styles.TabBarStyle.Width(w - 2).Render(tabBar)

	// Render active content
	var activeViewContent string
	switch m.activeTab {
	case 0:
		activeViewContent = m.dashboard.View()
	case 1:
		activeViewContent = m.providers.View()
	case 2:
		activeViewContent = m.routes.View()
	case 3:
		activeViewContent = m.logs.View()
	}

	// Footer help
	footer := styles.HelpStyle.Render("[tab]/[]] next tab  │  [shift+tab]/[[] prev tab  │  [s] toggle server  │  [q] quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		activeViewContent,
		footer,
	)
}

func Run(cfg TUIConfig) error {
	m := New(cfg)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
