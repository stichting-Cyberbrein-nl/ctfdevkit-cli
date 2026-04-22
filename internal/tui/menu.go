// Developed by Olivier Flentge on behalf of Cyberbrein B.V. (KvK 97562912).
// Package tui provides the interactive Bubble Tea terminal UI for devkit.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
)

// item represents one row in the menu.
type item struct {
	icon   string
	label  string
	desc   string
	cmd    string   // devkit subcommand to run, "" = separator
	args   []string // extra args for the subcommand
	danger bool     // shown in red
}

var menuItems = []item{
	{icon: "⚡", label: "Setup environment", desc: "Install payload, certs, hosts, start containers", cmd: "setup"},
	{icon: "▶", label: "Start (up)", desc: "Start all containers", cmd: "up"},
	{icon: "■", label: "Stop (down)", desc: "Stop all containers", cmd: "down"},
	{icon: "", label: "", desc: ""}, // separator
	{icon: "♥", label: "Health check", desc: "Run devkit:doctor inside the app container", cmd: "doctor"},
	{icon: "≡", label: "View logs", desc: "Stream live container logs", cmd: "logs"},
	{icon: "◉", label: "Status", desc: "Show container status + health check", cmd: "status"},
	{icon: "🌐", label: "Open in browser", desc: "Open " + "https://ctf.dev" + " in your browser", cmd: "open"},
	{icon: "", label: "", desc: ""}, // separator
	{icon: "$", label: "Shell", desc: "Interactive shell in the app container", cmd: "shell"},
	{icon: ">", label: "Artisan", desc: "Run php artisan <command> in container", cmd: "artisan"},
	{icon: "+", label: "Scaffold assignment", desc: "Create a new CTF assignment skeleton", cmd: "scaffold"},
	{icon: "📁", label: "Set assignments repo", desc: "Choose where the public assignments repo is cloned", cmd: "config-assignments"},
	{icon: "", label: "", desc: ""}, // separator
	{icon: "↑", label: "Update payload", desc: "Pull latest Docker image from Docker Hub", cmd: "update"},
	{icon: "↑", label: "Update CLI", desc: "Self-update the devkit binary", cmd: "self-update"},
	{icon: "", label: "", desc: ""}, // separator
	{icon: "✕", label: "Prune Docker", desc: "Remove ALL unused Docker data", cmd: "prune", danger: true},
	{icon: "↺", label: "Reset environment", desc: "Stop containers + delete all volumes", cmd: "reset", danger: true},
}

// Model is the Bubble Tea model for the main menu.
type Model struct {
	cfg        config.Config
	st         state.State
	version    string
	cursor     int
	status     EnvStatus
	composeDir string
	quitting   bool
	runCmd     string // set when user selects an item
	runArgs    []string
	width      int
	height     int
}

// New creates the initial TUI model.
func New(cfg config.Config, st state.State, version, composeDir string) Model {
	m := Model{
		cfg:        cfg,
		st:         st,
		version:    version,
		status:     StatusChecking,
		composeDir: composeDir,
		cursor:     0,
	}
	// Skip separators on initial cursor.
	m.cursor = m.firstSelectable()
	return m
}

func (m Model) Init() tea.Cmd {
	return checkStatus(m.composeDir)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case msgStatusChecked:
		m.status = msg.status
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			m.moveCursor(-1)

		case "down", "j":
			m.moveCursor(1)

		case "enter", " ":
			it := menuItems[m.cursor]
			if it.cmd == "" {
				return m, nil
			}
			m.runCmd = it.cmd
			m.runArgs = it.args
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *Model) moveCursor(dir int) {
	for {
		m.cursor += dir
		if m.cursor < 0 {
			m.cursor = len(menuItems) - 1
		}
		if m.cursor >= len(menuItems) {
			m.cursor = 0
		}
		if menuItems[m.cursor].cmd != "" {
			break
		}
	}
}

func (m Model) firstSelectable() int {
	for i, it := range menuItems {
		if it.cmd != "" {
			return i
		}
	}
	return 0
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// ── Banner ────────────────────────────────────────────────────────────────
	banner := styleAccent.Render("  █▀▄ █▀▀ █░█ █▄▀ █ ▀█▀") + "\n" +
		styleAccent.Render("  █▄▀ ██▄ ▀▄▀ █░█ █ ░█░") + "\n"

	b.WriteString(banner)
	b.WriteString("\n")

	// Brand line
	b.WriteString(styleBold.Render("  "+m.cfg.Brand+" DevKit") + "  " + styleMuted.Render("v"+m.version) + "\n")

	// Status pill
	b.WriteString("  " + m.status.Label())
	if m.status == StatusRunning {
		b.WriteString("  " + styleMuted.Render(m.cfg.URL))
	}
	b.WriteString("\n\n")

	// ── Menu items ────────────────────────────────────────────────────────────
	panelWidth := 56
	var rows []string

	for i, it := range menuItems {
		if it.cmd == "" {
			// Separator
			rows = append(rows, styleDim.Render(strings.Repeat("─", panelWidth-2)))
			continue
		}

		selected := i == m.cursor

		icon := it.icon
		if icon == "" {
			icon = " "
		}

		label := it.label
		desc := it.desc

		if selected {
			labelStr := fmt.Sprintf(" %s  %-28s", icon, label)
			descStr := styleMuted.Render(desc)
			row := styleItemSelected.Width(panelWidth - 2).Render(labelStr)
			rows = append(rows, row)
			_ = descStr
		} else {
			prefix := styleMuted.Render(fmt.Sprintf(" %s  ", icon))
			var labelRender string
			if it.danger {
				labelRender = styleRed.Render(label)
			} else {
				labelRender = styleWhite.Render(label)
			}
			row := styleItem.Render(prefix + labelRender)
			rows = append(rows, row)
		}
	}

	panel := stylePanel.Width(panelWidth).Render(strings.Join(rows, "\n"))
	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(panel))
	b.WriteString("\n\n")

	// ── Description of selected item ─────────────────────────────────────────
	if m.cursor < len(menuItems) && menuItems[m.cursor].cmd != "" {
		b.WriteString(styleMuted.Render("  → " + menuItems[m.cursor].desc))
		b.WriteString("\n\n")
	}

	// ── Footer keybindings ────────────────────────────────────────────────────
	keys := []string{
		styleKey.Render("↑/↓") + styleHint.Render(" navigate"),
		styleKey.Render("enter") + styleHint.Render(" select"),
		styleKey.Render("q") + styleHint.Render(" quit"),
	}
	b.WriteString("  " + strings.Join(keys, styleMuted.Render("  ·  ")))
	b.WriteString("\n")

	return b.String()
}

// Run launches the TUI and blocks until the user selects an item or quits.
// If the user selects an action it re-execs the current binary with that subcommand.
func Run(ctx context.Context, cfg config.Config, st state.State, version, composeDir string) error {
	m := New(cfg, st, version, composeDir)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result, ok := finalModel.(Model)
	if !ok || result.runCmd == "" {
		return nil // user quit without selecting
	}

	// Re-exec the current binary with the selected subcommand.
	self, err := os.Executable()
	if err != nil {
		return err
	}

	args := append([]string{result.runCmd}, result.runArgs...)

	// For commands that need a prompt (scaffold, artisan), drop to plain terminal.
	cmd := exec.CommandContext(ctx, self, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
