package views

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"oapi/internal/config"
	"oapi/internal/rotation"
	"oapi/internal/testutil"
	"oapi/internal/tui/styles"
)

type viewMode int

const (
	modeView viewMode = iota
	modeDeleteConfirm
	modeImportPath
	modeForm
)

type TestResultMsg struct {
	Index  int
	Status string
	Err    error
}

type ProvidersModel struct {
	ctx          context.Context
	cfg          *config.Config
	configPath   string
	stateMgr     *config.StateManager
	pool         *rotation.KeyPool
	Width        int
	Height       int

	mode          viewMode
	selectedIndex int

	// Deletion confirmation
	deleteIndex int

	// Bulk import
	importPath string
	importForm *huh.Form

	// Add/Edit Huh form
	form      *huh.Form
	isEdit    bool
	editIndex int

	// Form values
	formID                  *string
	formProvider            *string
	formModel               *string
	formAPIKey              *string
	formRPMLimitStr         *string
	formRPDLimitStr         *string
	formTPMLimitStr         *string
	formStatus              *string

	// Connection testing state
	testMsg        string
	testingIndices map[int]bool
}

func NewProvidersModel(
	ctx context.Context,
	cfg *config.Config,
	configPath string,
	stateMgr *config.StateManager,
	pool *rotation.KeyPool,
) ProvidersModel {
	formID := ""
	formProvider := ""
	formModel := ""
	formAPIKey := ""
	formRPMLimitStr := ""
	formRPDLimitStr := ""
	formTPMLimitStr := ""
	formStatus := ""

	return ProvidersModel{
		ctx:            ctx,
		cfg:            cfg,
		configPath:     configPath,
		stateMgr:       stateMgr,
		pool:           pool,
		mode:           modeView,
		testingIndices: make(map[int]bool),
		formID:         &formID,
		formProvider:   &formProvider,
		formModel:      &formModel,
		formAPIKey:     &formAPIKey,
		formRPMLimitStr: &formRPMLimitStr,
		formRPDLimitStr: &formRPDLimitStr,
		formTPMLimitStr: &formTPMLimitStr,
		formStatus:     &formStatus,
	}
}

func (m ProvidersModel) Init() tea.Cmd {
	return nil
}

func (m ProvidersModel) IsEditing() bool {
	return m.mode == modeForm || m.mode == modeImportPath
}

func (m *ProvidersModel) initForm(isEdit bool) {
	m.isEdit = isEdit
	if isEdit {
		key := m.cfg.Keys[m.selectedIndex]
		*m.formID = key.ID
		*m.formProvider = key.Provider
		*m.formModel = key.Model
		*m.formAPIKey = key.APIKey
		*m.formRPMLimitStr = strconv.Itoa(key.RPMLimit)
		*m.formRPDLimitStr = strconv.Itoa(key.RPDLimit)
		*m.formTPMLimitStr = strconv.Itoa(key.TPMLimit)
		*m.formStatus = key.Status
	} else {
		*m.formID = ""
		*m.formProvider = "groq"
		*m.formModel = ""
		*m.formAPIKey = ""
		*m.formRPMLimitStr = "0"
		*m.formRPDLimitStr = "0"
		*m.formTPMLimitStr = "0"
		*m.formStatus = "active"
	}

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Provider").
				Options(
					huh.NewOption("Groq", "groq"),
					// "Google" is disabled for now
					// huh.NewOption("Google", "google"),
					huh.NewOption("Cerebras", "cerebras"),
					huh.NewOption("GitHub", "github"),
					huh.NewOption("OpenRouter", "openrouter"),
					huh.NewOption("Mistral", "mistral"),
					huh.NewOption("NVIDIA NIM", "nvidia"),
					huh.NewOption("OpenCode Zen", "opencode"),
					huh.NewOption("Cohere", "cohere"),
				).
				Value(m.formProvider),
			huh.NewInput().
				Title("Model").
				Placeholder("e.g. llama-3.3-70b-versatile").
				Value(m.formModel).
				Validate(func(str string) error {
					if len(str) == 0 {
						return fmt.Errorf("model is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("API Key").
				Placeholder("API Key").
				EchoMode(huh.EchoModePassword).
				Value(m.formAPIKey).
				Validate(func(str string) error {
					if len(str) == 0 {
						return fmt.Errorf("API key is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("RPM Limit (optional, 0 for default)").
				Value(m.formRPMLimitStr).
				Validate(validateInt),
			huh.NewInput().
				Title("RPD Limit (optional, 0 for default)").
				Value(m.formRPDLimitStr).
				Validate(validateInt),
			huh.NewInput().
				Title("TPM Limit (optional, 0 for default)").
				Value(m.formTPMLimitStr).
				Validate(validateInt),
			huh.NewSelect[string]().
				Title("Status").
				Options(
					huh.NewOption("Active", "active"),
					huh.NewOption("Error", "error"),
				).
				Value(m.formStatus),
		),
	).WithTheme(huh.ThemeCharm())

	m.form.Init()
}

func validateInt(str string) error {
	if str == "" {
		return nil
	}
	_, err := strconv.Atoi(str)
	if err != nil {
		return fmt.Errorf("must be a valid integer")
	}
	return nil
}

func (m *ProvidersModel) initImportForm() {
	m.importPath = ""
	m.importForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("JSON Import Path").
				Placeholder("e.g. keys.json").
				Value(&m.importPath).
				Validate(func(str string) error {
					if len(str) == 0 {
						return fmt.Errorf("path is required")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCharm())

	m.importForm.Init()
}

func (m *ProvidersModel) testKeyCmd(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.cfg.Keys) {
		return nil
	}
	key := m.cfg.Keys[idx]
	return func() tea.Msg {
		client := testutil.DefaultHTTPClient()
		status, err := testutil.ProbeKey(client, key)
		return TestResultMsg{Index: idx, Status: status, Err: err}
	}
}

func (m *ProvidersModel) saveForm() error {
	rpm, _ := strconv.Atoi(*m.formRPMLimitStr)
	rpd, _ := strconv.Atoi(*m.formRPDLimitStr)
	tpm, _ := strconv.Atoi(*m.formTPMLimitStr)

	if m.isEdit {
		m.cfg.Keys[m.selectedIndex].Provider = *m.formProvider
		m.cfg.Keys[m.selectedIndex].Model = *m.formModel
		m.cfg.Keys[m.selectedIndex].APIKey = *m.formAPIKey
		m.cfg.Keys[m.selectedIndex].RPMLimit = rpm
		m.cfg.Keys[m.selectedIndex].RPDLimit = rpd
		m.cfg.Keys[m.selectedIndex].TPMLimit = tpm
		m.cfg.Keys[m.selectedIndex].Status = *m.formStatus
	} else {
		id := fmt.Sprintf("%s-%s-%d", *m.formProvider, *m.formModel, time.Now().Unix())
		newKey := config.KeyConfig{
			ID:       id,
			Provider: *m.formProvider,
			Model:    *m.formModel,
			APIKey:   *m.formAPIKey,
			RPMLimit: rpm,
			RPDLimit: rpd,
			TPMLimit: tpm,
			Status:   *m.formStatus,
		}
		m.cfg.Keys = append(m.cfg.Keys, newKey)
		m.selectedIndex = len(m.cfg.Keys) - 1
	}

	err := config.SaveConfig(m.configPath, m.cfg)
	if err != nil {
		return err
	}
	m.pool.UpdateConfig(m.cfg)
	return nil
}

func (m *ProvidersModel) deleteKey(idx int) {
	if idx < 0 || idx >= len(m.cfg.Keys) {
		return
	}
	m.cfg.Keys = append(m.cfg.Keys[:idx], m.cfg.Keys[idx+1:]...)
	if m.selectedIndex >= len(m.cfg.Keys) && m.selectedIndex > 0 {
		m.selectedIndex = len(m.cfg.Keys) - 1
	}
	_ = config.SaveConfig(m.configPath, m.cfg)
	m.pool.UpdateConfig(m.cfg)
	m.testMsg = "Key deleted successfully"
}

func (m *ProvidersModel) performImport() error {
	data, err := os.ReadFile(m.importPath)
	if err != nil {
		return err
	}
	var imported []config.KeyConfig
	if err := json.Unmarshal(data, &imported); err != nil {
		return err
	}

	for _, key := range imported {
		if key.ID == "" {
			key.ID = fmt.Sprintf("%s-%s-%d", key.Provider, key.Model, time.Now().UnixNano())
		}
		if key.Status == "" {
			key.Status = "active"
		}
		m.cfg.Keys = append(m.cfg.Keys, key)
	}

	err = config.SaveConfig(m.configPath, m.cfg)
	if err != nil {
		return err
	}
	m.pool.UpdateConfig(m.cfg)
	m.selectedIndex = len(m.cfg.Keys) - 1
	return nil
}

func (m ProvidersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}

	switch m.mode {
	case modeForm:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			m.mode = modeView
			return m, nil
		}

		var newForm tea.Model
		newForm, cmd = m.form.Update(msg)
		m.form = newForm.(*huh.Form)
		cmds = append(cmds, cmd)

		if m.form.State == huh.StateCompleted {
			err := m.saveForm()
			if err != nil {
				m.testMsg = fmt.Sprintf("Error saving: %v", err)
				m.mode = modeView
				return m, tea.Batch(cmds...)
			} else {
				m.testMsg = fmt.Sprintf("Key saved. Testing connection for key %s...", m.cfg.Keys[m.selectedIndex].ID)
				m.testingIndices[m.selectedIndex] = true
				m.mode = modeView
				cmds = append(cmds, m.testKeyCmd(m.selectedIndex))
				return m, tea.Batch(cmds...)
			}
		} else if m.form.State == huh.StateAborted {
			m.mode = modeView
		}
		return m, tea.Batch(cmds...)

	case modeImportPath:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			m.mode = modeView
			return m, nil
		}

		var newForm tea.Model
		newForm, cmd = m.importForm.Update(msg)
		m.importForm = newForm.(*huh.Form)
		cmds = append(cmds, cmd)

		if m.importForm.State == huh.StateCompleted {
			err := m.performImport()
			if err != nil {
				m.testMsg = fmt.Sprintf("Import error: %v", err)
			} else {
				m.testMsg = "Keys imported successfully"
			}
			m.mode = modeView
		} else if m.importForm.State == huh.StateAborted {
			m.mode = modeView
		}
		return m, tea.Batch(cmds...)

	case modeDeleteConfirm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				m.deleteKey(m.deleteIndex)
				m.mode = modeView
			case "n", "N", "esc":
				m.mode = modeView
			}
		}
		return m, nil

	case modeView:
		switch msg := msg.(type) {
		case TestResultMsg:
			m.testingIndices[msg.Index] = false
			if msg.Err != nil {
				m.testMsg = fmt.Sprintf("Key %d (%s) test failed: %v", msg.Index+1, m.cfg.Keys[msg.Index].ID, msg.Err)
				m.cfg.Keys[msg.Index].Status = "error"
			} else {
				m.testMsg = fmt.Sprintf("Key %d (%s) test succeeded: active", msg.Index+1, m.cfg.Keys[msg.Index].ID)
				m.cfg.Keys[msg.Index].Status = msg.Status
			}
			m.stateMgr.UpdateState(func(s *config.RuntimeState) {
				ks := s.Keys[m.cfg.Keys[msg.Index].ID]
				if msg.Status == "cooling_rpm" {
					d := time.Now().Add(time.Minute)
					ks.CoolingUntil = &d
				} else {
					ks.CoolingUntil = nil
				}
				s.Keys[m.cfg.Keys[msg.Index].ID] = ks
			})
			_ = m.stateMgr.SaveState()
			_ = config.SaveConfig(m.configPath, m.cfg)
			m.pool.UpdateConfig(m.cfg)

		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case "down", "j":
				if m.selectedIndex < len(m.cfg.Keys)-1 {
					m.selectedIndex++
				}
			case "a":
				m.initForm(false)
				m.mode = modeForm
			case "e":
				if len(m.cfg.Keys) > 0 {
					m.initForm(true)
					m.mode = modeForm
				}
			case "d":
				if len(m.cfg.Keys) > 0 {
					m.deleteIndex = m.selectedIndex
					m.mode = modeDeleteConfirm
				}
			case "i":
				m.initImportForm()
				m.mode = modeImportPath
			case "t":
				if len(m.cfg.Keys) > 0 {
					m.testingIndices[m.selectedIndex] = true
					m.testMsg = fmt.Sprintf("Testing connection for key %s...", m.cfg.Keys[m.selectedIndex].ID)
					return m, m.testKeyCmd(m.selectedIndex)
				}
			case "T":
				if len(m.cfg.Keys) > 0 {
					m.testMsg = "Testing all connections..."
					for i := range m.cfg.Keys {
						m.testingIndices[i] = true
						cmds = append(cmds, m.testKeyCmd(i))
					}
					return m, tea.Batch(cmds...)
				}
			}
		}
	}

	return m, nil
}

func (m ProvidersModel) View() string {
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

	// Left panel
	var keyRows []string
	keyRows = append(keyRows, styles.HeaderStyle.Render("API Keys"))
	keyRows = append(keyRows, lipgloss.NewStyle().Foreground(styles.ColorBorder).Render("----------------------------"))

	for i, key := range m.cfg.Keys {
		prefix := "  "
		if i == m.selectedIndex {
			prefix = "> "
		}
		statusIcon := "●"
		statusStyle := styles.ActiveStyle
		if key.Status != "active" {
			statusStyle = styles.ErrorStyle
		}
		if m.testingIndices[i] {
			statusStyle = styles.CoolingStyle
			statusIcon = "↻"
		}

		keyText := fmt.Sprintf("%s%s (%s)", prefix, key.Provider, key.Model)
		if len(keyText) > leftWidth-4 {
			keyText = keyText[:leftWidth-7] + "..."
		}
		if i == m.selectedIndex {
			keyRows = append(keyRows, lipgloss.NewStyle().
				Background(styles.ColorBorder).
				Foreground(styles.ColorAccent).
				Bold(true).
				Render(fmt.Sprintf("%s %s", statusStyle.Render(statusIcon), keyText)))
		} else {
			keyRows = append(keyRows, fmt.Sprintf("%s %s", statusStyle.Render(statusIcon), keyText))
		}
	}

	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).
		Height(h - 12).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(styles.ColorBorder).
		Render(lipgloss.JoinVertical(lipgloss.Left, keyRows...))

	// Right panel
	var rightPanel string
	switch m.mode {
	case modeForm:
		rightPanel = lipgloss.NewStyle().
			Width(rightWidth).
			PaddingLeft(2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				styles.HeaderStyle.Render(func() string {
					if m.isEdit {
						return "Edit Key"
					}
					return "Add Key"
				}()),
				"",
				m.form.View(),
			))

	case modeImportPath:
		rightPanel = lipgloss.NewStyle().
			Width(rightWidth).
			PaddingLeft(2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				styles.HeaderStyle.Render("Bulk Import JSON Keys"),
				"",
				m.importForm.View(),
			))

	case modeDeleteConfirm:
		rightPanel = lipgloss.NewStyle().
			Width(rightWidth).
			PaddingLeft(2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				styles.HeaderStyle.Render("Confirm Deletion"),
				"",
				fmt.Sprintf("Are you sure you want to delete key %s?", m.cfg.Keys[m.deleteIndex].ID),
				"",
				"Press [y] to confirm, [n] to cancel",
			))

	case modeView:
		if len(m.cfg.Keys) == 0 {
			rightPanel = lipgloss.NewStyle().
				Width(rightWidth).
				PaddingLeft(2).
				Render("No keys configured. Press [a] to add a key, or [i] to bulk import.")
		} else {
			key := m.cfg.Keys[m.selectedIndex]
			maskedKey := config.MaskAPIKey(key.APIKey)

			details := []string{
				styles.HeaderStyle.Render("Key Details"),
				lipgloss.NewStyle().Foreground(styles.ColorBorder).Render("--------------------------------------"),
				fmt.Sprintf("ID:       %s", key.ID),
				fmt.Sprintf("Provider: %s", key.Provider),
				fmt.Sprintf("Model:    %s", key.Model),
				fmt.Sprintf("API Key:  %s", maskedKey),
				fmt.Sprintf("RPM:      %d (Limit)", key.RPMLimit),
				fmt.Sprintf("RPD:      %d (Limit)", key.RPDLimit),
				fmt.Sprintf("TPM:      %d (Limit)", key.TPMLimit),
				fmt.Sprintf("Status:   %s", key.Status),
			}

			if m.testMsg != "" {
				details = append(details, "", styles.CoolingStyle.Render(m.testMsg))
			}

			// Add instructions panel at the bottom of key details
			details = append(details, "",
				lipgloss.NewStyle().Foreground(styles.ColorBorder).Render("--------------------------------------"),
				styles.HelpStyle.Render("[a] add  │  [e] edit  │  [d] delete  │  [i] import  │  [t] test  │  [T] test all"),
			)

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
