package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// dialogMaxWidth is the outer width of a dialog card on a roomy terminal.
const dialogMaxWidth = 64

// dialogWidth returns the outer card width for a terminal of termWidth columns
// (0 = not reported yet).
func dialogWidth(termWidth int) int {
	if termWidth == 0 {
		return dialogMaxWidth
	}
	return max(min(termWidth-4, dialogMaxWidth), 24)
}

// dialogInnerWidth is the text width inside the card's border and padding.
func dialogInnerWidth(termWidth int) int {
	return dialogWidth(termWidth) - 2 - styleCard.GetHorizontalPadding()
}

// renderDialog draws the card shared by the inline prompts: a title in the accent
// color, the body below it and the key hints underneath the card.
func renderDialog(termWidth int, accent lipgloss.TerminalColor, title, body, keys string) string {
	card := styleCard.
		BorderForeground(accent).
		Width(dialogWidth(termWidth) - 2).
		Render(styleTitle.Foreground(accent).Render(title) + "\n\n" + body)
	// Hints line up with the text inside the card.
	indent := strings.Repeat(" ", 1+styleCard.GetPaddingLeft())
	return "\n" + styleApp.Render(card+"\n"+indent+keys) + "\n"
}

// keyHints renders "key action" pairs for a footer line.
func keyHints(pairs ...string) string {
	hints := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		hints = append(hints, styleKey.Render(pairs[i])+" "+styleHint.Render(pairs[i+1]))
	}
	return strings.Join(hints, styleSubtle.Render("  ·  "))
}
