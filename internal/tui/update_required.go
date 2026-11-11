package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type requiredUpdateModel struct {
	current  string
	latest   string
	width    int
	accepted bool
	quitting bool
}

func (m requiredUpdateModel) Init() tea.Cmd {
	return nil
}

func (m requiredUpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
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

	body := "DevKit CLI " + styleMuted.Render(formatVersion(m.current)+" → ") +
		styleTitle.Render(formatVersion(m.latest)) + "\n\n" +
		styleMuted.Render("You need to update before you can continue using DevKit.") + "\n\n" +
		styleButton.Background(colorYellow).Render("Update now")
	return renderDialog(m.width, colorYellow, "Update required", body,
		keyHints("enter", "update now", "esc", "cancel"))
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
