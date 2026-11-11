package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// promptPrefix is the input marker; the textinput width excludes it.
const promptPrefix = "❯ "

// promptModel is a Bubble Tea model for a single-field path input.
type promptModel struct {
	input     textinput.Model
	title     string
	subtitle  string
	detected  string // pre-filled suggested path
	confirmed string // set when user presses Enter
	err       string
	width     int
	quitting  bool
}

func newPromptModel(title, subtitle, placeholder, detected string) promptModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = promptInputWidth(0)
	ti.Prompt = styleAccent.Render(promptPrefix)
	ti.PlaceholderStyle = styleMuted

	if detected != "" {
		ti.SetValue(detected)
	}

	return promptModel{
		input:    ti,
		title:    title,
		subtitle: subtitle,
		detected: detected,
	}
}

// promptInputWidth leaves room for the prefix and the cursor inside the card.
func promptInputWidth(termWidth int) int {
	return dialogInnerWidth(termWidth) - len([]rune(promptPrefix)) - 1
}

func (m promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = promptInputWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if strings.TrimSpace(m.input.Value()) == "" && m.detected != "" {
				m.confirmed = m.detected
			}
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			val := strings.TrimSpace(m.input.Value())
			if val == "" && m.detected != "" {
				val = m.detected
			}
			if val == "" {
				m.err = "Path cannot be empty"
				return m, nil
			}
			// Expand ~ manually.
			if strings.HasPrefix(val, "~/") {
				home, _ := os.UserHomeDir()
				val = home + val[1:]
			}
			m.confirmed = val
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = ""
	return m, cmd
}

func (m promptModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(styleMuted.Render(m.subtitle) + "\n\n")

	escHint := "cancel"
	if m.detected != "" {
		b.WriteString(styleGreen.Render("✓ Suggested: "+m.detected) + "\n")
		b.WriteString(styleMuted.Render("Press Enter to use it, or type a different path.") + "\n\n")
		escHint = "use suggestion"
	}

	b.WriteString(m.input.View())
	if m.err != "" {
		b.WriteString("\n" + styleRed.Render("✗ "+m.err))
	}

	return renderDialog(m.width, colorAccent, m.title, b.String(),
		keyHints("enter", "confirm", "esc", escHint))
}

// AskAssignmentsPath shows an interactive prompt to choose where DevKit places the assignments repo.
func AskAssignmentsPath(suggested string) (string, error) {
	m := newPromptModel(
		"Where should DevKit place the assignments repo?",
		"Enter the path where the public assignments repo should live.",
		"/path/to/assignments",
		suggested,
	)

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}

	final, ok := result.(promptModel)
	if !ok || final.confirmed == "" {
		return suggested, nil
	}
	return final.confirmed, nil
}
