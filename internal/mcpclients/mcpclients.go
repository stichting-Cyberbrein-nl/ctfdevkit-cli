// Package mcpclients registers the DevKit MCP server in AI coding tools
// (Claude Code, Codex, GitHub Copilot, Cursor, Gemini CLI).
package mcpclients

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// ServerName is the key under which the DevKit server is registered.
const ServerName = "ctfdevkit"

// Server is the command an AI tool starts to reach the DevKit MCP server.
type Server struct {
	Command string
	Args    []string
}

// Env abstracts the host so clients can be tested without touching the real home directory.
type Env struct {
	Home     string
	Getenv   func(string) string
	LookPath func(string) (string, error)
	Exists   func(string) bool
	Run      func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// DefaultEnv returns the real host environment.
func DefaultEnv() (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, err
	}
	return Env{
		Home:     home,
		Getenv:   os.Getenv,
		LookPath: exec.LookPath,
		Exists: func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		},
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
	}, nil
}

func (e Env) has(bin string) bool {
	_, err := e.LookPath(bin)
	return err == nil
}

// Result describes what Install or Uninstall did.
type Result struct {
	Client   string
	Action   string // installed|updated|unchanged|removed|skipped
	Location string
	Note     string
}

// Result actions.
const (
	ActionInstalled = "installed"
	ActionUpdated   = "updated"
	ActionUnchanged = "unchanged"
	ActionRemoved   = "removed"
	ActionSkipped   = "skipped"
)

// Client is one supported AI tool.
type Client interface {
	ID() string
	Name() string
	Detected(Env) bool
	Location(Env) string
	Install(context.Context, Env, Server) (Result, error)
	Uninstall(context.Context, Env) (Result, error)
	Installed(context.Context, Env) (bool, error)
}

// All returns every supported client in display order.
func All() []Client {
	return []Client{claudeClient{}, codexClient{}, copilotClient(), cursorClient(), geminiClient()}
}

// IDs returns the IDs of all clients in display order.
func IDs() []string {
	var ids []string
	for _, c := range All() {
		ids = append(ids, c.ID())
	}
	return ids
}

// ByID returns the client with the given ID.
func ByID(id string) (Client, bool) {
	for _, c := range All() {
		if c.ID() == id {
			return c, true
		}
	}
	return nil, false
}

func homeOr(e Env, envVar string, def ...string) string {
	if v := e.Getenv(envVar); v != "" {
		return v
	}
	return filepath.Join(append([]string{e.Home}, def...)...)
}

func copilotClient() Client {
	home := func(e Env) string { return homeOr(e, "COPILOT_HOME", ".copilot") }
	return jsonClient{
		id:   "copilot",
		name: "GitHub Copilot (VS Code + CLI)",
		path: func(e Env) string { return filepath.Join(home(e), "mcp-config.json") },
		detect: func(e Env) bool {
			return e.has("copilot") || e.has("code") || e.Exists(home(e))
		},
		entry: func(s Server) map[string]any {
			return map[string]any{"type": "stdio", "command": s.Command, "args": toAny(s.Args), "tools": []any{"*"}}
		},
	}
}

func cursorClient() Client {
	return jsonClient{
		id:     "cursor",
		name:   "Cursor",
		path:   func(e Env) string { return filepath.Join(e.Home, ".cursor", "mcp.json") },
		detect: func(e Env) bool { return e.Exists(filepath.Join(e.Home, ".cursor")) || e.has("cursor") },
		entry: func(s Server) map[string]any {
			return map[string]any{"command": s.Command, "args": toAny(s.Args)}
		},
	}
}

func geminiClient() Client {
	return jsonClient{
		id:     "gemini",
		name:   "Gemini CLI",
		path:   func(e Env) string { return filepath.Join(e.Home, ".gemini", "settings.json") },
		detect: func(e Env) bool { return e.has("gemini") || e.Exists(filepath.Join(e.Home, ".gemini")) },
		entry: func(s Server) map[string]any {
			return map[string]any{"command": s.Command, "args": toAny(s.Args)}
		},
	}
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
