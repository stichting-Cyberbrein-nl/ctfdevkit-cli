package mcpclients

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// errClaudeMissing is returned when the claude CLI is not on PATH.
var errClaudeMissing = errors.New("Claude Code CLI ('claude') not found on PATH")

// claudeClient registers the server through the `claude mcp` commands (user scope).
type claudeClient struct{}

func (claudeClient) ID() string          { return "claude" }
func (claudeClient) Name() string        { return "Claude Code" }
func (claudeClient) Detected(e Env) bool { return e.has("claude") }
func (claudeClient) Location(Env) string { return "claude mcp (user scope)" }

func (c claudeClient) Installed(ctx context.Context, e Env) (bool, error) {
	if !e.has("claude") {
		return false, nil
	}
	_, err := e.Run(ctx, "claude", "mcp", "get", ServerName)
	return err == nil, nil
}

func (c claudeClient) Install(ctx context.Context, e Env, s Server) (Result, error) {
	res := Result{Client: c.ID(), Location: c.Location(e)}
	if !e.has("claude") {
		return res, errClaudeMissing
	}
	had, _ := c.Installed(ctx, e)
	_, _ = e.Run(ctx, "claude", "mcp", "remove", "--scope", "user", ServerName)

	args := append([]string{"mcp", "add", "--scope", "user", ServerName, "--", s.Command}, s.Args...)
	if out, err := e.Run(ctx, "claude", args...); err != nil {
		return res, fmt.Errorf("claude mcp add failed: %v %s", err, strings.TrimSpace(string(out)))
	}
	res.Action = ActionInstalled
	if had {
		res.Action = ActionUpdated
	}
	return res, nil
}

func (c claudeClient) Uninstall(ctx context.Context, e Env) (Result, error) {
	res := Result{Client: c.ID(), Location: c.Location(e), Action: ActionUnchanged}
	if !e.has("claude") {
		return res, nil
	}
	if _, err := e.Run(ctx, "claude", "mcp", "remove", "--scope", "user", ServerName); err == nil {
		res.Action = ActionRemoved
	}
	return res, nil
}

// IsMissingBinary reports whether err means the client's CLI is not installed.
func IsMissingBinary(err error) bool { return errors.Is(err, errClaudeMissing) }
