package views

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/config"
	"oapi/internal/registry"
	"oapi/internal/tui/styles"
)

type routesViewMode int

const (
	routesModeRouteList routesViewMode = iota
	routesModeSlotList
	routesModeAddRoute
	routesModeAddSlot
	routesModeDeleteRoute
	routesModeDeleteSlot
)

type RoutesModel struct {
	cfg        *config.Config
	configPath string
	Width      int
	Height     int

	mode        routesViewMode
	routeIdx    int // selected route (left panel)
	slotSection int // 0 = chain, 1 = fallback
	slotIdx     int // index within current section

	isDirty   bool // pending reorder changes not yet saved
	statusMsg string

	addRouteForm *huh.Form
	addSlotForm  *huh.Form

	formRouteName    string
	formRouteAlias   string
	formSlotProvider string
	formSlotModel    string
}

func NewRoutesModel(cfg *config.Config, configPath string) RoutesModel {
	return RoutesModel{
		cfg:        cfg,
		configPath: configPath,
		mode:       routesModeRouteList,
	}
}

func (m RoutesModel) Init() tea.Cmd {
	return nil
}

func (m RoutesModel) IsEditing() bool {
	return m.mode == routesModeAddRoute || m.mode == routesModeAddSlot
}

func (m *RoutesModel) saveNow() {
	_ = config.SaveConfig(m.configPath, m.cfg)
	m.isDirty = false
}

func (m *RoutesModel) FlushIfDirty() {
	if m.isDirty {
		_ = config.SaveConfig(m.configPath, m.cfg)
		m.isDirty = false
	}
}

func (m *RoutesModel) initAddRouteForm() {
	m.formRouteName = ""
	m.formRouteAlias = ""

	m.addRouteForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Route Name").
				Placeholder("e.g. Production GPT-4 Route").
				Value(&m.formRouteName).
				Validate(func(str string) error {
					if len(str) == 0 {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Model Alias").
				Placeholder("e.g. gpt-4").
				Value(&m.formRouteAlias).
				Validate(func(str string) error {
					if len(str) == 0 {
						return fmt.Errorf("model alias is required")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCharm())

	m.addRouteForm.Init()
}

func (m *RoutesModel) saveRoute() {
	newRoute := config.RouteConfig{
		Name:       m.formRouteName,
		ModelAlias: m.formRouteAlias,
		Chain:      []config.SlotConfig{},
		Fallback:   []config.SlotConfig{},
	}
	m.cfg.Routes = append(m.cfg.Routes, newRoute)
	m.routeIdx = len(m.cfg.Routes) - 1
	m.saveNow()
	m.statusMsg = "Route added successfully"
}

func (m *RoutesModel) deleteRoute(idx int) {
	if idx < 0 || idx >= len(m.cfg.Routes) {
		return
	}
	m.cfg.Routes = append(m.cfg.Routes[:idx], m.cfg.Routes[idx+1:]...)
	m.routeIdx = max(0, len(m.cfg.Routes)-1)
	m.saveNow()
	m.statusMsg = "Route deleted successfully"
}

func (m *RoutesModel) initAddSlotForm() {
	providerIDs := make([]string, 0, len(registry.Providers))
	for id := range registry.Providers {
		providerIDs = append(providerIDs, id)
	}
	sort.Strings(providerIDs)

	if m.formSlotProvider == "" {
		m.formSlotProvider = providerIDs[0]
	}

	var opts []huh.Option[string]
	for _, id := range providerIDs {
		opts = append(opts, huh.NewOption(id, id))
	}

	placeholder := "e.g. llama-3.3-70b-versatile"
	if recs, ok := registry.RecommendedModels[m.formSlotProvider]; ok && len(recs) > 0 {
		placeholder = fmt.Sprintf("e.g. %s", recs[0])
	}

	m.formSlotModel = ""

	m.addSlotForm = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Provider").
				Options(opts...).
				Value(&m.formSlotProvider),
			huh.NewInput().
				Title("Model").
				Placeholder(placeholder).
				Value(&m.formSlotModel).
				Validate(func(str string) error {
					if len(str) == 0 {
						return fmt.Errorf("model is required")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCharm())

	m.addSlotForm.Init()
}

func (m *RoutesModel) saveSlot() {
	if len(m.cfg.Routes) == 0 || m.routeIdx < 0 || m.routeIdx >= len(m.cfg.Routes) {
		return
	}

	newSlot := config.SlotConfig{
		Provider: m.formSlotProvider,
		Model:    m.formSlotModel,
	}

	if m.slotSection == 0 {
		m.cfg.Routes[m.routeIdx].Chain = append(m.cfg.Routes[m.routeIdx].Chain, newSlot)
		m.slotIdx = len(m.cfg.Routes[m.routeIdx].Chain) - 1
	} else {
		m.cfg.Routes[m.routeIdx].Fallback = append(m.cfg.Routes[m.routeIdx].Fallback, newSlot)
		m.slotIdx = len(m.cfg.Routes[m.routeIdx].Fallback) - 1
	}

	m.saveNow()
	m.statusMsg = "Slot added successfully"
}

func (m *RoutesModel) deleteSlot() {
	if len(m.cfg.Routes) == 0 || m.routeIdx < 0 || m.routeIdx >= len(m.cfg.Routes) {
		return
	}

	if m.slotSection == 0 {
		chain := m.cfg.Routes[m.routeIdx].Chain
		if m.slotIdx < 0 || m.slotIdx >= len(chain) {
			return
		}
		m.cfg.Routes[m.routeIdx].Chain = append(chain[:m.slotIdx], chain[m.slotIdx+1:]...)
		m.slotIdx = max(0, len(m.cfg.Routes[m.routeIdx].Chain)-1)
	} else {
		fallback := m.cfg.Routes[m.routeIdx].Fallback
		if m.slotIdx < 0 || m.slotIdx >= len(fallback) {
			return
		}
		m.cfg.Routes[m.routeIdx].Fallback = append(fallback[:m.slotIdx], fallback[m.slotIdx+1:]...)
		m.slotIdx = max(0, len(m.cfg.Routes[m.routeIdx].Fallback)-1)
	}

	m.saveNow()
	m.statusMsg = "Slot deleted successfully"
}

func (m *RoutesModel) reorderSlotUp() {
	if len(m.cfg.Routes) == 0 || m.routeIdx < 0 || m.routeIdx >= len(m.cfg.Routes) {
		return
	}

	if m.slotSection == 0 {
		chain := m.cfg.Routes[m.routeIdx].Chain
		if m.slotIdx <= 0 || m.slotIdx >= len(chain) {
			return
		}
		chain[m.slotIdx], chain[m.slotIdx-1] = chain[m.slotIdx-1], chain[m.slotIdx]
		m.slotIdx--
	} else {
		fallback := m.cfg.Routes[m.routeIdx].Fallback
		if m.slotIdx <= 0 || m.slotIdx >= len(fallback) {
			return
		}
		fallback[m.slotIdx], fallback[m.slotIdx-1] = fallback[m.slotIdx-1], fallback[m.slotIdx]
		m.slotIdx--
	}
	m.isDirty = true
}

func (m *RoutesModel) reorderSlotDown() {
	if len(m.cfg.Routes) == 0 || m.routeIdx < 0 || m.routeIdx >= len(m.cfg.Routes) {
		return
	}

	if m.slotSection == 0 {
		chain := m.cfg.Routes[m.routeIdx].Chain
		if m.slotIdx < 0 || m.slotIdx >= len(chain)-1 {
			return
		}
		chain[m.slotIdx], chain[m.slotIdx+1] = chain[m.slotIdx+1], chain[m.slotIdx]
		m.slotIdx++
	} else {
		fallback := m.cfg.Routes[m.routeIdx].Fallback
		if m.slotIdx < 0 || m.slotIdx >= len(fallback)-1 {
			return
		}
		fallback[m.slotIdx], fallback[m.slotIdx+1] = fallback[m.slotIdx+1], fallback[m.slotIdx]
		m.slotIdx++
	}
	m.isDirty = true
}

func (m RoutesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}

	switch m.mode {
	case routesModeAddRoute:
		var newForm tea.Model
		newForm, cmd = m.addRouteForm.Update(msg)
		m.addRouteForm = newForm.(*huh.Form)
		cmds = append(cmds, cmd)

		if m.addRouteForm.State == huh.StateCompleted {
			m.saveRoute()
			m.mode = routesModeRouteList
		} else if m.addRouteForm.State == huh.StateAborted {
			m.mode = routesModeRouteList
		}
		return m, tea.Batch(cmds...)

	case routesModeAddSlot:
		var newForm tea.Model
		newForm, cmd = m.addSlotForm.Update(msg)
		m.addSlotForm = newForm.(*huh.Form)
		cmds = append(cmds, cmd)

		if m.addSlotForm.State == huh.StateCompleted {
			m.saveSlot()
			m.mode = routesModeSlotList
		} else if m.addSlotForm.State == huh.StateAborted {
			m.mode = routesModeSlotList
		}
		return m, tea.Batch(cmds...)

	case routesModeDeleteRoute:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				m.deleteRoute(m.routeIdx)
				m.mode = routesModeRouteList
			case "n", "N", "esc":
				m.mode = routesModeRouteList
			}
		}
		return m, nil

	case routesModeDeleteSlot:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				m.deleteSlot()
				m.mode = routesModeSlotList
			case "n", "N", "esc":
				m.mode = routesModeSlotList
			}
		}
		return m, nil

	case routesModeRouteList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.routeIdx > 0 {
					m.routeIdx--
				}
			case "down", "j":
				if m.routeIdx < len(m.cfg.Routes)-1 {
					m.routeIdx++
				}
			case "enter", "right", "l":
				if len(m.cfg.Routes) > 0 {
					m.mode = routesModeSlotList
					m.slotSection = 0
					m.slotIdx = 0
				}
			case "a":
				m.initAddRouteForm()
				m.mode = routesModeAddRoute
			case "d", "x":
				if len(m.cfg.Routes) > 0 {
					m.mode = routesModeDeleteRoute
				}
			}
		}
		return m, nil

	case routesModeSlotList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "left", "h":
				m.FlushIfDirty()
				m.mode = routesModeRouteList
			case "up":
				m.reorderSlotUp()
			case "down":
				m.reorderSlotDown()
			case "k":
				if m.slotIdx > 0 {
					m.slotIdx--
				}
			case "j":
				var limit int
				if m.slotSection == 0 {
					limit = len(m.cfg.Routes[m.routeIdx].Chain)
				} else {
					limit = len(m.cfg.Routes[m.routeIdx].Fallback)
				}
				if m.slotIdx < limit-1 {
					m.slotIdx++
				}
			case "space":
				m.slotSection = 1 - m.slotSection
				var limit int
				if m.slotSection == 0 {
					limit = len(m.cfg.Routes[m.routeIdx].Chain)
				} else {
					limit = len(m.cfg.Routes[m.routeIdx].Fallback)
				}
				m.slotIdx = max(0, limit-1)
			case "a":
				m.initAddSlotForm()
				m.mode = routesModeAddSlot
			case "d", "x":
				var limit int
				if m.slotSection == 0 {
					limit = len(m.cfg.Routes[m.routeIdx].Chain)
				} else {
					limit = len(m.cfg.Routes[m.routeIdx].Fallback)
				}
				if limit > 0 {
					m.mode = routesModeDeleteSlot
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m RoutesModel) View() string {
	w := m.Width
	if w < 80 {
		w = 80
	}
	h := m.Height
	if h < 24 {
		h = 24
	}

	leftWidth := 30
	rightWidth := w - 10 - leftWidth - 3

	// Left panel - Route List
	var leftRows []string
	leftRows = append(leftRows, styles.HeaderStyle.Render("Routes"))
	leftRows = append(leftRows, lipgloss.NewStyle().Foreground(styles.ColorBorder).Render("----------------------------"))

	for i, route := range m.cfg.Routes {
		prefix := "  "
		if i == m.routeIdx {
			prefix = "> "
		}
		routeText := fmt.Sprintf("%s%s", prefix, route.ModelAlias)
		if len(routeText) > leftWidth-4 {
			routeText = routeText[:leftWidth-7] + "..."
		}
		if i == m.routeIdx && m.mode == routesModeRouteList {
			leftRows = append(leftRows, lipgloss.NewStyle().
				Background(styles.ColorBorder).
				Foreground(styles.ColorAccent).
				Bold(true).
				Render(routeText))
		} else if i == m.routeIdx {
			leftRows = append(leftRows, lipgloss.NewStyle().
				Foreground(styles.ColorAccent).
				Bold(true).
				Render(routeText))
		} else {
			leftRows = append(leftRows, routeText)
		}
	}

	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).
		Height(h - 12).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(styles.ColorBorder).
		Render(lipgloss.JoinVertical(lipgloss.Left, leftRows...))

	// Right panel
	var rightPanel string
	switch m.mode {
	case routesModeAddRoute:
		rightPanel = lipgloss.NewStyle().
			Width(rightWidth).
			PaddingLeft(2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				styles.HeaderStyle.Render("Add Route"),
				"",
				m.addRouteForm.View(),
			))

	case routesModeAddSlot:
		rightPanel = lipgloss.NewStyle().
			Width(rightWidth).
			PaddingLeft(2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				styles.HeaderStyle.Render(func() string {
					if m.slotSection == 0 {
						return "Add Slot to Chain"
					}
					return "Add Slot to Fallback"
				}()),
				"",
				m.addSlotForm.View(),
			))

	case routesModeDeleteRoute:
		rightPanel = lipgloss.NewStyle().
			Width(rightWidth).
			PaddingLeft(2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				styles.HeaderStyle.Render("Confirm Deletion"),
				"",
				fmt.Sprintf("Are you sure you want to delete route '%s'?", m.cfg.Routes[m.routeIdx].ModelAlias),
				"",
				"Press [y] to confirm, [n] to cancel",
			))

	case routesModeDeleteSlot:
		rightPanel = lipgloss.NewStyle().
			Width(rightWidth).
			PaddingLeft(2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				styles.HeaderStyle.Render("Confirm Deletion"),
				"",
				"Are you sure you want to delete this slot?",
				"",
				"Press [y] to confirm, [n] to cancel",
			))

	case routesModeRouteList, routesModeSlotList:
		if len(m.cfg.Routes) == 0 {
			rightPanel = lipgloss.NewStyle().
				Width(rightWidth).
				PaddingLeft(2).
				Render("No routes configured. Press [a] to add a route.")
		} else {
			route := m.cfg.Routes[m.routeIdx]
			var details []string
			details = append(details, styles.HeaderStyle.Render("Route Details"))
			details = append(details, lipgloss.NewStyle().Foreground(styles.ColorBorder).Render("--------------------------------------"))
			details = append(details, fmt.Sprintf("Name:  %s", route.Name))
			details = append(details, fmt.Sprintf("Alias: %s", route.ModelAlias))
			details = append(details, "")

			// Primary Chain
			details = append(details, styles.HeaderStyle.Render("PRIMARY CHAIN"))
			if len(route.Chain) == 0 {
				details = append(details, "  (No slots)")
			} else {
				for i, slot := range route.Chain {
					prefix := "  "
					if m.mode == routesModeSlotList && m.slotSection == 0 && i == m.slotIdx {
						prefix = "> "
					}
					slotText := fmt.Sprintf("%s[%d] %s / %s", prefix, i+1, slot.Provider, slot.Model)
					if m.mode == routesModeSlotList && m.slotSection == 0 && i == m.slotIdx {
						details = append(details, lipgloss.NewStyle().
							Background(styles.ColorBorder).
							Foreground(styles.ColorAccent).
							Bold(true).
							Render(slotText))
					} else {
						details = append(details, slotText)
					}
				}
			}
			details = append(details, "")

			// Fallback Pool
			details = append(details, styles.HeaderStyle.Render("FALLBACK POOL"))
			if len(route.Fallback) == 0 {
				details = append(details, "  (No slots)")
			} else {
				for i, slot := range route.Fallback {
					prefix := "  "
					if m.mode == routesModeSlotList && m.slotSection == 1 && i == m.slotIdx {
						prefix = "> "
					}
					slotText := fmt.Sprintf("%s[%d] %s / %s", prefix, i+1, slot.Provider, slot.Model)
					if m.mode == routesModeSlotList && m.slotSection == 1 && i == m.slotIdx {
						details = append(details, lipgloss.NewStyle().
							Background(styles.ColorBorder).
							Foreground(styles.ColorAccent).
							Bold(true).
							Render(slotText))
					} else {
						details = append(details, slotText)
					}
				}
			}

			if m.statusMsg != "" {
				details = append(details, "", styles.CoolingStyle.Render(m.statusMsg))
			}

			// Footer guidelines
			details = append(details, "", lipgloss.NewStyle().Foreground(styles.ColorBorder).Render("--------------------------------------"))
			if m.mode == routesModeRouteList {
				details = append(details, styles.HelpStyle.Render("[a] add route  │  [d] delete route  │  [Enter] edit slots"))
			} else {
				details = append(details, styles.HelpStyle.Render("[j/k] navigate slots  │  [↑/↓] reorder slot  │  [Space] toggle pool  │  [a] add slot  │  [d] delete slot  │  [Esc] back"))
			}

			rightPanel = lipgloss.NewStyle().
				Width(rightWidth).
				PaddingLeft(2).
				Render(lipgloss.JoinVertical(lipgloss.Left, details...))
		}
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	return styles.PanelStyle.
		Width(w - 6).
		Height(h - 10).
		Render(body)
}
