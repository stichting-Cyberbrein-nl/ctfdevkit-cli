// Developed by Olivier Flentge on behalf of Cyberbrein B.V. (KvK 97562912).
// Package output provides terminal output helpers with color and progress support.
package output

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

var (
	bold   = color.New(color.Bold)
	red    = color.New(color.FgRed, color.Bold)
	green  = color.New(color.FgGreen, color.Bold)
	yellow = color.New(color.FgYellow, color.Bold)
	blue   = color.New(color.FgBlue, color.Bold)
	cyan   = color.New(color.FgCyan, color.Bold)
	faint  = color.New(color.Faint)
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}
}

var bannerLines = []string{
	"██████╗ ███████╗██╗   ██╗██╗  ██╗██╗████████╗",
	"██╔══██╗██╔════╝██║   ██║██║ ██╔╝██║╚══██╔══╝",
	"██║  ██║█████╗  ██║   ██║█████╔╝ ██║   ██║   ",
	"██║  ██║██╔══╝  ╚██╗ ██╔╝██╔═██╗ ██║   ██║   ",
	"██████╔╝███████╗ ╚████╔╝ ██║  ██╗██║   ██║   ",
	"╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝╚═╝   ╚═╝   ",
}

// bannerLetterWidths are the rune widths of D, E, V, K, I and T in bannerLines.
var bannerLetterWidths = []int{8, 8, 9, 8, 3, 9}

// IsChristmasSeason reports whether the banners use the Christmas theme
// (November and December), so it switches off by itself in January.
func IsChristmasSeason(now time.Time) bool {
	return now.Month() == time.November || now.Month() == time.December
}

// Banner prints the branded devkit header.
func Banner(brand, version string) {
	fmt.Println()
	if IsChristmasSeason(time.Now()) {
		faint.Println("  *   .  ❄    *    .   ❄   .    *    ❄   .  *")
		for _, line := range bannerLines {
			printChristmasLine(line)
		}
	} else {
		for _, line := range bannerLines {
			cyan.Printf("  %s\n", line)
		}
	}
	fmt.Println()
	bold.Printf("  %s DevKit  ", brand)
	faint.Printf("v%s\n", version)
	fmt.Println()
}

// printChristmasLine colors the banner letters alternately red and green.
func printChristmasLine(line string) {
	runes := []rune(line)
	fmt.Print("  ")
	pos := 0
	for i, w := range bannerLetterWidths {
		letter := string(runes[pos : pos+w])
		if i%2 == 0 {
			red.Print(letter)
		} else {
			green.Print(letter)
		}
		pos += w
	}
	fmt.Println(string(runes[pos:]))
}

// Step prints a numbered step indicator.
func Step(current, total int, msg string) {
	blue.Printf("  [%d/%d] ", current, total)
	fmt.Println(msg)
}

// Info prints an informational message.
func Info(msg string) {
	blue.Print("  ● ")
	fmt.Println(msg)
}

// Infof prints a formatted informational message.
func Infof(format string, args ...any) {
	Info(fmt.Sprintf(format, args...))
}

// Warn prints a warning message.
func Warn(msg string) {
	yellow.Print("  ⚠ ")
	fmt.Println(msg)
}

// Warnf prints a formatted warning message.
func Warnf(format string, args ...any) {
	Warn(fmt.Sprintf(format, args...))
}

// Success prints a success message.
func Success(msg string) {
	green.Print("  ✓ ")
	fmt.Println(msg)
}

// Successf prints a formatted success message.
func Successf(format string, args ...any) {
	Success(fmt.Sprintf(format, args...))
}

// Fail prints a failure/error message.
func Fail(msg string) {
	red.Print("  ✗ ")
	fmt.Println(msg)
}

// Failf prints a formatted failure message.
func Failf(format string, args ...any) {
	Fail(fmt.Sprintf(format, args...))
}

// Fatal prints a failure message and exits with code 1.
func Fatal(msg string) {
	Fail(msg)
	os.Exit(1)
}

// Fatalf prints a formatted failure message and exits with code 1.
func Fatalf(format string, args ...any) {
	Fatal(fmt.Sprintf(format, args...))
}

// Header prints a bold section header.
func Header(msg string) {
	fmt.Println()
	bold.Printf("  %s\n", msg)
	fmt.Println("  " + strings.Repeat("─", len(msg)))
}

// Plain prints a plain message with leading indent.
func Plain(msg string) {
	fmt.Printf("  %s\n", msg)
}

// Plainf prints a formatted plain message.
func Plainf(format string, args ...any) {
	Plain(fmt.Sprintf(format, args...))
}

// Divider prints a horizontal divider line.
func Divider() {
	faint.Println("  " + strings.Repeat("─", 50))
}

// Hint prints a faint hint/tip message.
func Hint(msg string) {
	faint.Printf("  → %s\n", msg)
}

// Hintf prints a formatted hint message.
func Hintf(format string, args ...any) {
	Hint(fmt.Sprintf(format, args...))
}
