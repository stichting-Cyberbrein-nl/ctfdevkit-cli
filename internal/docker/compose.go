package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// composeArgs builds the base docker compose argument slice for a given compose directory.
func composeArgs(dir string, extra ...string) []string {
	args := []string{"compose", "-f", filepath.Join(dir, "docker-compose.yml")}
	return append(args, extra...)
}

// ComposeUp runs `docker compose up -d` in the given directory.
// On failure it attempts one retry after clearing the builder cache.
func ComposeUp(ctx context.Context, dir string) error {
	args := composeArgs(dir, "up", "-d", "--pull", "always")
	if err := runStreamed(ctx, "docker", args...); err != nil {
		// Retry once after clearing builder cache.
		_ = exec.CommandContext(ctx, "docker", "builder", "prune", "-f").Run()
		return runStreamed(ctx, "docker", args...)
	}
	return nil
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

// ComposeExecOutput runs a command inside a container and returns stdout.
func ComposeExecOutput(ctx context.Context, dir, container string, cmdArgs ...string) (string, error) {
	args := composeArgs(dir, "exec", "-T", container)
	args = append(args, cmdArgs...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	return string(out), err
}

// ReloadProxy restarts the proxy container so it picks up new TLS certs.
func ReloadProxy(ctx context.Context, dir string) error {
	return runStreamed(ctx, "docker", composeArgs(dir, "restart", "proxy")...)
}

// Pull pulls the latest image for the given image reference.
func Pull(ctx context.Context, image string) error {
	return runStreamed(ctx, "docker", "pull", image)
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

// runSilent runs a command discarding its output.
func runSilent(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// EnsureUp starts the app container if it is not already running.
func EnsureUp(ctx context.Context, dir string) error {
	out, err := ComposeExecOutput(ctx, dir, "app", "echo", "ok")
	if err != nil || out == "" {
		return ComposeUp(ctx, dir)
	}
	return nil
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
