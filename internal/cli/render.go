package cli

import (
	"fmt"
	"strings"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

// renderDoctor prints a doctor report: ✓ ok, ✗ failed and required, → skipped/optional failure.
func renderDoctor(r webapp.DoctorReport) {
	output.Plainf("DevKit v%s · API v%d", r.AppVersion, r.APIVersion)
	for _, c := range r.Checks {
		line := fmt.Sprintf("%s: %s", c.Key, c.Message)
		switch {
		case c.OK:
			output.Success(line)
		case c.Required:
			output.Fail(line)
		default:
			output.Hint(line)
		}
	}
}

// renderPreview prints one assignment's platform preview.
func renderPreview(p webapp.PreviewResult) {
	if p.Valid {
		output.Success(p.DirectoryName)
	} else {
		output.Fail(p.DirectoryName)
	}
	stars := max(0, min(5, p.DifficultyStars))
	output.Plainf("    %s · %s%s · %d points", p.Category, strings.Repeat("★", stars), strings.Repeat("☆", 5-stars), p.BasePoints)
	for _, e := range p.Errors {
		output.Fail(e)
	}
	for _, w := range p.Warnings {
		output.Warn(w)
	}
}

// renderCheck prints the preview, the house-style issues and the smoke test result.
func renderCheck(c webapp.CheckReport) {
	renderPreview(c.Preview)
	for _, sev := range []string{"error", "warning", "info"} {
		for _, s := range c.Style {
			if s.Severity != sev {
				continue
			}
			line := fmt.Sprintf("[%s] %s", s.Code, s.Message)
			switch sev {
			case "error":
				output.Fail(line)
			case "warning":
				output.Warn(line)
			default:
				output.Hint(line)
			}
		}
	}
	switch {
	case !c.Smoke.Ran:
		output.Hint("Smoke: skipped")
	case c.Smoke.OK:
		status := 0
		if c.Smoke.Status != nil {
			status = *c.Smoke.Status
		}
		output.Successf("Smoke: ok (HTTP %d)", status)
	default:
		output.Fail("Smoke: failed")
		for _, p := range c.Smoke.Problems {
			output.Fail("  " + p)
		}
	}
}
