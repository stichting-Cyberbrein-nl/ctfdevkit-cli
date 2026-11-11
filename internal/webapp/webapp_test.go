package webapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const infoJSON = `{
    "api_version": 1,
    "app_version": "1.3.0",
    "brand": "Cyberbrein",
    "assignments": {"path": "/var/www/html/assignments", "host_path": "C:\\Users\\x\\ctfdevkit\\assignments", "total": 34, "valid": 34},
    "categories": ["Awareness", "Cross-site scripting"],
    "languages": ["nl", "en"],
    "mcp": {"handle": "ctfdevkit"}
}`

const doctorJSON = `{"ok": true, "strict": true, "app_version": "1.3.0", "api_version": 1,
 "checks": [{"key": "database", "ok": true, "required": true, "message": "Connection ok"},
            {"key": "http", "ok": true, "required": false, "message": "Skipped (DEVKIT_INTERNAL_URL not set)"}]}`

const previewJSON = `{"directory_name": "[9] Cookie uitlezen", "valid": true, "errors": [], "warnings": ["w"],
 "title": "Cookie uitlezen", "category": "Onbeveiligde gegevens", "category_icon": "🔓", "difficulty_stars": 1,
 "base_points": 100, "description": "d", "hints": ["h1"], "placeholder_answer": "PLACEHOLDER_ANSWER",
 "flag_example": "FLAG{0123456789ABCDEF}", "php_files": [{"path": "wwwroot/index.php", "ok": true, "error": null}],
 "reveals_flag": true, "story_start": "s"}`

const checkJSON = `{
  "name": "[9] Cookie uitlezen",
  "ok": true,
  "language": "nl",
  "paths": {"container": "/var/www/html/assignments/[9] Cookie uitlezen", "host": "C:\\x\\[9] Cookie uitlezen"},
  "url": "https://ctf.dev/assignments/%5B9%5D%20Cookie%20uitlezen/join",
  "preview": ` + previewJSON + `,
  "style": [{"code": "S014", "severity": "error", "message": "m", "file": null}],
  "smoke": {"ran": true, "ok": true, "status": 200, "final_url": "http://127.0.0.1:8080/x", "problems": [], "flag_visible_on_entry": false}
}`

func fixed(stdout, stderr string, err error) Runner {
	return func(context.Context, []string) ([]byte, []byte, error) {
		return []byte(stdout), []byte(stderr), err
	}
}

func TestDecodeContract(t *testing.T) {
	ctx := context.Background()

	info, err := NewWithRunner(fixed(infoJSON, "", nil)).Info(ctx)
	if err != nil || info.APIVersion != 1 || info.MCP.Handle != "ctfdevkit" || info.Assignments.HostPath == nil ||
		*info.Assignments.HostPath != `C:\Users\x\ctfdevkit\assignments` || len(info.Languages) != 2 {
		t.Fatalf("Info = %+v, %v", info, err)
	}

	doc, err := NewWithRunner(fixed(doctorJSON, "", nil)).Doctor(ctx, true)
	if err != nil || !doc.OK || len(doc.Checks) != 2 || doc.Checks[0].Key != "database" {
		t.Fatalf("Doctor = %+v, %v", doc, err)
	}

	chk, err := NewWithRunner(fixed(checkJSON, "", nil)).Check(ctx, "[9] Cookie uitlezen", true)
	if err != nil || !chk.OK || chk.Style[0].Code != "S014" || chk.Smoke.Status == nil || *chk.Smoke.Status != 200 ||
		chk.Paths.Host == nil || !chk.Preview.Valid || chk.Preview.PHPFiles[0].Error != nil {
		t.Fatalf("Check = %+v, %v", chk, err)
	}

	val, err := NewWithRunner(fixed(`{"ok": true, "strict": false, "error": null, "assignments": [`+previewJSON+`]}`, "", nil)).Validate(ctx, "", false)
	if err != nil || !val.OK || val.Error != nil || len(val.Assignments) != 1 || val.Assignments[0].BasePoints != 100 {
		t.Fatalf("Validate = %+v, %v", val, err)
	}
}

func TestFailureWithJSON(t *testing.T) {
	r := fixed(`{"ok": false, "error": "Invalid assignment name \"../x\".", "assignments": []}`, "", errors.New("exit status 1"))
	_, err := NewWithRunner(r).Validate(context.Background(), "../x", false)
	if err == nil || !strings.Contains(err.Error(), "Invalid assignment name") {
		t.Fatalf("err = %v", err)
	}

	_, err = NewWithRunner(fixed(`{"ok": false, "strict": true, "checks": []}`, "boom", errors.New("exit 1"))).Doctor(context.Background(), true)
	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want CommandError", err)
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		stderr string
		want   error
	}{
		{"not defined", "", "Command \"devkit:info\" is not defined.", ErrUnsupportedPayload},
		{"not running", "", `service "app" is not running`, ErrNotRunning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWithRunner(fixed(tt.stdout, tt.stderr, errors.New("exit 1"))).Info(context.Background())
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestWaitForReady(t *testing.T) {
	calls := 0
	oks := []bool{false, false, true}
	r := func(context.Context, []string) ([]byte, []byte, error) {
		ok := oks[calls]
		calls++
		if ok {
			return []byte(`{"ok": true, "checks": []}`), nil, nil
		}
		return []byte(`{"ok": false, "checks": []}`), nil, errors.New("exit 1")
	}
	if err := NewWithRunner(r).WaitForReady(context.Background(), 5, time.Millisecond, nil); err != nil || calls != 3 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}

	calls = 0
	unsupported := func(context.Context, []string) ([]byte, []byte, error) {
		calls++
		return nil, []byte(`Command "devkit:doctor" is not defined.`), errors.New("exit 1")
	}
	err := NewWithRunner(unsupported).WaitForReady(context.Background(), 5, time.Millisecond, nil)
	if !errors.Is(err, ErrUnsupportedPayload) || calls != 1 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}
}

func TestRequireAPI(t *testing.T) {
	if err := RequireAPI(Info{APIVersion: 0}); !errors.Is(err, ErrUnsupportedPayload) {
		t.Fatalf("v0: %v", err)
	}
	if err := RequireAPI(Info{APIVersion: 2, AppVersion: "2.0.0"}); err == nil || !strings.Contains(err.Error(), "needs a newer CLI") {
		t.Fatalf("v2: %v", err)
	}
	if err := RequireAPI(Info{APIVersion: 1}); err != nil {
		t.Fatalf("v1: %v", err)
	}
}
