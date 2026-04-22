package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/assignments"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

func newAssignmentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assignments",
		Short: "Manage the assignments repository",
	}

	cmd.AddCommand(
		newAssignmentsStatusCmd(),
		newAssignmentsUpdateCmd(),
		newAssignmentsSetPathCmd(),
	)

	return cmd
}

func newAssignmentsStatusCmd() *cobra.Command {
	var repoURL string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the assignments repository status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)
			if repoURL != "" {
				cfg.AssignmentsRepoURL = repoURL
			}

			output.Header("Assignments Repo")
			if strings.TrimSpace(cfg.AssignmentsPath) == "" {
				suggested, err := assignments.SuggestedPath(plat)
				if err != nil {
					return err
				}
				output.Warn("Assignments path is not configured.")
				output.Hintf("Run `devkit assignments set-path` to clone the repo into %s.", suggested)
				return nil
			}

			status, err := assignments.Inspect(ctx, cfg.AssignmentsPath, assignmentsRepoURL(cfg))
			if err != nil {
				return err
			}

			output.Plainf("Path: %s", cfg.AssignmentsPath)
			if status.RemoteURL != "" {
				output.Plainf("Remote: %s", status.RemoteURL)
			}
			if status.Branch != "" {
				output.Plainf("Branch: %s", status.Branch)
			}
			if status.LastCommit != "" {
				output.Plainf("Last commit: %s", status.LastCommit)
			}
			output.Plainf("Assignments: %d", status.AssignmentCount)
			if status.Branch != "" {
				output.Plainf("Ahead/behind: %d/%d", status.AheadCount, status.BehindCount)
			}
			if status.HasUntrackedChanges {
				output.Warn("Local untracked assignment folders/files detected.")
			}
			if status.HasTrackedChanges {
				output.Warn("Local tracked file changes detected.")
			}

			switch status.State {
			case assignments.StateReady:
				output.Success("Assignments repo is ready")
			case assignments.StateMissing:
				output.Warn("Assignments path does not exist yet.")
			case assignments.StateEmpty:
				output.Warn("Assignments path is empty and still needs the repo clone.")
			case assignments.StateWrongRepo:
				output.Fail("Configured path points to a different Git repository.")
			default:
				output.Fail("Configured path is not a usable assignments repo.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo-url", "", "Assignments repository URL override")
	return cmd
}

func newAssignmentsUpdateCmd() *cobra.Command {
	var repoURL string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Pull the latest public assignments into the local repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)
			if repoURL != "" {
				cfg.AssignmentsRepoURL = repoURL
			}
			if strings.TrimSpace(cfg.AssignmentsPath) == "" {
				return fmt.Errorf("assignments path is not configured - run `devkit assignments set-path` first")
			}

			status, updated, err := updateAssignmentsRepoPath(ctx, cfg.AssignmentsPath, assignmentsRepoURL(cfg), plat, defaultAssignmentsPrompter(), defaultAssignmentsFlowOps())
			if err != nil {
				return err
			}
			if !updated {
				output.Info("No changes made.")
				return nil
			}

			output.Successf("Assignments repo updated: %s", cfg.AssignmentsPath)
			output.Hintf("Assignments available: %d", status.AssignmentCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo-url", "", "Assignments repository URL override")
	return cmd
}

func newAssignmentsSetPathCmd() *cobra.Command {
	var pathOverride string
	var repoURL string

	cmd := &cobra.Command{
		Use:   "set-path",
		Short: "Choose where DevKit clones and updates the assignments repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)
			if repoURL != "" {
				cfg.AssignmentsRepoURL = repoURL
			}

			path, err := chooseAssignmentsRepoPath(ctx, cfg, plat, pathOverride, defaultAssignmentsPrompter(), defaultAssignmentsFlowOps())
			if err != nil {
				return err
			}

			cfg.AssignmentsPath = path
			if err := config.Save(cfg); err != nil {
				return err
			}

			output.Successf("Assignments path saved: %s", path)
			output.Hint("Run `devkit up` to restart containers with the new path.")
			return nil
		},
	}

	cmd.Flags().StringVar(&pathOverride, "path", "", "Assignments repository path override")
	cmd.Flags().StringVar(&repoURL, "repo-url", "", "Assignments repository URL override")
	return cmd
}
