package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

// requireWebapp returns a client for a running DevKit that speaks this CLI's contract.
func requireWebapp(ctx context.Context) (*webapp.Client, webapp.Info, error) {
	cfg := configFrom(ctx)
	s := stateFrom(ctx)

	if !s.IsPayloadInstalled() {
		return nil, webapp.Info{}, fmt.Errorf("payload not installed — run `devkit setup` first")
	}
	composeDir, err := payload.ComposeDir(s)
	if err != nil {
		return nil, webapp.Info{}, err
	}

	running, err := docker.IsServiceRunning(ctx, composeDir, cfg.AppContainer)
	if err != nil || !running {
		return nil, webapp.Info{}, webapp.ErrNotRunning
	}

	client := webapp.New(composeDir, cfg.AppContainer)
	info, err := client.Info(ctx)
	if err != nil {
		return nil, webapp.Info{}, err
	}
	if err := webapp.RequireAPI(info); err != nil {
		return nil, webapp.Info{}, err
	}
	return client, info, nil
}

// waitForApp polls the doctor until the app is healthy, reporting progress on stdout.
func waitForApp(ctx context.Context, client *webapp.Client) error {
	return client.WaitForReady(ctx, 45, 2*time.Second, func(i, n int) {
		output.Infof("  Not ready yet (%d/%d)...", i, n)
	})
}
