package ports

import (
	"context"
	"os"
	"testing"
	"time"
)

// A privileged port must never trigger a kill when we are not root: on Debian that path
// escalated with an interactive sudo and setup died with "signal: killed". It must return
// promptly so the caller can fall back to an alternative port.
func TestForceReleaseUnixSkipsPrivilegedPortWhenNotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; the guard intentionally does not apply")
	}

	done := make(chan error, 1)
	go func() { done <- forceReleaseUnix(context.Background(), 443) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("forceReleaseUnix blocked on a privileged port instead of returning")
	}
}

// signalPID must refuse init and this process itself.
func TestSignalPIDRefusesSelfAndInit(t *testing.T) {
	signalPID(context.Background(), 1, "-TERM")
	signalPID(context.Background(), os.Getpid(), "-KILL")
}
