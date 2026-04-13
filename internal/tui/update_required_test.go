package tui

import (
	"strings"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "1.0.6", want: "v1.0.6"},
		{in: "v1.0.6", want: "v1.0.6"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := formatVersion(tt.in); got != tt.want {
				t.Fatalf("formatVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRequiredUpdateView(t *testing.T) {
	view := (requiredUpdateModel{current: "1.0.6", latest: "1.0.7"}).View()
	for _, want := range []string{"Update available! (v1.0.6) -> (v1.0.7)", "Update now"} {
		if !strings.Contains(view, want) {
			t.Fatalf("required update view missing %q in %q", want, view)
		}
	}
}
