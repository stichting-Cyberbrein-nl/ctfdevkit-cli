package mcpclients

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ProbeResult is what an MCP server reported during the handshake.
type ProbeResult struct {
	ServerName    string
	ServerVersion string
	Tools         []string
	Stderr        string
}

// Probe starts cmd, performs the MCP initialize handshake and lists the tools.
func Probe(ctx context.Context, cmd string, args []string, clientVersion string) (ProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	proc := exec.Command(cmd, args...)
	stdin, err := proc.StdinPipe()
	if err != nil {
		return ProbeResult{}, err
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return ProbeResult{}, err
	}
	var stderr lockedBuffer
	proc.Stderr = &stderr
	if err := proc.Start(); err != nil {
		return ProbeResult{}, err
	}

	res, probeErr := probeRW(ctx, stdout, stdin, clientVersion)

	_ = stdin.Close()
	done := make(chan struct{})
	go func() { _ = proc.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = proc.Process.Kill()
		<-done
	}

	res.Stderr = tailLines(stderr.String(), 20)
	return res, probeErr
}

type rpcMessage struct {
	ID     *json.RawMessage `json:"id"`
	Result json.RawMessage  `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// probeRW speaks newline-delimited JSON-RPC 2.0 over r/w.
func probeRW(ctx context.Context, r io.Reader, w io.Writer, clientVersion string) (ProbeResult, error) {
	var res ProbeResult

	send := func(v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(append(data, '\n'))
		return err
	}

	lines := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			select {
			case lines <- line:
			case <-ctx.Done():
				return
			}
		}
		err := sc.Err()
		if err == nil {
			err = io.EOF
		}
		readErr <- err
	}()

	await := func(id string) (json.RawMessage, error) {
		for {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("timed out waiting for the MCP server")
			case err := <-readErr:
				return nil, fmt.Errorf("MCP server closed the connection: %v", err)
			case line := <-lines:
				line = bytes.TrimSpace(line)
				if len(line) == 0 || line[0] != '{' {
					continue
				}
				var msg rpcMessage
				if json.Unmarshal(line, &msg) != nil || msg.ID == nil || strings.TrimSpace(string(*msg.ID)) != id {
					continue
				}
				if msg.Error != nil {
					return nil, errors.New(msg.Error.Message)
				}
				return msg.Result, nil
			}
		}
	}

	if err := send(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "devkit-cli", "version": clientVersion},
		},
	}); err != nil {
		return res, err
	}
	raw, err := await("1")
	if err != nil {
		return res, err
	}
	var init struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(raw, &init); err != nil {
		return res, fmt.Errorf("invalid initialize response: %w", err)
	}
	res.ServerName, res.ServerVersion = init.ServerInfo.Name, init.ServerInfo.Version

	if err := send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return res, err
	}
	if err := send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}}); err != nil {
		return res, err
	}
	raw, err = await("2")
	if err != nil {
		return res, err
	}
	var list struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return res, fmt.Errorf("invalid tools/list response: %w", err)
	}
	for _, t := range list.Tools {
		res.Tools = append(res.Tools, t.Name)
	}
	return res, nil
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
