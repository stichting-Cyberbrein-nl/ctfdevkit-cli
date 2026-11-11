package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
)

// EnvStatus represents the live state of the Docker environment.
type EnvStatus int

const (
	StatusChecking EnvStatus = iota
	StatusRunning
	StatusStopped
	StatusNotInstalled
	StatusDockerDown
)

// Label renders the status as a colored badge.
func (s EnvStatus) Label() string {
	switch s {
	case StatusChecking:
		return styleMuted.Render("checking…")
	case StatusRunning:
		return stylePill.Background(colorGreen).Render("● RUNNING")
	case StatusStopped:
		return stylePill.Background(colorYellow).Render("○ STOPPED")
	case StatusDockerDown:
		return stylePill.Background(colorRed).Render("DOCKER DOWN")
	case StatusNotInstalled:
		return stylePill.Background(colorMuted).Render("NOT INSTALLED")
	default:
		return stylePill.Background(colorMuted).Render("UNKNOWN")
	}
}

// msgStatusChecked is sent when the background status check completes.
type msgStatusChecked struct{ status EnvStatus }

// checkStatus probes Docker and the running containers in the background.
func checkStatus(composeDir string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if !docker.IsAvailable() || !docker.IsRunning(ctx) {
			return msgStatusChecked{StatusDockerDown}
		}

		if composeDir == "" {
			return msgStatusChecked{StatusNotInstalled}
		}

		out, err := docker.ComposeExecOutput(ctx, composeDir, "app", "echo", "ok")
		if err != nil || out == "" {
			return msgStatusChecked{StatusStopped}
		}
		return msgStatusChecked{StatusRunning}
	}
}

// msgLatestDevKit is sent when the release manifest has been fetched.
type msgLatestDevKit struct{ manifest *releases.Manifest }

// checkLatestDevKit fetches the release manifest in the background. Offline the menu
// simply shows the installed version without an update hint.
func checkLatestDevKit(manifestURL string) tea.Cmd {
	return func() tea.Msg {
		manifest, err := releases.Fetch(context.Background(), manifestURL)
		if err != nil {
			return nil
		}
		return msgLatestDevKit{manifest}
	}
}
