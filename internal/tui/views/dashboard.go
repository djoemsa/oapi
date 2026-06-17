package views

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/config"
	"oapi/internal/proxy"
	"oapi/internal/rotation"
	"oapi/internal/tui/styles"
)

type TickMsg time.Time

type SrvStartedMsg struct {
	Srv *proxy.Server
	Err error
}

type SrvStoppedMsg struct{}


type DashboardModel struct {
	parentCtx  context.Context
	cfg        *config.Config
	configPath string
	stateMgr   *config.StateManager
	pool       *rotation.KeyPool
	engine     *rotation.RotationEngine

	srvCtx    context.Context
	srvCancel context.CancelFunc
	srv       *proxy.Server
	running   bool
	width     int
	height    int
	lastTick  time.Time
	err       error
}

func NewDashboardModel(
	parentCtx context.Context,
	cfg *config.Config,
	configPath string,
	stateMgr *config.StateManager,
	pool *rotation.KeyPool,
	engine *rotation.RotationEngine,
) DashboardModel {
	return DashboardModel{
		parentCtx:  parentCtx,
		cfg:        cfg,
		configPath: configPath,
		stateMgr:   stateMgr,
		pool:       pool,
		engine:     engine,
		lastTick:   time.Now(),
	}
}

func (m DashboardModel) Init() tea.Cmd {
	// Start tick loop and auto-start server
	return tea.Batch(
		tickCmd(),
		m.startServerCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *DashboardModel) startServerCmd() tea.Cmd {
	return func() tea.Msg {
		m.srvCtx, m.srvCancel = context.WithCancel(m.parentCtx)
		// Instantiate and start server
		srv := proxy.NewServer(m.cfg, m.configPath, m.stateMgr, m.pool, m.engine)
		err := srv.Start(m.srvCtx)
		return SrvStartedMsg{Srv: srv, Err: err}
	}
}

func (m *DashboardModel) stopServerCmd() tea.Cmd {
	return func() tea.Msg {
		if m.srvCancel != nil {
			m.srvCancel()
		}
		if m.srv != nil {
			<-m.srv.Stopped()
		}
		return SrvStoppedMsg{}
	}
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case TickMsg:
		m.lastTick = time.Time(msg)
		// Request next tick
		return m, tickCmd()

	case SrvStartedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.running = false
		} else {
			m.srv = msg.Srv
			m.running = true
			m.err = nil
		}

	case SrvStoppedMsg:
		m.srv = nil
		m.running = false


	case tea.KeyMsg:
		switch msg.String() {
		case "s":
			if m.running {
				return m, m.stopServerCmd()
			} else {
				return m, m.startServerCmd()
			}
		}
	}

	return m, cmd
}

func (m DashboardModel) View() string {
	w := m.width
	if w < 80 {
		w = 80
	}
	h := m.height
	if h < 24 {
		h = 24
	}

	// Status banner
	var statusBadge string
	var serverAddr string

	if m.running {
		statusBadge = styles.ActiveStyle.Render("● RUNNING")
		serverAddr = fmt.Sprintf("http://%s:%d", m.cfg.Server.Host, m.cfg.Server.Port)
	} else if m.err != nil {
		statusBadge = styles.ErrorStyle.Render(fmt.Sprintf("● ERROR: %v", m.err))
		serverAddr = "Stopped"
	} else {
		statusBadge = styles.ErrorStyle.Render("◉ STOPPED")
		serverAddr = "Stopped"
	}

	statusRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		"Status: ", statusBadge, "  │  Address: ", serverAddr,
	)

	bannerPanel := styles.PanelStyle.
		Width(w - 6).
		Render(statusRow)

	// Summary stats
	state := m.stateMgr.GetState()
	requestsToday := state.TotalRequestsToday

	// Compute average request rate (RPM) from all keys
	totalRPM := 0
	for _, key := range m.cfg.Keys {
		if stats, ok := m.pool.GetKeyStats(key.ID); ok {
			totalRPM += stats.RPMUsed
		}
	}

	statsText := fmt.Sprintf("Requests Today: %d  │  Current Requests/Min: %d", requestsToday, totalRPM)
	statsPanel := styles.PanelStyle.
		Width(w - 6).
		Render(statsText)

	// Key health table header
	headers := []string{"Provider", "Model", "Status", "RPM", "RPD", "Last Used"}
	colWidths := []int{12, 20, 12, 8, 8, 15}

	// Dynamic sizing support
	availableWidth := w - 12
	if availableWidth > 75 {
		// Distribute extra width to Model column
		colWidths[1] = availableWidth - (colWidths[0] + colWidths[2] + colWidths[3] + colWidths[4] + colWidths[5] + 5)
	}

	headerRow := ""
	for i, h := range headers {
		headerRow += fmt.Sprintf("%-*s ", colWidths[i], h)
	}
	headerRow = styles.HeaderStyle.Render(headerRow)

	// Key health rows
	var rows []string
	rows = append(rows, headerRow)
	rows = append(rows, lipgloss.NewStyle().Foreground(styles.ColorBorder).Render(time.Now().Format("15:04:05")+" Current Status:"))

	now := time.Now()
	for _, key := range m.cfg.Keys {
		statusStr := key.Status
		_ = statusStr // Avoid unused variable warning if statusStr isn't used
		var statusStyled string
		var rpmStr, rpdStr, lastUsedStr string

		stats, ok := m.pool.GetKeyStats(key.ID)
		if ok {
			if stats.Cooling {
				statusStyled = styles.CoolingStyle.Render("● Cooling")
			} else if key.Status == "active" {
				statusStyled = styles.ActiveStyle.Render("● Active")
			} else {
				statusStyled = styles.ErrorStyle.Render("◉ Error")
			}

			rpmStr = fmt.Sprintf("%d", stats.RPMUsed)
			rpdStr = fmt.Sprintf("%d", stats.RPDUsed)
			if stats.LastUsed.IsZero() {
				lastUsedStr = "Never"
			} else {
				diff := now.Sub(stats.LastUsed)
				if diff < time.Second {
					lastUsedStr = "just now"
				} else {
					lastUsedStr = fmt.Sprintf("%v ago", diff.Round(time.Second))
				}
			}
		} else {
			statusStyled = styles.MutedStyle.Render("Unknown")
			rpmStr = "-"
			rpdStr = "-"
			lastUsedStr = "-"
		}

		rowText := fmt.Sprintf(
			"%-*s %-*s %-*s %-*s %-*s %-*s",
			colWidths[0], truncateString(key.Provider, colWidths[0]),
			colWidths[1], truncateString(key.Model, colWidths[1]),
			colWidths[2], statusStyled,
			colWidths[3], rpmStr,
			colWidths[4], rpdStr,
			colWidths[5], lastUsedStr,
		)
		rows = append(rows, rowText)
	}

	tableBody := lipgloss.JoinVertical(lipgloss.Left, rows...)
	tablePanel := styles.PanelStyle.
		Width(w - 6).
		Height(h - 15).
		Render(tableBody)

	// Combine sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
		bannerPanel,
		statsPanel,
		tablePanel,
	)
}


func truncateString(s string, l int) string {
	if len(s) > l {
		if l > 3 {
			return s[:l-3] + "..."
		}
		return s[:l]
	}
	return s
}
