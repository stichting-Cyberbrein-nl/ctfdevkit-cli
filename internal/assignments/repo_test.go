package assignments

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

func TestSuggestedPathFromHomeDirectory(t *testing.T) {
	tests := []struct {
		name string
		plat platform.Platform
		want string
	}{
		{
			name: "linux",
			plat: platform.Platform{OS: platform.OSLinux},
			want: filepath.Join("/home/alice", "ctfdevkit", "assignments"),
		},
		{
			name: "windows",
			plat: platform.Platform{OS: platform.OSWindows},
			want: filepath.Join(`C:\Users\Alice`, "ctfdevkit", "assignments"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggestedPath(tt.plat, func() (string, error) {
				switch tt.plat.OS {
				case platform.OSWindows:
					return `C:\Users\Alice`, nil
				default:
					return "/home/alice", nil
				}
			})
			if got != tt.want {
				t.Fatalf("suggestedPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchesRepoURLAcceptsHTTPSAndSSHForms(t *testing.T) {
	expected := "https://github.com/stichting-Cyberbrein-nl/assignments"

	for _, actual := range []string{
		"https://github.com/stichting-Cyberbrein-nl/assignments",
		"https://github.com/stichting-Cyberbrein-nl/assignments.git",
		"git@github.com:stichting-Cyberbrein-nl/assignments.git",
		"ssh://git@github.com/stichting-Cyberbrein-nl/assignments.git",
	} {
		if !matchesRepoURL(actual, expected) {
			t.Fatalf("matchesRepoURL(%q, %q) = false, want true", actual, expected)
		}
	}

	if matchesRepoURL("https://github.com/example/other", expected) {
		t.Fatal("matchesRepoURL() = true for wrong remote, want false")
	}
}

func TestInspectClassifiesWrongRepoAndChangeTypes(t *testing.T) {
	ctx := context.Background()

	t.Run("wrong repo", func(t *testing.T) {
		path := createGitRepo(t, "https://github.com/example/other")
		status, err := Inspect(ctx, path, "https://github.com/stichting-Cyberbrein-nl/assignments")
		if err != nil {
			t.Fatalf("Inspect() error = %v", err)
		}
		if status.State != StateWrongRepo {
			t.Fatalf("Inspect().State = %q, want %q", status.State, StateWrongRepo)
		}
	})

	t.Run("untracked only", func(t *testing.T) {
		path := createGitRepo(t, "https://github.com/stichting-Cyberbrein-nl/assignments")
		mkdirAll(t, filepath.Join(path, "custom-assignment"))
		writeFile(t, filepath.Join(path, "custom-assignment", "config.json"), []byte(`{"name":"custom"}`))

		status, err := Inspect(ctx, path, "https://github.com/stichting-Cyberbrein-nl/assignments")
		if err != nil {
			t.Fatalf("Inspect() error = %v", err)
		}
		if status.State != StateReady {
			t.Fatalf("Inspect().State = %q, want %q", status.State, StateReady)
		}
		if !status.HasUntrackedChanges {
			t.Fatal("Inspect().HasUntrackedChanges = false, want true")
		}
		if status.HasTrackedChanges {
			t.Fatal("Inspect().HasTrackedChanges = true, want false")
		}
		if status.AssignmentCount != 1 {
			t.Fatalf("Inspect().AssignmentCount = %d, want 1", status.AssignmentCount)
		}
	})

	t.Run("tracked changes", func(t *testing.T) {
		path := createGitRepo(t, "https://github.com/stichting-Cyberbrein-nl/assignments")
		writeFile(t, filepath.Join(path, "README.md"), []byte("changed\n"))

		status, err := Inspect(ctx, path, "https://github.com/stichting-Cyberbrein-nl/assignments")
		if err != nil {
			t.Fatalf("Inspect() error = %v", err)
		}
		if status.State != StateReady {
			t.Fatalf("Inspect().State = %q, want %q", status.State, StateReady)
		}
		if !status.HasTrackedChanges {
			t.Fatal("Inspect().HasTrackedChanges = false, want true")
		}
	})
}

func createGitRepo(t *testing.T, remote string) string {
	t.Helper()

	path := t.TempDir()
	runGit(t, path, "init")
	runGit(t, path, "config", "user.name", "DevKit Test")
	runGit(t, path, "config", "user.email", "devkit@example.com")

	writeFile(t, filepath.Join(path, "README.md"), []byte("hello\n"))
	runGit(t, path, "add", "README.md")
	runGit(t, path, "commit", "-m", "initial")
	runGit(t, path, "branch", "-M", "main")
	runGit(t, path, "remote", "add", "origin", remote)

	return path
}

func runGit(t *testing.T, path string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
