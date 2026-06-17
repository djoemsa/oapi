package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Curated modern color palette
const (
	ColorActive   = lipgloss.Color("#00D46A") // Vibrant Green
	ColorCooling  = lipgloss.Color("#FFD700") // Warm Yellow
	ColorError    = lipgloss.Color("#FF4136") // Bright Red
	ColorAccent   = lipgloss.Color("#7B61FF") // Purple Accent
	ColorMuted    = lipgloss.Color("#6B7280") // Grey Muted
	ColorText     = lipgloss.Color("#F9FAFB") // Crisp Off-White
	ColorBorder   = lipgloss.Color("#374151") // Dark Border
	ColorBg       = lipgloss.Color("#111827") // Dark background tint
)

// Base UI styles
var (
	// Tab Styles
	ActiveTabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorAccent).
			Padding(0, 2).
			Foreground(ColorAccent).
			Bold(true)

	InactiveTabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorBorder).
			Padding(0, 2).
			Foreground(ColorMuted)

	TabBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorBorder)

	// Panel & Box Styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)

	// Status Badges
	ActiveStyle = lipgloss.NewStyle().
			Foreground(ColorActive).
			Bold(true)

	CoolingStyle = lipgloss.NewStyle().
			Foreground(ColorCooling).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Footer Help Text
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
)
