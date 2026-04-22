package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/assignments"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/prereqs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
)

type assignmentsPrompter struct {
	askPath    func(string) (string, error)
	askConfirm func(string, string) (bool, error)
}

type assignmentsFlowOps struct {
	suggestedPath func(platform.Platform) (string, error)
	ensureGit     func(context.Context, platform.Platform) error
	inspect       func(context.Context, string, string) (assignments.RepoStatus, error)
	clone         func(context.Context, string, string) error
	fetch         func(context.Context, string) error
	pull          func(context.Context, string, bool) error
}

func defaultAssignmentsPrompter() assignmentsPrompter {
	return assignmentsPrompter{
		askPath:    tui.AskAssignmentsPath,
		askConfirm: tui.AskConfirm,
	}
}

func defaultAssignmentsFlowOps() assignmentsFlowOps {
	return assignmentsFlowOps{
		suggestedPath: assignments.SuggestedPath,
		ensureGit:     prereqs.EnsureGit,
		inspect:       assignments.Inspect,
		clone:         assignments.Clone,
		fetch:         assignments.Fetch,
		pull:          assignments.Pull,
	}
}

func resolveAssignmentsRepoPath(ctx context.Context, cfg config.Config, plat platform.Platform, prompts assignmentsPrompter, ops assignmentsFlowOps) (string, error) {
	repoURL := assignmentsRepoURL(cfg)
	if path := strings.TrimSpace(cfg.AssignmentsPath); path != "" {
		status, err := ops.inspect(ctx, path, repoURL)
		if err != nil {
			return "", err
		}

		switch status.State {
		case assignments.StateReady:
			output.Successf("Assignments repo ready: %s (%d assignments)", path, status.AssignmentCount)
			return path, nil
		case assignments.StateMissing, assignments.StateEmpty:
			return ensureAssignmentsRepoAtPath(ctx, path, repoURL, plat, false, prompts, ops)
		default:
			output.Warnf("Configured assignments path is not usable: %s", path)
		}
	}

	suggested, err := ops.suggestedPath(plat)
	if err != nil {
		return "", err
	}
	path, err := prompts.askPath(suggested)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		path = suggested
	}

	return ensureAssignmentsRepoAtPath(ctx, path, repoURL, plat, true, prompts, ops)
}

func chooseAssignmentsRepoPath(ctx context.Context, cfg config.Config, plat platform.Platform, pathOverride string, prompts assignmentsPrompter, ops assignmentsFlowOps) (string, error) {
	repoURL := assignmentsRepoURL(cfg)
	if strings.TrimSpace(pathOverride) != "" {
		return ensureAssignmentsRepoAtPath(ctx, pathOverride, repoURL, plat, true, prompts, ops)
	}

	suggested := strings.TrimSpace(cfg.AssignmentsPath)
	if suggested == "" {
		var err error
		suggested, err = ops.suggestedPath(plat)
		if err != nil {
			return "", err
		}
	}

	path, err := prompts.askPath(suggested)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		path = suggested
	}

	return ensureAssignmentsRepoAtPath(ctx, path, repoURL, plat, true, prompts, ops)
}

func ensureAssignmentsRepoAtPath(ctx context.Context, path, repoURL string, plat platform.Platform, promptForUpdate bool, prompts assignmentsPrompter, ops assignmentsFlowOps) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("assignments path cannot be empty")
	}

	status, err := ops.inspect(ctx, path, repoURL)
	if err != nil {
		return "", err
	}

	switch status.State {
	case assignments.StateMissing, assignments.StateEmpty:
		if err := ops.ensureGit(ctx, plat); err != nil {
			return "", err
		}
		if err := ops.clone(ctx, repoURL, path); err != nil {
			return "", err
		}

		status, err = ops.inspect(ctx, path, repoURL)
		if err != nil {
			return "", err
		}
		if status.State != assignments.StateReady {
			return "", fmt.Errorf("assignments repo was cloned but is not ready at %s", path)
		}

		output.Successf("Assignments repo ready: %s (%d assignments)", path, status.AssignmentCount)
		return path, nil

	case assignments.StateReady:
		output.Successf("Assignments repo ready: %s (%d assignments)", path, status.AssignmentCount)
		if promptForUpdate && prompts.askConfirm != nil {
			_, updated, err := updateAssignmentsRepoPath(ctx, path, repoURL, plat, prompts, ops)
			if err != nil {
				return "", err
			}
			if updated {
				output.Success("Assignments repo updated")
			}
		}
		return path, nil

	case assignments.StateWrongRepo:
		return "", fmt.Errorf("path %s already contains a different git repository", path)

	default:
		return "", fmt.Errorf("path %s is not empty and is not the assignments repo", path)
	}
}

func updateAssignmentsRepoPath(ctx context.Context, path, repoURL string, plat platform.Platform, prompts assignmentsPrompter, ops assignmentsFlowOps) (assignments.RepoStatus, bool, error) {
	if err := ops.ensureGit(ctx, plat); err != nil {
		return assignments.RepoStatus{}, false, err
	}

	status, err := ops.inspect(ctx, path, repoURL)
	if err != nil {
		return status, false, err
	}
	if status.State != assignments.StateReady {
		return status, false, fmt.Errorf("assignments repo is not ready at %s", path)
	}

	if err := ops.fetch(ctx, path); err != nil {
		return status, false, err
	}

	status, err = ops.inspect(ctx, path, repoURL)
	if err != nil {
		return status, false, err
	}

	allowMerge := status.HasTrackedChanges
	title, subtitle := updateConfirmation(status)
	if prompts.askConfirm != nil {
		confirmed, err := prompts.askConfirm(title, subtitle)
		if err != nil {
			return status, false, err
		}
		if !confirmed {
			return status, false, nil
		}
	}

	if err := ops.pull(ctx, path, allowMerge); err != nil {
		return status, false, err
	}

	refreshed, err := ops.inspect(ctx, path, repoURL)
	if err == nil {
		status = refreshed
	}
	return status, true, nil
}

func assignmentsRepoURL(cfg config.Config) string {
	if strings.TrimSpace(cfg.AssignmentsRepoURL) != "" {
		return cfg.AssignmentsRepoURL
	}
	return config.Default().AssignmentsRepoURL
}

func updateConfirmation(status assignments.RepoStatus) (string, string) {
	switch {
	case status.HasTrackedChanges:
		return "Update assignments repo?", "Tracked files were changed locally. Pulling can create merge conflicts."
	case status.HasUntrackedChanges:
		return "Update assignments repo?", "Untracked assignment folders will be kept. Continue with the pull?"
	default:
		return "Update assignments repo?", "Pull the latest public assignments into this local repo?"
	}
}
