package ports_test

import (
	"net"
	"testing"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ports"
)

func TestIsInUseFreePort(t *testing.T) {
	// Find a free port.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // Release it.

	// Should report as free immediately after close.
	if ports.IsInUse(port) {
		t.Errorf("port %d should be free", port)
	}
}

func TestIsInUseBoundPort(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	if !ports.IsInUse(port) {
		t.Errorf("port %d should be reported as in use", port)
	}
}
