package mcpclients

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type recorder struct{ calls [][]string }

func testEnv(t *testing.T, bins ...string) (Env, *recorder) {
	t.Helper()
	rec := &recorder{}
	home := t.TempDir()
	return Env{
		Home:   home,
		Getenv: func(string) string { return "" },
		LookPath: func(name string) (string, error) {
			for _, b := range bins {
				if b == name {
					return "/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		},
		Exists: func(p string) bool { _, err := os.Stat(p); return err == nil },
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			rec.calls = append(rec.calls, append([]string{name}, args...))
			if len(args) > 1 && args[1] == "get" {
				return nil, errors.New("not found")
			}
			return nil, nil
		},
	}, rec
}

var testServer = Server{Command: `C:\Users\a b\devkit.exe`, Args: []string{"mcp", "serve"}}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCopilotJSON(t *testing.T) {
	ctx := context.Background()
	env, _ := testEnv(t)
	c, _ := ByID("copilot")
	path := c.Location(env)
	writeFile(t, path, `{"mcpServers": {"other": {"command": "x"}}, "keep": 1}`)

	steps := []struct {
		name string
		run  func() (Result, error)
		want string
	}{
		{"install", func() (Result, error) { return c.Install(ctx, env, testServer) }, ActionInstalled},
		{"reinstall", func() (Result, error) { return c.Install(ctx, env, testServer) }, ActionUnchanged},
		{"uninstall", func() (Result, error) { return c.Uninstall(ctx, env) }, ActionRemoved},
	}
	for _, s := range steps {
		res, err := s.run()
		if err != nil || res.Action != s.want {
			t.Fatalf("%s: %+v, %v", s.name, res, err)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(readFile(t, path)), &doc); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		servers := doc["mcpServers"].(map[string]any)
		if _, ok := servers["other"]; !ok || doc["keep"] == nil {
			t.Fatalf("%s lost existing content: %v", s.name, doc)
		}
		_, has := servers[ServerName]
		if has != (s.name != "uninstall") {
			t.Fatalf("%s: ctfdevkit present = %v", s.name, has)
		}
		if s.name == "install" {
			entry := servers[ServerName].(map[string]any)
			if entry["type"] != "stdio" || entry["command"] != testServer.Command {
				t.Fatalf("entry = %v", entry)
			}
		}
	}
	if _, err := os.Stat(path + ".devkit-backup"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestInvalidJSON(t *testing.T) {
	env, _ := testEnv(t)
	c, _ := ByID("cursor")
	path := c.Location(env)
	writeFile(t, path, `{"mcpServers": `)

	_, err := c.Install(context.Background(), env, testServer)
	if err == nil || !strings.Contains(err.Error(), "is not valid JSON") || !strings.Contains(err.Error(), `"ctfdevkit"`) {
		t.Fatalf("err = %v", err)
	}
	if got := readFile(t, path); got != `{"mcpServers": ` {
		t.Fatalf("file changed: %q", got)
	}
}

func TestCodexTOML(t *testing.T) {
	ctx := context.Background()
	env, _ := testEnv(t)
	c, _ := ByID("codex")
	path := c.Location(env)
	orig := "model = \"o3\"\r\n\r\n[mcp_servers.other]\r\ncommand = \"x\"\r\n"
	writeFile(t, path, orig)

	if res, err := c.Install(ctx, env, testServer); err != nil || res.Action != ActionInstalled {
		t.Fatalf("install: %+v, %v", res, err)
	}
	got := readFile(t, path)
	wantBlock := codexBegin + "\r\n[mcp_servers.ctfdevkit]\r\ncommand = \"C:\\\\Users\\\\a b\\\\devkit.exe\"\r\n"
	if !strings.HasPrefix(got, orig+"\r\n") || !strings.Contains(got, wantBlock) || strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Fatalf("install wrote %q", got)
	}

	if res, err := c.Install(ctx, env, Server{Command: "/usr/bin/devkit", Args: []string{"mcp", "serve"}}); err != nil || res.Action != ActionUpdated {
		t.Fatalf("reinstall: %+v, %v", res, err)
	}
	got = readFile(t, path)
	if strings.Count(got, codexBegin) != 1 || !strings.Contains(got, `command = "/usr/bin/devkit"`) {
		t.Fatalf("reinstall wrote %q", got)
	}

	if res, err := c.Uninstall(ctx, env); err != nil || res.Action != ActionRemoved {
		t.Fatalf("uninstall: %+v, %v", res, err)
	}
	if got := readFile(t, path); got != orig {
		t.Fatalf("uninstall left %q, want %q", got, orig)
	}

	writeFile(t, path, "[mcp_servers.ctfdevkit]\ncommand = \"x\"\n")
	if _, err := c.Install(ctx, env, testServer); err == nil || !strings.Contains(err.Error(), "outside the devkit block") {
		t.Fatalf("conflict err = %v", err)
	}
}

func TestTOMLString(t *testing.T) {
	tests := []struct{ in, want string }{
		{`C:\Users\a b\devkit.exe`, `"C:\\Users\\a b\\devkit.exe"`},
		{`say "hi"`, `"say \"hi\""`},
	}
	for _, tt := range tests {
		if got := tomlString(tt.in); got != tt.want {
			t.Errorf("tomlString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestClaudeInstall(t *testing.T) {
	env, rec := testEnv(t, "claude")
	c, _ := ByID("claude")
	res, err := c.Install(context.Background(), env, testServer)
	if err != nil || res.Action != ActionInstalled {
		t.Fatalf("install: %+v, %v", res, err)
	}
	want := [][]string{
		{"claude", "mcp", "remove", "--scope", "user", "ctfdevkit"},
		{"claude", "mcp", "add", "--scope", "user", "ctfdevkit", "--", testServer.Command, "mcp", "serve"},
	}
	if n := len(rec.calls); n < 2 || !reflect.DeepEqual(rec.calls[n-2:], want) {
		t.Fatalf("calls = %v", rec.calls)
	}

	noClaude, _ := testEnv(t)
	if _, err := c.Install(context.Background(), noClaude, testServer); !IsMissingBinary(err) {
		t.Fatalf("missing binary err = %v", err)
	}
}

func TestProbeRW(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()
	go func() {
		dec := json.NewDecoder(serverR)
		for {
			var req map[string]any
			if err := dec.Decode(&req); err != nil {
				return
			}
			switch req["method"] {
			case "initialize":
				io.WriteString(serverW, `{"jsonrpc":"2.0","method":"notifications/message","params":{}}`+"\n")
				io.WriteString(serverW, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"CTF:DevKit","version":"1.0.0"}}}`+"\n")
			case "tools/list":
				io.WriteString(serverW, `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"list-assignments"},{"name":"check-assignment"}]}}`+"\n")
			}
		}
	}()

	res, err := probeRW(ctx, clientR, clientW, "test")
	if err != nil {
		t.Fatal(err)
	}
	if res.ServerName != "CTF:DevKit" || res.ServerVersion != "1.0.0" || !reflect.DeepEqual(res.Tools, []string{"list-assignments", "check-assignment"}) {
		t.Fatalf("res = %+v", res)
	}
	clientW.Close()
	serverW.Close()
}
