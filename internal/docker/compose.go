package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// composeArgs builds the base docker compose argument slice for a given compose directory.
func composeArgs(dir string, extra ...string) []string {
	args := []string{"compose", "-f", filepath.Join(dir, "docker-compose.yml")}
	return append(args, extra...)
}

// ComposeUp runs `docker compose up -d` in the given directory, streaming to stdout.
func ComposeUp(ctx context.Context, dir string) error {
	return ComposeUpTo(ctx, dir, os.Stdout)
}

// ComposeUpTo runs `docker compose up -d`, writing compose output to w.
// Images are only pulled when missing; `devkit update` pulls new versions explicitly.
// On failure it attempts one retry after clearing the builder cache.
func ComposeUpTo(ctx context.Context, dir string, w io.Writer) error {
	args := composeArgs(dir, "up", "-d", "--pull", "missing")
	run := func() error {
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Stdout = w
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := run(); err != nil {
		// Retry once after clearing builder cache.
		_ = exec.CommandContext(ctx, "docker", "builder", "prune", "-f").Run()
		return run()
	}
	return nil
}

// IsServiceRunning reports whether the compose service is running. An error means
// docker itself could not be queried (daemon down, compose file missing).
func IsServiceRunning(ctx context.Context, dir, service string) (bool, error) {
	out, err := exec.CommandContext(ctx, "docker", composeArgs(dir, "ps", "--status", "running", "--services")...).Output()
	if err != nil {
		return false, err
	}
	return containsLine(string(out), service), nil
}

func containsLine(out, want string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// ComposeExecStdio runs a command in the service with the process's stdin, stdout and
// stderr attached directly (no TTY), passing env as `-e KEY=VALUE`.
func ComposeExecStdio(ctx context.Context, dir, service string, env map[string]string, cmdArgs ...string) error {
	args := composeArgs(dir, "exec", "-T")
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+env[k])
	}
	args = append(args, service)
	args = append(args, cmdArgs...)
	return runAttached(ctx, "docker", args...)
}

// ComposeDown stops and removes containers.
func ComposeDown(ctx context.Context, dir string) error {
	return runStreamed(ctx, "docker", composeArgs(dir, "down")...)
}

// ComposeReset stops containers and removes volumes.
func ComposeReset(ctx context.Context, dir string) error {
	return runStreamed(ctx, "docker", composeArgs(dir, "down", "-v", "--remove-orphans")...)
}

// ComposeLogs streams logs for an optional service to stdout.
// Pass an empty service string to stream all services.
func ComposeLogs(ctx context.Context, dir, service string) error {
	args := composeArgs(dir, "logs", "-f")
	if service != "" {
		args = append(args, service)
	}
	return runStreamed(ctx, "docker", args...)
}

// ComposePS prints the container status table.
func ComposePS(ctx context.Context, dir string) error {
	return runStreamed(ctx, "docker", composeArgs(dir, "ps")...)
}

// ComposeExec runs a command inside a container, attaching stdin/stdout/stderr.
// Use this for interactive commands like `shell`.
func ComposeExec(ctx context.Context, dir, container string, interactive bool, cmdArgs ...string) error {
	args := composeArgs(dir, "exec")
	if !interactive {
		args = append(args, "-T")
	}
	args = append(args, container)
	args = append(args, cmdArgs...)
	return runAttached(ctx, "docker", args...)
}

// ReloadProxy restarts the proxy container so it picks up new TLS certs.
func ReloadProxy(ctx context.Context, dir string) error {
	return runStreamed(ctx, "docker", composeArgs(dir, "restart", "proxy")...)
}

// Pull pulls the latest image for the given image reference.
func Pull(ctx context.Context, image string) error {
	return runStreamed(ctx, "docker", "pull", image)
}

// ImageID returns the local ID of an image reference, or "" when it is not present.
func ImageID(ctx context.Context, image string) string {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.Id}}", image).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runStreamed runs a command with stdout/stderr connected to the terminal.
func runStreamed(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runAttached runs a command with stdin, stdout, and stderr all connected.
func runAttached(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Validate checks that docker and docker compose are both working.
func Validate(ctx context.Context) error {
	if !IsAvailable() {
		return fmt.Errorf("docker is not installed or not in PATH")
	}
	if !HasComposePlugin(ctx) {
		return fmt.Errorf("docker compose plugin is not available — update Docker Desktop or install docker-compose-plugin")
	}
	return nil
}
