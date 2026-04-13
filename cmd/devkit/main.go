package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/cli"
)

// version is injected at build time:
// go build -ldflags "-X main.version=1.2.3" ./cmd/devkit
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cli.Execute(ctx, version)
}
