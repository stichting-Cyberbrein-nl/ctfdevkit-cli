// Package webapp talks to the DevKit webapp inside the app container through its
// versioned `php artisan devkit:* --json` contract.
package webapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// APIVersion is the contract version this CLI speaks.
const APIVersion = 1

// ErrUnsupportedPayload means the installed webapp lacks the contract commands.
var ErrUnsupportedPayload = errors.New("the installed DevKit is too old for this CLI — run `devkit update`")

// ErrNotRunning means the app container is not running.
var ErrNotRunning = errors.New("DevKit is not running — run `devkit up` first")

// Runner executes `args` inside the app container and returns stdout and stderr separately.
type Runner func(ctx context.Context, args []string) (stdout, stderr []byte, err error)

// Client calls the webapp contract commands.
type Client struct{ run Runner }

// New returns a client that runs commands with `docker compose exec -T <service>`.
func New(composeDir, service string) *Client {
	return NewWithRunner(func(ctx context.Context, args []string) ([]byte, []byte, error) {
		full := []string{"compose", "-f", filepath.Join(composeDir, "docker-compose.yml"), "exec", "-T", service}
		full = append(full, args...)
		cmd := exec.CommandContext(ctx, "docker", full...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.Bytes(), stderr.Bytes(), err
	})
}

// NewWithRunner returns a client backed by a custom runner (used by tests).
func NewWithRunner(run Runner) *Client { return &Client{run: run} }

// CommandError is returned when a contract command exits non-zero but still printed JSON.
type CommandError struct {
	Args   []string
	Stderr string
}

func (e *CommandError) Error() string {
	msg := fmt.Sprintf("`%s` failed", strings.Join(e.Args, " "))
	if s := strings.TrimSpace(e.Stderr); s != "" {
		msg += ": " + lastLines(s, 20)
	}
	return msg
}

// Info is the output of `devkit:info --json`.
type Info struct {
	APIVersion  int             `json:"api_version"`
	AppVersion  string          `json:"app_version"`
	Brand       string          `json:"brand"`
	Assignments InfoAssignments `json:"assignments"`
	Categories  []string        `json:"categories"`
	Languages   []string        `json:"languages"`
	MCP         InfoMCP         `json:"mcp"`
}

// InfoAssignments describes the assignments folder linked to the DevKit.
type InfoAssignments struct {
	Path     string  `json:"path"`
	HostPath *string `json:"host_path"`
	Total    int     `json:"total"`
	Valid    int     `json:"valid"`
}

// InfoMCP describes the MCP server.
type InfoMCP struct {
	Handle string `json:"handle"`
}

// DoctorReport is the output of `devkit:doctor --json`.
type DoctorReport struct {
	OK         bool          `json:"ok"`
	Strict     bool          `json:"strict"`
	AppVersion string        `json:"app_version"`
	APIVersion int           `json:"api_version"`
	Checks     []DoctorCheck `json:"checks"`
}

// DoctorCheck is a single health check.
type DoctorCheck struct {
	Key      string `json:"key"`
	OK       bool   `json:"ok"`
	Required bool   `json:"required"`
	Message  string `json:"message"`
}

// ValidateReport is the output of `devkit:validate --json`.
type ValidateReport struct {
	OK          bool            `json:"ok"`
	Strict      bool            `json:"strict"`
	Error       *string         `json:"error"`
	Assignments []PreviewResult `json:"assignments"`
}

// PreviewResult mirrors AssignmentPreview::preview().
type PreviewResult struct {
	DirectoryName     string    `json:"directory_name"`
	Valid             bool      `json:"valid"`
	Errors            []string  `json:"errors"`
	Warnings          []string  `json:"warnings"`
	Title             string    `json:"title"`
	Category          string    `json:"category"`
	CategoryIcon      string    `json:"category_icon"`
	DifficultyStars   int       `json:"difficulty_stars"`
	BasePoints        int       `json:"base_points"`
	Description       string    `json:"description"`
	Hints             []string  `json:"hints"`
	PlaceholderAnswer string    `json:"placeholder_answer"`
	FlagExample       string    `json:"flag_example"`
	PHPFiles          []PHPFile `json:"php_files"`
	RevealsFlag       bool      `json:"reveals_flag"`
	StoryStart        string    `json:"story_start"`
}

// PHPFile is the lint result of one PHP file.
type PHPFile struct {
	Path  string  `json:"path"`
	OK    bool    `json:"ok"`
	Error *string `json:"error"`
}

// CheckReport is the output of `devkit:check-assignment --json`.
type CheckReport struct {
	Name     string        `json:"name"`
	OK       bool          `json:"ok"`
	Error    *string       `json:"error,omitempty"`
	Language string        `json:"language"`
	Paths    CheckPaths    `json:"paths"`
	URL      string        `json:"url"`
	Preview  PreviewResult `json:"preview"`
	Style    []StyleIssue  `json:"style"`
	Smoke    SmokeResult   `json:"smoke"`
}

// CheckPaths holds the container and host location of an assignment.
type CheckPaths struct {
	Container string  `json:"container"`
	Host      *string `json:"host"`
}

// StyleIssue is one house-style finding.
type StyleIssue struct {
	Code     string  `json:"code"`
	Severity string  `json:"severity"`
	Message  string  `json:"message"`
	File     *string `json:"file"`
}

// SmokeResult is the runtime smoke test outcome.
type SmokeResult struct {
	Ran                bool     `json:"ran"`
	OK                 bool     `json:"ok"`
	Status             *int     `json:"status"`
	FinalURL           *string  `json:"final_url"`
	Problems           []string `json:"problems"`
	FlagVisibleOnEntry bool     `json:"flag_visible_on_entry"`
}

// ScaffoldInput are the arguments for `devkit:scaffold-assignment`.
type ScaffoldInput struct {
	Title      string
	Language   string
	Category   string
	Difficulty int
	Number     *int
}

// ScaffoldResult is the output of `devkit:scaffold-assignment --json`.
type ScaffoldResult struct {
	Name  string      `json:"name"`
	Error *string     `json:"error,omitempty"`
	Check CheckReport `json:"check"`
}

// Info runs `devkit:info --json`.
func (c *Client) Info(ctx context.Context) (Info, error) {
	var v Info
	err := c.runJSON(ctx, artisan("devkit:info", "--json"), &v)
	return v, err
}

// Doctor runs `devkit:doctor --json [--strict]`.
func (c *Client) Doctor(ctx context.Context, strict bool) (DoctorReport, error) {
	args := artisan("devkit:doctor", "--json")
	if strict {
		args = append(args, "--strict")
	}
	var v DoctorReport
	err := c.runJSON(ctx, args, &v)
	return v, err
}

// Validate runs `devkit:validate [name] --json [--strict]`.
func (c *Client) Validate(ctx context.Context, name string, strict bool) (ValidateReport, error) {
	args := artisan("devkit:validate")
	if name != "" {
		args = append(args, name)
	}
	args = append(args, "--json")
	if strict {
		args = append(args, "--strict")
	}
	var v ValidateReport
	err := c.runJSON(ctx, args, &v)
	if err != nil && v.Error != nil {
		err = errors.New(*v.Error)
	}
	return v, err
}

// Check runs `devkit:check-assignment <name> --json [--no-smoke]`.
func (c *Client) Check(ctx context.Context, name string, smoke bool) (CheckReport, error) {
	args := artisan("devkit:check-assignment", name, "--json")
	if !smoke {
		args = append(args, "--no-smoke")
	}
	var v CheckReport
	err := c.runJSON(ctx, args, &v)
	if err != nil && v.Error != nil {
		err = errors.New(*v.Error)
	}
	return v, err
}

// Scaffold runs `devkit:scaffold-assignment`.
func (c *Client) Scaffold(ctx context.Context, in ScaffoldInput) (ScaffoldResult, error) {
	args := artisan("devkit:scaffold-assignment", in.Title,
		"--language="+in.Language,
		"--category="+in.Category,
		"--difficulty="+strconv.Itoa(in.Difficulty),
	)
	if in.Number != nil {
		args = append(args, "--number="+strconv.Itoa(*in.Number))
	}
	args = append(args, "--json")
	var v ScaffoldResult
	err := c.runJSON(ctx, args, &v)
	if err != nil && v.Error != nil {
		err = errors.New(*v.Error)
	}
	return v, err
}

// WaitForReady polls `devkit:doctor --strict` until it reports ok.
func (c *Client) WaitForReady(ctx context.Context, attempts int, interval time.Duration, progress func(attempt, total int)) error {
	for i := 1; i <= attempts; i++ {
		report, err := c.Doctor(ctx, true)
		if errors.Is(err, ErrUnsupportedPayload) {
			return err
		}
		if err == nil && report.OK {
			return nil
		}
		if progress != nil {
			progress(i, attempts)
		}
		if i == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check cancelled")
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("application did not become healthy after %d attempts — run `devkit logs` to investigate", attempts)
}

// RequireAPI checks that the webapp speaks this CLI's contract version.
func RequireAPI(info Info) error {
	switch {
	case info.APIVersion < APIVersion:
		return ErrUnsupportedPayload
	case info.APIVersion > APIVersion:
		return fmt.Errorf("this DevKit (v%s) needs a newer CLI — run `devkit self-update`", info.AppVersion)
	}
	return nil
}

func artisan(args ...string) []string {
	return append([]string{"php", "artisan"}, args...)
}

func (c *Client) runJSON(ctx context.Context, args []string, v any) error {
	stdout, stderr, runErr := c.run(ctx, args)
	trimmed := bytes.TrimSpace(stdout)

	if bytes.HasPrefix(trimmed, []byte("{")) {
		if err := json.Unmarshal(trimmed, v); err != nil {
			return fmt.Errorf("unexpected output from %s: %s", strings.Join(args, " "), head(trimmed, 200))
		}
		if runErr != nil {
			return &CommandError{Args: args, Stderr: string(stderr)}
		}
		return nil
	}

	combined := string(stdout) + string(stderr)
	if strings.Contains(combined, "is not defined") || strings.Contains(combined, "does not exist.") {
		return ErrUnsupportedPayload
	}
	if strings.Contains(string(stderr), "is not running") {
		return ErrNotRunning
	}
	tail := lastLines(strings.TrimSpace(string(stderr)), 20)
	if runErr != nil {
		if tail == "" {
			return fmt.Errorf("%s failed: %w", strings.Join(args, " "), runErr)
		}
		return fmt.Errorf("%s failed: %w\n%s", strings.Join(args, " "), runErr, tail)
	}
	return fmt.Errorf("unexpected output from %s: %s", strings.Join(args, " "), head(trimmed, 200))
}

func head(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
