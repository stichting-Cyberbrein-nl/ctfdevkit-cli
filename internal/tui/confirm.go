package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type confirmModel struct {
	title     string
	subtitle  string
	confirmed bool
	quitting  bool
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter, tea.KeySpace:
			m.confirmed = true
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleAccent.Render("  "+m.title) + "\n")
	b.WriteString(styleMuted.Render("  "+m.subtitle) + "\n\n")

	row := styleItemSelected.Width(52).Render("  Continue")
	panel := stylePanel.Width(56).Render(row)
	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(panel))
	b.WriteString("\n\n")

	b.WriteString("  " + styleKey.Render("enter") + styleHint.Render(" continue") +
		styleMuted.Render("  ·  ") +
		styleKey.Render("esc") + styleHint.Render(" cancel"))
	b.WriteString("\n")
	return b.String()
}

func AskConfirm(title, subtitle string) (bool, error) {
	p := tea.NewProgram(confirmModel{title: title, subtitle: subtitle})
	result, err := p.Run()
	if err != nil {
		return false, err
	}

	final, ok := result.(confirmModel)
	return ok && final.confirmed, nil
}
