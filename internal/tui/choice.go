package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// choiceModel lets the user pick one option from a list.
type choiceModel struct {
	title    string
	subtitle string
	options  []string
	cursor   int
	width    int
	chosen   bool
	quitting bool
}

func (m choiceModel) Init() tea.Cmd { return nil }

func (m choiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = (m.cursor - 1 + len(m.options)) % len(m.options)
		case "down", "j":
			m.cursor = (m.cursor + 1) % len(m.options)
		case "enter":
			m.chosen = true
			m.quitting = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m choiceModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(styleMuted.Render(m.subtitle) + "\n\n")
	for i, opt := range m.options {
		if i == m.cursor {
			b.WriteString(styleAccent.Render(promptPrefix + opt))
		} else {
			b.WriteString("  " + opt)
		}
		if i < len(m.options)-1 {
			b.WriteString("\n")
		}
	}
	return renderDialog(m.width, colorAccent, m.title, b.String(),
		keyHints("↑↓", "move", "enter", "select", "esc", "cancel"))
}

// AskChoice shows a list and returns the index of the selected option.
func AskChoice(title, subtitle string, options []string, initial int) (int, error) {
	if len(options) == 0 {
		return 0, ErrCancelled
	}
	m := choiceModel{title: title, subtitle: subtitle, options: options, cursor: max(0, min(initial, len(options)-1))}
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return 0, err
	}
	final, ok := result.(choiceModel)
	if !ok || !final.chosen {
		return 0, ErrCancelled
	}
	return final.cursor, nil
}
