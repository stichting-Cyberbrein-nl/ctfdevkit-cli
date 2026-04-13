package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/assignments"
)

// promptModel is a Bubble Tea model for a single-field path input.
type promptModel struct {
	input     textinput.Model
	title     string
	subtitle  string
	detected  string // pre-filled auto-detected path
	confirmed string // set when user presses Enter
	err       string
	quitting  bool
}

func newPromptModel(title, subtitle, placeholder, detected string) promptModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 52
	ti.Prompt = styleAccent.Render("  ❯ ")

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

func (m promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
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
	b.WriteString("\n")
	b.WriteString(styleAccent.Render("  "+m.title) + "\n")
	b.WriteString(styleMuted.Render("  "+m.subtitle) + "\n\n")

	if m.detected != "" {
		detected := lipgloss.NewStyle().
			Foreground(colorGreen).
			Render(fmt.Sprintf("  ✓ Auto-detected: %s", m.detected))
		b.WriteString(detected + "\n")
		b.WriteString(styleMuted.Render("  Press Enter to use it, or type a different path.") + "\n\n")
	}

	box := stylePanel.Width(56).Render(m.input.View())
	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(box) + "\n\n")

	if m.err != "" {
		b.WriteString(styleRed.Render("  ✗ "+m.err) + "\n\n")
	}

	b.WriteString("  " + styleKey.Render("enter") + styleHint.Render(" confirm") +
		styleMuted.Render("  ·  ") +
		styleKey.Render("esc") + styleHint.Render(" skip"))
	b.WriteString("\n")

	return b.String()
}

// AskAssignmentsPath shows an interactive prompt to configure the assignments directory.
// Returns the confirmed path, or "" if the user skipped.
func AskAssignmentsPath() (string, error) {
	detected := assignments.AutoDetect()

	m := newPromptModel(
		"Where are your CTF assignments?",
		"Enter the path to your assignments folder.",
		"/path/to/assignments",
		detected,
	)

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}

	final, ok := result.(promptModel)
	if !ok || final.confirmed == "" {
		return detected, nil // fall back to auto-detected if user skipped
	}
	return final.confirmed, nil
}
