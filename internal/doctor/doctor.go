// Package doctor runs Laravel health checks inside the app container.
package doctor

import (
	"context"
	"fmt"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

const (
	defaultAttempts = 45
	pollInterval    = 2 * time.Second
)

// WaitForReady polls `php artisan devkit:doctor --strict` until it succeeds or times out.
func WaitForReady(ctx context.Context, composeDir, container string) error {
	return WaitForReadyWithAttempts(ctx, composeDir, container, defaultAttempts)
}

// WaitForReadyWithAttempts is the same as WaitForReady but with a configurable attempt count.
func WaitForReadyWithAttempts(ctx context.Context, composeDir, container string, attempts int) error {
	output.Info("Waiting for application to become healthy...")

	for i := 1; i <= attempts; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check cancelled")
		default:
		}

		_, err := docker.ComposeExecOutput(ctx, composeDir, container,
			"php", "artisan", "devkit:doctor", "--strict")
		if err == nil {
			return nil
		}

		output.Infof("  Not ready yet (%d/%d)...", i, attempts)
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("application did not become healthy after %d attempts — run `devkit logs` to investigate", attempts)
}

// Run executes the doctor command once, streaming output to the terminal.
func Run(ctx context.Context, composeDir, container string) error {
	return docker.ComposeExec(ctx, composeDir, container, false,
		"php", "artisan", "devkit:doctor")
}
