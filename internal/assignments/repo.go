package assignments

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

type RepoState string

const (
	StateMissing   RepoState = "missing"
	StateEmpty     RepoState = "empty"
	StateReady     RepoState = "ready"
	StateWrongRepo RepoState = "wrong_repo"
	StateInvalid   RepoState = "invalid"
)

type RepoStatus struct {
	State               RepoState
	Path                string
	RemoteURL           string
	Branch              string
	LastCommit          string
	AssignmentCount     int
	AheadCount          int
	BehindCount         int
	MatchesRemote       bool
	HasUntrackedChanges bool
	HasTrackedChanges   bool
}

func SuggestedPath(plat platform.Platform) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return suggestedPath(plat, func() (string, error) { return home, nil }), nil
}

func suggestedPath(_ platform.Platform, homeDir func() (string, error)) string {
	home, err := homeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "ctfdevkit", "assignments")
}

func Inspect(ctx context.Context, path, repoURL string) (RepoStatus, error) {
	status := RepoStatus{
		Path:            path,
		AssignmentCount: Count(path),
	}
	if path == "" {
		status.State = StateMissing
		return status, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			status.State = StateMissing
			return status, nil
		}
		return status, err
	}
	if !info.IsDir() {
		status.State = StateInvalid
		return status, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return status, err
	}
	if len(entries) == 0 {
		status.State = StateEmpty
		return status, nil
	}

	remoteURL, err := gitOutput(ctx, path, "remote", "get-url", "origin")
	if err != nil {
		status.State = StateInvalid
		return status, nil
	}
	status.RemoteURL = remoteURL
	status.MatchesRemote = matchesRepoURL(remoteURL, repoURL)
	if !status.MatchesRemote {
		status.State = StateWrongRepo
		return status, nil
	}

	status.LastCommit, _ = gitOutput(ctx, path, "log", "-1", "--pretty=format:%h %s")

	lines, err := gitLines(ctx, path, "status", "--porcelain", "--branch")
	if err != nil {
		return status, err
	}
	if len(lines) > 0 && strings.HasPrefix(lines[0], "## ") {
		status.Branch, status.AheadCount, status.BehindCount = parseBranchStatusLine(lines[0])
		lines = lines[1:]
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "?? ") {
			status.HasUntrackedChanges = true
			continue
		}
		if strings.TrimSpace(line) != "" {
			status.HasTrackedChanges = true
		}
	}

	status.AssignmentCount = Count(path)
	status.State = StateReady
	return status, nil
}

func Clone(ctx context.Context, repoURL, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating assignments parent directory: %w", err)
	}
	return gitRun(ctx, "", "clone", repoURL, path)
}

func Fetch(ctx context.Context, path string) error {
	return gitRun(ctx, path, "fetch", "--prune", "origin")
}

func Pull(ctx context.Context, path string, allowMerge bool) error {
	args := []string{"pull", "--prune"}
	if !allowMerge {
		args = append(args, "--ff-only")
	}
	return gitRun(ctx, path, args...)
}

func matchesRepoURL(actual, expected string) bool {
	return normalizeRepoURL(actual) == normalizeRepoURL(expected)
}

func normalizeRepoURL(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = strings.TrimSuffix(normalized, "/")
	normalized = strings.TrimSuffix(normalized, ".git")

	if strings.HasPrefix(normalized, "git@") {
		trimmed := strings.TrimPrefix(normalized, "git@")
		hostAndPath := strings.SplitN(trimmed, ":", 2)
		if len(hostAndPath) == 2 {
			return strings.ToLower(hostAndPath[0] + "/" + hostAndPath[1])
		}
	}

	if u, err := url.Parse(normalized); err == nil && u.Host != "" {
		return strings.ToLower(strings.Trim(u.Host+"/"+strings.Trim(u.Path, "/"), "/"))
	}

	return strings.ToLower(strings.Trim(normalized, "/"))
}

func gitRun(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", gitArgs(dir, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", gitArgs(dir, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func gitLines(ctx context.Context, dir string, args ...string) ([]string, error) {
	out, err := gitOutput(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func gitArgs(dir string, args ...string) []string {
	if dir == "" {
		return args
	}
	return append([]string{"-C", dir}, args...)
}

func parseBranchStatusLine(line string) (string, int, int) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(line, "## "))
	branch := trimmed
	if idx := strings.Index(branch, "..."); idx >= 0 {
		branch = branch[:idx]
	}

	ahead := 0
	behind := 0
	if start := strings.Index(trimmed, "["); start >= 0 {
		branch = strings.TrimSpace(branch)
		for _, part := range strings.Split(strings.Trim(trimmed[start:], "[]"), ",") {
			part = strings.TrimSpace(part)
			switch {
			case strings.HasPrefix(part, "ahead "):
				fmt.Sscanf(strings.TrimPrefix(part, "ahead "), "%d", &ahead)
			case strings.HasPrefix(part, "behind "):
				fmt.Sscanf(strings.TrimPrefix(part, "behind "), "%d", &behind)
			}
		}
	}

	return strings.TrimSpace(branch), ahead, behind
}
