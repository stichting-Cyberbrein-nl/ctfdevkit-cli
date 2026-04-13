package tui

import "github.com/charmbracelet/lipgloss"

// Brand palette — cyan/blue Cyberbrein identity.
var (
	colorAccent    = lipgloss.Color("#00D7FF") // cyan
	colorAccentDim = lipgloss.Color("#0087AF") // darker cyan
	colorGreen     = lipgloss.Color("#00D787") // success
	colorYellow    = lipgloss.Color("#FFD700") // warning
	colorRed       = lipgloss.Color("#FF5F5F") // error / danger
	colorMuted     = lipgloss.Color("#626262") // faint text
	colorBg        = lipgloss.Color("#0D0D0D") // near-black bg
	colorSelected  = lipgloss.Color("#1C3A4A") // selected row bg
	colorBorder    = lipgloss.Color("#1E3A4A") // panel border
	colorWhite     = lipgloss.Color("#EBEBEB") // primary text
	colorDim       = lipgloss.Color("#4A4A4A") // separator
)

// Text styles.
var (
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleAccent  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)
	styleGreen   = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleYellow  = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleRed     = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleWhite   = lipgloss.NewStyle().Foreground(colorWhite)
	styleDim     = lipgloss.NewStyle().Foreground(colorDim)
)

// Menu panel — the outer bordered box.
var stylePanel = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 1)

// Selected menu item.
var styleItemSelected = lipgloss.NewStyle().
	Background(colorSelected).
	Foreground(colorAccent).
	Bold(true).
	PaddingLeft(1).
	PaddingRight(1)

// Normal menu item.
var styleItem = lipgloss.NewStyle().
	Foreground(colorWhite).
	PaddingLeft(1).
	PaddingRight(1)

// Disabled / separator item.
var styleItemDisabled = lipgloss.NewStyle().
	Foreground(colorDim).
	PaddingLeft(1).
	PaddingRight(1)

// Status pill styles.
var (
	statusRunning = lipgloss.NewStyle().
		Foreground(colorBg).
		Background(colorGreen).
		Bold(true).
		Padding(0, 1)

	statusStopped = lipgloss.NewStyle().
		Foreground(colorBg).
		Background(colorYellow).
		Bold(true).
		Padding(0, 1)

	statusUnknown = lipgloss.NewStyle().
		Foreground(colorBg).
		Background(colorMuted).
		Bold(true).
		Padding(0, 1)
)

// Footer key hint style.
var styleKey = lipgloss.NewStyle().
	Foreground(colorAccentDim).
	Bold(true)

var styleHint = lipgloss.NewStyle().
	Foreground(colorMuted)
