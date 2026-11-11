package mcpclients

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	codexBegin = "# >>> devkit mcp (managed by `devkit mcp install`) >>>"
	codexEnd   = "# <<< devkit mcp <<<"
)

var codexUnmanaged = regexp.MustCompile(`(?m)^\s*\[mcp_servers\.ctfdevkit\]\s*$`)

// codexClient manages a marked block in Codex's config.toml.
type codexClient struct{}

func (codexClient) ID() string   { return "codex" }
func (codexClient) Name() string { return "OpenAI Codex" }

func codexHome(e Env) string { return homeOr(e, "CODEX_HOME", ".codex") }

func (codexClient) Detected(e Env) bool { return e.has("codex") || e.Exists(codexHome(e)) }

func (codexClient) Location(e Env) string { return filepath.Join(codexHome(e), "config.toml") }

func (c codexClient) Installed(_ context.Context, e Env) (bool, error) {
	data, _, err := readText(c.Location(e))
	if err != nil {
		return false, err
	}
	return strings.Contains(data, codexBegin), nil
}

func (c codexClient) Install(_ context.Context, e Env, s Server) (Result, error) {
	path := c.Location(e)
	res := Result{Client: c.ID(), Location: path}

	orig, mode, err := readText(path)
	if err != nil {
		return res, err
	}
	had := strings.Contains(orig, codexBegin)
	rest := removeCodexBlock(orig)
	if codexUnmanaged.MatchString(rest) {
		return res, fmt.Errorf("%s already defines [mcp_servers.ctfdevkit] outside the devkit block; remove it and run again", path)
	}

	nl := "\n"
	if strings.Contains(orig, "\r\n") {
		nl = "\r\n"
	}
	block := strings.Join(codexBlockLines(s), nl) + nl

	out := rest
	if strings.TrimSpace(out) != "" {
		out = strings.TrimRight(out, "\r\n") + nl + nl
	} else {
		out = ""
	}
	out += block

	if out == orig {
		res.Action = ActionUnchanged
		return res, nil
	}
	if err := writeConfigFile(path, []byte(out), mode); err != nil {
		return res, err
	}
	res.Action = ActionInstalled
	if had {
		res.Action = ActionUpdated
	}
	return res, nil
}

func (c codexClient) Uninstall(_ context.Context, e Env) (Result, error) {
	path := c.Location(e)
	res := Result{Client: c.ID(), Location: path, Action: ActionUnchanged}

	orig, mode, err := readText(path)
	if err != nil || !strings.Contains(orig, codexBegin) {
		return res, err
	}
	if err := writeConfigFile(path, []byte(removeCodexBlock(orig)), mode); err != nil {
		return res, err
	}
	res.Action = ActionRemoved
	return res, nil
}

func codexBlockLines(s Server) []string {
	args := make([]string, len(s.Args))
	for i, a := range s.Args {
		args[i] = tomlString(a)
	}
	return []string{
		codexBegin,
		"[mcp_servers.ctfdevkit]",
		"command = " + tomlString(s.Command),
		"args = [" + strings.Join(args, ", ") + "]",
		"startup_timeout_sec = 120",
		"tool_timeout_sec = 300",
		codexEnd,
	}
}

// removeCodexBlock strips the managed block and the blank line Install placed before
// it, so install followed by uninstall restores the original bytes.
func removeCodexBlock(s string) string {
	i := strings.Index(s, codexBegin)
	if i < 0 {
		return s
	}
	j := strings.Index(s[i:], codexEnd)
	if j < 0 {
		return s
	}
	before, after := s[:i], s[i+j+len(codexEnd):]
	if strings.HasPrefix(after, "\r\n") {
		after = after[2:]
	} else {
		after = strings.TrimPrefix(after, "\n")
	}
	for _, nl := range []string{"\r\n", "\n"} {
		if strings.HasSuffix(before, nl+nl) {
			before = before[:len(before)-len(nl)]
			break
		}
	}
	return before + after
}

// tomlString quotes s as a TOML basic string.
func tomlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// readText reads a file; a missing file is empty with mode 0.
func readText(path string) (string, fs.FileMode, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	return string(data), info.Mode().Perm(), nil
}
