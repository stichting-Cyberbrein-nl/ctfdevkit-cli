package prereqs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

func TestEnsureGitInstallsWhenMissing(t *testing.T) {
	originalLookPath := gitLookPath
	originalInstall := gitInstaller
	t.Cleanup(func() {
		gitLookPath = originalLookPath
		gitInstaller = originalInstall
	})

	lookups := 0
	gitLookPath = func(file string) (string, error) {
		lookups++
		if lookups == 1 {
			return "", errors.New("missing")
		}
		return "/usr/bin/git", nil
	}

	installed := false
	gitInstaller = func(context.Context, platform.Platform) error {
		installed = true
		return nil
	}

	if err := EnsureGit(context.Background(), platform.Platform{OS: platform.OSLinux}); err != nil {
		t.Fatalf("EnsureGit() error = %v", err)
	}
	if !installed {
		t.Fatal("EnsureGit() did not attempt installation")
	}
}

func TestEnsureGitReturnsManualInstallHintWhenInstallFails(t *testing.T) {
	originalLookPath := gitLookPath
	originalInstall := gitInstaller
	t.Cleanup(func() {
		gitLookPath = originalLookPath
		gitInstaller = originalInstall
	})

	gitLookPath = func(string) (string, error) {
		return "", errors.New("missing")
	}
	gitInstaller = func(context.Context, platform.Platform) error {
		return errors.New("winget failed")
	}

	err := EnsureGit(context.Background(), platform.Platform{OS: platform.OSWindows})
	if err == nil {
		t.Fatal("EnsureGit() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "git-scm.com") {
		t.Fatalf("EnsureGit() error = %q, want manual install hint", err)
	}
}
