package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type confirmModel struct {
	title     string
	subtitle  string
	width     int
	confirmed bool
	quitting  bool
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
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

	body := styleMuted.Render(m.subtitle) + "\n\n" + styleButton.Render("Continue")
	return renderDialog(m.width, colorAccent, m.title, body,
		keyHints("enter", "continue", "esc", "cancel"))
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
