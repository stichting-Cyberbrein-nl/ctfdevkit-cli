package config

import (
	"os"
	"testing"
)

func TestDefaultIncludesAssignmentsRepoURL(t *testing.T) {
	cfg := Default()

	if got, want := cfg.AssignmentsRepoURL, "https://github.com/stichting-Cyberbrein-nl/assignments"; got != want {
		t.Fatalf("Default().AssignmentsRepoURL = %q, want %q", got, want)
	}
}

func TestApplyEnvOverridesAssignmentsRepoURL(t *testing.T) {
	t.Setenv("DEVKIT_ASSIGNMENTS_REPO_URL", "https://github.com/example/custom-assignments")
	os.Unsetenv("DEVKIT_ASSIGNMENTS_PATH")

	cfg := applyEnv(Default())

	if got, want := cfg.AssignmentsRepoURL, "https://github.com/example/custom-assignments"; got != want {
		t.Fatalf("applyEnv(Default()).AssignmentsRepoURL = %q, want %q", got, want)
	}
}
