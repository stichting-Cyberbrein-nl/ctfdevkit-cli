package tui

import "github.com/charmbracelet/lipgloss"

// Brand palette — cyan Cyberbrein identity. Every color is a light/dark pair so the
// UI stays readable on light terminals; primary text uses the terminal's own foreground.
var (
	colorAccent   = lipgloss.AdaptiveColor{Light: "#007A99", Dark: "#00D7FF"}
	colorGreen    = lipgloss.AdaptiveColor{Light: "#00875A", Dark: "#00D787"}
	colorYellow   = lipgloss.AdaptiveColor{Light: "#946800", Dark: "#FFD75F"}
	colorRed      = lipgloss.AdaptiveColor{Light: "#C4343A", Dark: "#FF5F5F"}
	colorMuted    = lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#8A8A8A"} // secondary text
	colorSubtle   = lipgloss.AdaptiveColor{Light: "#C8C8C8", Dark: "#3C3C3C"} // rules, borders
	colorSelected = lipgloss.AdaptiveColor{Light: "#DCEFF5", Dark: "#12303D"} // selected row bg
	colorDangerBg = lipgloss.AdaptiveColor{Light: "#F8DEDE", Dark: "#3D1A1C"} // selected danger row bg
	colorOnFill   = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0D0D0D"} // text on pills/buttons
)

// Text styles.
var (
	styleTitle  = lipgloss.NewStyle().Bold(true)
	styleAccent = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleMuted  = lipgloss.NewStyle().Foreground(colorMuted)
	styleSubtle = lipgloss.NewStyle().Foreground(colorSubtle)
	styleGreen  = lipgloss.NewStyle().Foreground(colorGreen)
	styleYellow = lipgloss.NewStyle().Foreground(colorYellow)
	styleRed    = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
)

// Christmas banner styles.
var (
	styleChristmasRed   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E8303A")).Bold(true)
	styleChristmasGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("#2EB85C")).Bold(true)
	styleSnow           = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7FA8C9", Dark: "#D7EFFF"})
)

// styleApp is the left margin shared by every screen.
var styleApp = lipgloss.NewStyle().PaddingLeft(2)

// styleCard frames the inline dialogs (confirm, prompt, required update).
var styleCard = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(1, 2)

// styleButton is the default action of a dialog.
var styleButton = lipgloss.NewStyle().
	Foreground(colorOnFill).
	Background(colorAccent).
	Bold(true).
	Padding(0, 2)

// stylePill is the base of the environment status badge.
var stylePill = lipgloss.NewStyle().
	Foreground(colorOnFill).
	Bold(true).
	Padding(0, 1)

// Footer key hints.
var (
	styleKey  = lipgloss.NewStyle().Foreground(colorAccent)
	styleHint = lipgloss.NewStyle().Foreground(colorMuted)
)
