package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/assignments"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

func TestEnsureAssignmentsRepoAtPathClonesMissingDirectory(t *testing.T) {
	cloned := false
	ensuredGit := false

	statuses := []assignments.RepoStatus{
		{State: assignments.StateMissing},
		{State: assignments.StateReady, Path: "/tmp/assignments"},
	}
	index := 0

	path, err := ensureAssignmentsRepoAtPath(
		context.Background(),
		"/tmp/assignments",
		config.Default().AssignmentsRepoURL,
		platform.Platform{OS: platform.OSLinux},
		false,
		assignmentsPrompter{},
		assignmentsFlowOps{
			ensureGit: func(context.Context, platform.Platform) error {
				ensuredGit = true
				return nil
			},
			inspect: func(context.Context, string, string) (assignments.RepoStatus, error) {
				status := statuses[index]
				index++
				return status, nil
			},
			clone: func(context.Context, string, string) error {
				cloned = true
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ensureAssignmentsRepoAtPath() error = %v", err)
	}
	if path != "/tmp/assignments" {
		t.Fatalf("ensureAssignmentsRepoAtPath() path = %q, want /tmp/assignments", path)
	}
	if !ensuredGit {
		t.Fatal("ensureAssignmentsRepoAtPath() did not ensure git")
	}
	if !cloned {
		t.Fatal("ensureAssignmentsRepoAtPath() did not clone missing repo")
	}
}

func TestEnsureAssignmentsRepoAtPathRejectsWrongRepo(t *testing.T) {
	_, err := ensureAssignmentsRepoAtPath(
		context.Background(),
		"/tmp/assignments",
		config.Default().AssignmentsRepoURL,
		platform.Platform{OS: platform.OSLinux},
		false,
		assignmentsPrompter{},
		assignmentsFlowOps{
			inspect: func(context.Context, string, string) (assignments.RepoStatus, error) {
				return assignments.RepoStatus{State: assignments.StateWrongRepo}, nil
			},
		},
	)
	if err == nil {
		t.Fatal("ensureAssignmentsRepoAtPath() error = nil, want wrong repo error")
	}
}

func TestUpdateAssignmentsRepoPathUsesMergeModeForTrackedChanges(t *testing.T) {
	confirmed := false
	allowMerge := false
	fetched := false

	statuses := []assignments.RepoStatus{
		{State: assignments.StateReady, HasTrackedChanges: true},
		{State: assignments.StateReady, HasTrackedChanges: true},
		{State: assignments.StateReady, HasTrackedChanges: true},
	}
	index := 0

	_, updated, err := updateAssignmentsRepoPath(
		context.Background(),
		"/tmp/assignments",
		config.Default().AssignmentsRepoURL,
		platform.Platform{OS: platform.OSLinux},
		assignmentsPrompter{
			askConfirm: func(string, string) (bool, error) {
				confirmed = true
				return true, nil
			},
		},
		assignmentsFlowOps{
			ensureGit: func(context.Context, platform.Platform) error { return nil },
			inspect: func(context.Context, string, string) (assignments.RepoStatus, error) {
				status := statuses[index]
				index++
				return status, nil
			},
			fetch: func(context.Context, string) error {
				fetched = true
				return nil
			},
			pull: func(_ context.Context, _ string, merge bool) error {
				allowMerge = merge
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("updateAssignmentsRepoPath() error = %v", err)
	}
	if !updated {
		t.Fatal("updateAssignmentsRepoPath() updated = false, want true")
	}
	if !fetched {
		t.Fatal("updateAssignmentsRepoPath() did not fetch before pull")
	}
	if !confirmed {
		t.Fatal("updateAssignmentsRepoPath() did not ask for confirmation")
	}
	if !allowMerge {
		t.Fatal("updateAssignmentsRepoPath() allowMerge = false, want true")
	}
}

func TestResolveAssignmentsRepoPathFallsBackToSuggestedPath(t *testing.T) {
	statuses := []assignments.RepoStatus{
		{State: assignments.StateMissing},
		{State: assignments.StateReady, Path: "/home/alice/ctfdevkit/assignments"},
	}
	index := 0

	path, err := resolveAssignmentsRepoPath(
		context.Background(),
		config.Config{AssignmentsRepoURL: config.Default().AssignmentsRepoURL},
		platform.Platform{OS: platform.OSLinux},
		assignmentsPrompter{
			askPath: func(string) (string, error) {
				return "", nil
			},
		},
		assignmentsFlowOps{
			suggestedPath: func(platform.Platform) (string, error) {
				return "/home/alice/ctfdevkit/assignments", nil
			},
			ensureGit: func(context.Context, platform.Platform) error { return nil },
			inspect: func(context.Context, string, string) (assignments.RepoStatus, error) {
				status := statuses[index]
				index++
				return status, nil
			},
			clone: func(context.Context, string, string) error { return nil },
		},
	)
	if err != nil {
		t.Fatalf("resolveAssignmentsRepoPath() error = %v", err)
	}
	if path != "/home/alice/ctfdevkit/assignments" {
		t.Fatalf("resolveAssignmentsRepoPath() = %q, want suggested path", path)
	}
}

func TestUpdateAssignmentsRepoPathStopsWhenConfirmationFails(t *testing.T) {
	_, updated, err := updateAssignmentsRepoPath(
		context.Background(),
		"/tmp/assignments",
		config.Default().AssignmentsRepoURL,
		platform.Platform{OS: platform.OSLinux},
		assignmentsPrompter{
			askConfirm: func(string, string) (bool, error) {
				return false, nil
			},
		},
		assignmentsFlowOps{
			ensureGit: func(context.Context, platform.Platform) error { return nil },
			inspect: func(context.Context, string, string) (assignments.RepoStatus, error) {
				return assignments.RepoStatus{State: assignments.StateReady, HasTrackedChanges: true}, nil
			},
			fetch: func(context.Context, string) error { return nil },
			pull: func(context.Context, string, bool) error {
				return errors.New("should not pull")
			},
		},
	)
	if err != nil {
		t.Fatalf("updateAssignmentsRepoPath() error = %v", err)
	}
	if updated {
		t.Fatal("updateAssignmentsRepoPath() updated = true, want false")
	}
}
