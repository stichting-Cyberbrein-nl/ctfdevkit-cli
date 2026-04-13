package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type requiredUpdateModel struct {
	current  string
	latest   string
	accepted bool
	quitting bool
}

func (m requiredUpdateModel) Init() tea.Cmd {
	return nil
}

func (m requiredUpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter, tea.KeySpace:
			m.accepted = true
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m requiredUpdateModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleYellow.Render("  Update available! ("+formatVersion(m.current)+") -> ("+formatVersion(m.latest)+")") + "\n")
	b.WriteString(styleMuted.Render("  Je moet eerst updaten voordat je DevKit verder gebruikt.") + "\n\n")

	row := styleItemSelected.Width(52).Render("  Update now")
	panel := stylePanel.Width(56).Render(row)
	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(panel))
	b.WriteString("\n\n")

	b.WriteString("  " + styleKey.Render("enter") + styleHint.Render(" update now"))
	b.WriteString("\n")
	return b.String()
}

// AskRequiredUpdate blocks the normal TUI behind a required update prompt.
func AskRequiredUpdate(current, latest string) (bool, error) {
	p := tea.NewProgram(requiredUpdateModel{current: current, latest: latest})
	result, err := p.Run()
	if err != nil {
		return false, err
	}

	final, ok := result.(requiredUpdateModel)
	return ok && final.accepted, nil
}

func formatVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}
