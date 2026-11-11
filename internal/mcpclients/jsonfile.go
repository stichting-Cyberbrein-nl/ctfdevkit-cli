package mcpclients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

// jsonClient registers the server under "mcpServers" in a JSON config file.
type jsonClient struct {
	id, name string
	path     func(Env) string
	detect   func(Env) bool
	entry    func(Server) map[string]any
}

func (c jsonClient) ID() string            { return c.id }
func (c jsonClient) Name() string          { return c.name }
func (c jsonClient) Detected(e Env) bool   { return c.detect(e) }
func (c jsonClient) Location(e Env) string { return c.path(e) }

func (c jsonClient) Installed(_ context.Context, e Env) (bool, error) {
	doc, _, err := readJSONConfig(c.path(e))
	if err != nil {
		return false, err
	}
	servers, err := mcpServers(c.path(e), doc)
	if err != nil {
		return false, err
	}
	_, ok := servers[ServerName]
	return ok, nil
}

func (c jsonClient) Install(_ context.Context, e Env, s Server) (Result, error) {
	path := c.path(e)
	res := Result{Client: c.id, Location: path}

	entry, err := normalize(c.entry(s))
	if err != nil {
		return res, err
	}

	doc, mode, err := readJSONConfig(path)
	if err != nil {
		snippet, _ := json.MarshalIndent(map[string]any{ServerName: entry}, "", "  ")
		return res, fmt.Errorf("%w. Add this entry to its \"mcpServers\" object yourself:\n%s", err, snippet)
	}
	servers, err := mcpServers(path, doc)
	if err != nil {
		return res, err
	}

	existing, had := servers[ServerName]
	if had && reflect.DeepEqual(existing, entry) {
		res.Action = ActionUnchanged
		return res, nil
	}
	servers[ServerName] = entry
	doc["mcpServers"] = servers

	if err := writeJSONConfig(path, doc, mode); err != nil {
		return res, err
	}
	res.Action = ActionInstalled
	if had {
		res.Action = ActionUpdated
	}
	return res, nil
}

func (c jsonClient) Uninstall(_ context.Context, e Env) (Result, error) {
	path := c.path(e)
	res := Result{Client: c.id, Location: path, Action: ActionUnchanged}

	doc, mode, err := readJSONConfig(path)
	if err != nil {
		return res, err
	}
	servers, err := mcpServers(path, doc)
	if err != nil {
		return res, err
	}
	if _, ok := servers[ServerName]; !ok {
		return res, nil
	}
	delete(servers, ServerName)
	if len(servers) == 0 {
		delete(doc, "mcpServers")
	} else {
		doc["mcpServers"] = servers
	}
	if err := writeJSONConfig(path, doc, mode); err != nil {
		return res, err
	}
	res.Action = ActionRemoved
	return res, nil
}

// readJSONConfig reads path as a JSON object. A missing or blank file is an empty object.
// mode is 0 when the file does not exist yet.
func readJSONConfig(path string) (map[string]any, fs.FileMode, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, info.Mode().Perm(), nil
	}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, 0, fmt.Errorf("%s is not valid JSON (%v)", path, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, info.Mode().Perm(), nil
}

func mcpServers(path string, doc map[string]any) (map[string]any, error) {
	raw, ok := doc["mcpServers"]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}
	servers, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: \"mcpServers\" is not an object", path)
	}
	return servers, nil
}

// normalize round-trips v through JSON so it compares equal to decoded file content.
func normalize(v map[string]any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	err = dec.Decode(&out)
	return out, err
}

func writeJSONConfig(path string, doc map[string]any, mode fs.FileMode) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return err
	}
	return writeConfigFile(path, buf.Bytes(), mode)
}

// writeConfigFile backs up an existing file to <path>.devkit-backup and replaces it
// atomically. New files get 0600 and their parent directories 0700.
func writeConfigFile(path string, data []byte, mode fs.FileMode) error {
	if mode == 0 {
		mode = 0o600
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
	} else {
		orig, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path+".devkit-backup", orig, mode); err != nil {
			return fmt.Errorf("backing up %s: %w", path, err)
		}
	}

	tmp := path + ".devkit-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
