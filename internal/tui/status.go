package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
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

func (s EnvStatus) Label() string {
	switch s {
	case StatusChecking:
		return "  checking…"
	case StatusRunning:
		return statusRunning.Render(" ● RUNNING ")
	case StatusStopped:
		return statusStopped.Render(" ○ STOPPED ")
	case StatusDockerDown:
		return statusUnknown.Render(" ⚠ DOCKER DOWN ")
	case StatusNotInstalled:
		return statusUnknown.Render(" ✗ NOT INSTALLED ")
	default:
		return statusUnknown.Render(" ? UNKNOWN ")
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
