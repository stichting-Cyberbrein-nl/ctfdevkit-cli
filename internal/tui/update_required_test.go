package tui

import "testing"

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
