package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

var difficultyOptions = []string{
	"1 – easy (100 points)",
	"2 – moderate (200 points)",
	"3 – medium (300 points)",
	"4 – hard (400 points)",
	"5 – very hard (500 points)",
}

func newAssignmentsNewCmd() *cobra.Command {
	var language, category string
	var difficulty, number int

	cmd := &cobra.Command{
		Use:   "new [title]",
		Short: "Create a new assignment in the house style",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, info, err := requireWebapp(ctx)
			if err != nil {
				return err
			}
			interactive := isInteractive()

			title := ""
			if len(args) == 1 {
				title = strings.TrimSpace(args[0])
			}
			if title == "" {
				if !interactive {
					return errors.New("missing title")
				}
				if title, err = tui.AskText("New assignment", "Title of the assignment (3–60 characters)", "Cookie uitlezen"); err != nil {
					return err
				}
			}

			if language == "" {
				if !interactive {
					return errors.New("missing --language (nl|en)")
				}
				i, err := tui.AskChoice("Content language", "Which language is the assignment written in?", []string{"Dutch (nl)", "English (en)"}, 0)
				if err != nil {
					return err
				}
				language = []string{"nl", "en"}[i]
			}

			if category == "" {
				if !interactive {
					return fmt.Errorf("missing --category (%s)", strings.Join(info.Categories, "|"))
				}
				i, err := tui.AskChoice("Category", "Which platform category fits the assignment?", info.Categories, 0)
				if err != nil {
					return err
				}
				category = info.Categories[i]
			}

			if !cmd.Flags().Changed("difficulty") {
				if !interactive {
					return errors.New("missing --difficulty (1-5)")
				}
				i, err := tui.AskChoice("Difficulty", "How hard is the assignment?", difficultyOptions, 0)
				if err != nil {
					return err
				}
				difficulty = i + 1
			}

			in := webapp.ScaffoldInput{Title: title, Language: language, Category: category, Difficulty: difficulty}
			if cmd.Flags().Changed("number") {
				in.Number = &number
			}

			output.Infof("Creating assignment %q...", title)
			res, err := client.Scaffold(ctx, in)
			if err != nil {
				return err
			}

			output.Successf("Created %s", res.Name)
			renderCheck(res.Check)
			if res.Check.Paths.Host != nil {
				output.Plainf("Folder: %s", *res.Check.Paths.Host)
			} else {
				output.Plainf("Folder: %s (inside the container)", res.Check.Paths.Container)
			}
			output.Plainf("Open: %s", res.Check.URL)
			output.Hint("Let your AI tool build it: devkit mcp install")
			return nil
		},
	}

	cmd.Flags().StringVar(&language, "language", "", "Content language (nl|en)")
	cmd.Flags().StringVar(&category, "category", "", "Platform category")
	cmd.Flags().IntVar(&difficulty, "difficulty", 1, "Difficulty 1–5")
	cmd.Flags().IntVar(&number, "number", 0, "Assignment number (default: next free number)")
	return cmd
}

func newAssignmentsValidateCmd() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate [name]",
		Short: "Validate assignments against the platform rules",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, _, err := requireWebapp(ctx)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}

			report, err := client.Validate(ctx, name, strict)
			var cmdErr *webapp.CommandError
			if err != nil && !errors.As(err, &cmdErr) {
				return err
			}
			for _, p := range report.Assignments {
				renderPreview(p)
			}
			if !report.OK {
				return errors.New("validation failed")
			}
			output.Successf("%d assignment(s) valid", len(report.Assignments))
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as failures")
	return cmd
}

func newAssignmentsCheckCmd() *cobra.Command {
	var noSmoke bool

	cmd := &cobra.Command{
		Use:   "check <name>",
		Short: "Check one assignment: platform rules, house style and a smoke test",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, _, err := requireWebapp(ctx)
			if err != nil {
				return err
			}

			report, err := client.Check(ctx, args[0], !noSmoke)
			var cmdErr *webapp.CommandError
			if err != nil && !errors.As(err, &cmdErr) {
				return err
			}
			renderCheck(report)
			if !report.OK {
				return fmt.Errorf("%s is not ready yet", args[0])
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&noSmoke, "no-smoke", false, "Skip the runtime smoke test")
	return cmd
}
