// Developed by Olivier Flentge on behalf of Cyberbrein B.V. (KvK 97562912).
// Package tui provides the interactive Bubble Tea terminal UI for devkit.
package tui

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
)

// item represents one row in the menu.
type item struct {
	group  string // section title; consecutive items with the same group are listed together
	label  string
	desc   string
	cmd    string   // devkit subcommand to run
	args   []string // extra args for the subcommand
	danger bool     // shown in red
}

var menuItems = []item{
	{group: "Environment", label: "Setup environment", desc: "Install everything and start the stack", cmd: "setup"},
	{group: "Environment", label: "Start", desc: "Start all containers", cmd: "up"},
	{group: "Environment", label: "Stop", desc: "Stop all containers", cmd: "down"},

	{group: "Inspect", label: "Health check", desc: "Run devkit:doctor in the app container", cmd: "doctor"},
	{group: "Inspect", label: "View logs", desc: "Stream live container logs", cmd: "logs"},
	{group: "Inspect", label: "Status", desc: "Container status and health check", cmd: "status"},
	{group: "Inspect", label: "Open in browser", desc: "Open the DevKit site in your browser", cmd: "open"},

	{group: "Develop", label: "Shell", desc: "Open a shell in the app container", cmd: "shell"},
	{group: "Develop", label: "Artisan", desc: "Run a php artisan command", cmd: "artisan"},
	{group: "Develop", label: "Scaffold assignment", desc: "Create a new CTF assignment skeleton", cmd: "scaffold"},
	{group: "Develop", label: "Set assignments repo", desc: "Where the assignments repo is cloned", cmd: "config-assignments"},

	{group: "Updates", label: "Update DevKit", desc: "Update the DevKit environment", cmd: "update"},
	{group: "Updates", label: "Update CLI", desc: "Self-update the devkit binary", cmd: "self-update"},

	{group: "Danger zone", label: "Prune Docker", desc: "Remove ALL unused Docker data", cmd: "prune", danger: true},
	{group: "Danger zone", label: "Reset environment", desc: "Stop containers, delete all volumes", cmd: "reset", danger: true},
}

// Menu column widths.
const (
	maxContentWidth = 88
	gutterWidth     = 13 // group titles, left of the rows
	labelWidth      = 21
	minDescWidth    = 38 // fits every description; narrower, they move to the footer
	columnGap       = 2
)

// menuLayout is the column split for the current terminal width.
type menuLayout struct {
	width  int // total content width
	gutter int // 0 = no room for group titles
	label  int
	desc   int // 0 = descriptions do not fit next to the labels
}

// Model is the Bubble Tea model for the main menu.
type Model struct {
	cfg        config.Config
	st         state.State
	version    string
	cursor     int
	status     EnvStatus
	spinner    spinner.Model // shown while the status check runs
	latest     string        // newest DevKit version from the manifest, "" until fetched
	hasUpdate  bool          // latest is newer than the installed DevKit
	composeDir string
	quitting   bool
	runCmd     string // set when user selects an item
	runArgs    []string
	width      int
	height     int
}

// New creates the initial TUI model.
func New(cfg config.Config, st state.State, version, composeDir string) Model {
	return Model{
		cfg:        cfg,
		st:         st,
		version:    version,
		status:     StatusChecking,
		spinner:    spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styleAccent)),
		composeDir: composeDir,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, checkStatus(m.composeDir), checkLatestDevKit(m.cfg.ManifestURL))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		// Dropping the tick once the status is known stops the animation.
		if m.status != StatusChecking {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case msgStatusChecked:
		m.status = msg.status
		return m, nil

	case msgLatestDevKit:
		m.latest = msg.manifest.Payload.Version
		if m.st.PayloadVersion != "" {
			newer, err := msg.manifest.IsNewerPayload(m.st.PayloadVersion)
			m.hasUpdate = err == nil && newer
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			m.cursor = (m.cursor - 1 + len(menuItems)) % len(menuItems)

		case "down", "j":
			m.cursor = (m.cursor + 1) % len(menuItems)

		case "enter", " ":
			it := menuItems[m.cursor]
			m.runCmd = it.cmd
			m.runArgs = it.args
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	l := m.layout()
	header := m.renderHeader(l.width)
	footer := m.renderFooter(l)
	rule := styleSubtle.Render(strings.Repeat("─", l.width))
	chrome := lipgloss.Height(header) + lipgloss.Height(footer) + 2 // + both rules

	var lines []string
	rows, focus := m.menuRows(l, true)
	if m.height == 0 || chrome+len(rows)+3 <= m.height {
		lines = append(lines, "", header, rule, "")
		lines = append(lines, rows...)
		lines = append(lines, "", rule, footer)
	} else {
		// Short terminal: drop the breathing room, then scroll what still does not fit.
		rows, focus = m.menuRows(l, false)
		lines = append(lines, header, rule)
		lines = append(lines, window(rows, focus, m.height-chrome, l.gutter+columnGap)...)
		lines = append(lines, rule, footer)
	}

	return styleApp.Render(strings.Join(lines, "\n"))
}

// layout splits the terminal width into the menu columns.
func (m Model) layout() menuLayout {
	w := 76 // until the terminal reports its size
	if m.width > 0 {
		w = max(min(m.width-4, maxContentWidth), 24)
	}

	l := menuLayout{width: w, label: labelWidth}
	if w >= gutterWidth+columnGap+labelWidth {
		l.gutter = gutterWidth
	}
	l.desc = w - l.gutter - columnGap - labelWidth - columnGap
	if l.desc < minDescWidth {
		l.desc = 0
		l.label = w - l.gutter - columnGap
	}
	return l
}

// renderHeader draws the logo with the versions next to it and the environment
// status right-aligned. As the terminal narrows the status first loses its detail
// line, then moves below the logo.
func (m Model) renderHeader(width int) string {
	info := lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render(m.cfg.Brand+" DevKit")+m.devKitVersion(),
		styleMuted.Render("CLI "+formatVersion(m.version)),
	)
	left := lipgloss.JoinHorizontal(lipgloss.Bottom,
		renderBanner(output.IsChristmasSeason(time.Now())), "   ", info)
	if lipgloss.Width(left) > width {
		left = info
	}

	pill, detail := m.statusPill(), m.statusDetail()
	for _, under := range []string{detail, ""} {
		right := lipgloss.JoinVertical(lipgloss.Right, pill, under)
		if gap := width - lipgloss.Width(left) - lipgloss.Width(right); gap >= columnGap {
			return lipgloss.JoinHorizontal(lipgloss.Bottom, left, strings.Repeat(" ", gap), right)
		}
	}
	return left + "\n\n" + pill + "  " + detail
}

// menuRows renders the menu, one string per line, and reports the line of the
// selected item. With gaps, a blank line separates the groups.
func (m Model) menuRows(l menuLayout, gaps bool) (rows []string, focus int) {
	active := menuItems[m.cursor].group

	for i, it := range menuItems {
		first := i == 0 || menuItems[i-1].group != it.group
		if first && gaps && i > 0 {
			rows = append(rows, "")
		}

		gutter := ""
		if l.gutter > 0 {
			title, style := "", styleMuted
			if first {
				title = strings.ToUpper(it.group)
			}
			switch {
			case it.group == active && it.danger:
				style = styleRed
			case it.group == active:
				style = styleAccent
			}
			gutter = style.Width(l.gutter).Render(title)
		}

		if i == m.cursor {
			focus = len(rows)
		}
		rows = append(rows, gutter+m.renderRow(it, i == m.cursor, l))
	}
	return rows, focus
}

// renderRow draws one item: marker, label and (when it fits) the description.
func (m Model) renderRow(it item, selected bool, l menuLayout) string {
	base := lipgloss.NewStyle()
	marker := strings.Repeat(" ", columnGap)
	labelStyle, descStyle := base, styleMuted
	switch {
	case it.danger:
		labelStyle = labelStyle.Foreground(colorRed)
	case it.cmd == "update" && m.hasUpdate:
		labelStyle = labelStyle.Foreground(colorYellow)
	}

	if selected {
		fg, bg := colorAccent, colorSelected
		if it.danger {
			fg, bg = colorRed, colorDangerBg
		}
		base = base.Background(bg)
		marker = base.Foreground(fg).Render("▌ ")
		labelStyle = base.Foreground(fg).Bold(true)
		descStyle = base
	}

	row := marker + labelStyle.Width(l.label).Render(truncate(it.label, l.label))
	if l.desc > 0 {
		row += base.Render(strings.Repeat(" ", columnGap)) +
			descStyle.Width(l.desc).Render(truncate(m.itemDesc(it), l.desc))
	}
	return row
}

// renderFooter draws the key hints and the command the selected item runs. When the
// descriptions do not fit in the menu, the selected one is shown above the hints.
func (m Model) renderFooter(l menuLayout) string {
	it := menuItems[m.cursor]

	line := keyHints("↑↓", "navigate", "enter", "run", "q", "quit")
	cmd := styleMuted.Render("devkit " + it.cmd)
	if gap := l.width - lipgloss.Width(line) - lipgloss.Width(cmd); gap >= columnGap {
		line += strings.Repeat(" ", gap) + cmd
	}

	if l.desc == 0 {
		return styleMuted.Render(truncate(m.itemDesc(it), l.width)) + "\n" + line
	}
	return line
}

// statusPill is the environment badge, or a spinner while the check is running.
func (m Model) statusPill() string {
	if m.status == StatusChecking {
		return m.spinner.View() + " " + styleMuted.Render("checking")
	}
	return m.status.Label()
}

// statusDetail is the line under the badge: where the site lives, or what to do next.
func (m Model) statusDetail() string {
	switch m.status {
	case StatusRunning:
		return styleMuted.Render(m.cfg.URL)
	case StatusStopped:
		return styleMuted.Render("Start to bring it online")
	case StatusNotInstalled:
		return styleMuted.Render("Run Setup environment first")
	case StatusDockerDown:
		return styleMuted.Render("Start Docker to continue")
	default:
		return ""
	}
}

// devKitVersion follows the title with the installed DevKit version and, when
// known, the newer one.
func (m Model) devKitVersion() string {
	if m.st.PayloadVersion == "" {
		return "" // the status badge already says it is not installed
	}
	v := " " + styleMuted.Render(formatVersion(m.st.PayloadVersion))
	if m.hasUpdate {
		v += styleYellow.Render(" → " + formatVersion(m.latest))
	}
	return v
}

// itemDesc is the description of a menu item for the current config and versions.
func (m Model) itemDesc(it item) string {
	switch {
	case it.cmd == "open" && m.cfg.URL != "":
		return "Open " + m.cfg.URL + " in your browser"
	case it.cmd == "update" && m.hasUpdate:
		return "Available: " + formatVersion(m.st.PayloadVersion) + " → " + formatVersion(m.latest)
	case it.cmd == "update" && m.latest != "" && m.st.PayloadVersion != "":
		return "Up to date (" + formatVersion(m.st.PayloadVersion) + ")"
	default:
		return it.desc
	}
}

// truncate shortens s to at most width cells, ending in an ellipsis when cut.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > width {
		r = r[:len(r)-1]
	}
	return strings.TrimRight(string(r), " ") + "…"
}

// window returns the height rows around focus, marking the edges that hide more rows.
func window(rows []string, focus, height, indent int) []string {
	height = max(height, 1)
	if len(rows) <= height {
		return rows
	}

	start := max(0, min(focus-height/2, len(rows)-height))
	out := slices.Clone(rows[start : start+height])
	if height >= 3 { // below that a marker would hide the selected row
		pad := strings.Repeat(" ", indent)
		if start > 0 {
			out[0] = pad + styleMuted.Render("↑ more")
		}
		if start+height < len(rows) {
			out[height-1] = pad + styleMuted.Render("↓ more")
		}
	}
	return out
}

var tuiBannerLines = []string{
	"█▀▄ █▀▀ █░█ █▄▀ █ ▀█▀",
	"█▄▀ ██▄ ▀▄▀ █░█ █ ░█░",
}

// renderBanner draws the DEVKIT logo; in the Christmas season the letters
// alternate red and green under a line of snow.
func renderBanner(christmas bool) string {
	var lines []string
	if christmas {
		lines = append(lines, styleSnow.Render("* . ❄  *  . ❄ . *"))
	}
	for _, line := range tuiBannerLines {
		if !christmas {
			lines = append(lines, styleAccent.Render(line))
			continue
		}
		letters := strings.Split(line, " ")
		for i, letter := range letters {
			style := styleChristmasRed
			if i%2 == 1 {
				style = styleChristmasGreen
			}
			letters[i] = style.Render(letter)
		}
		lines = append(lines, strings.Join(letters, " "))
	}
	return strings.Join(lines, "\n")
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
