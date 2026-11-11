package payload

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteComposeEnv(t *testing.T) {
	assignments := `C:\a b\#x`
	if runtime.GOOS != "windows" {
		assignments = "/a b/#x" // filepath.Abs keeps absolute paths of the host OS unchanged
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr string
	}{
		{name: "spaces and hash", path: assignments, want: "ASSIGNMENTS_PATH='" + assignments + "'\n"},
		{name: "single quote", path: filepath.Join(assignments, "it's"), wantErr: "path must not contain a single quote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := WriteComposeEnv(dir, ComposeEnv{BindIP: "127.0.0.2", HTTPPort: 80, HTTPSPort: 443, AppURL: "https://ctf.dev", AssignmentsPath: tt.path})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, ".env"))
			if err != nil {
				t.Fatal(err)
			}
			got := string(data)
			if !strings.Contains(got, tt.want) || !strings.Contains(got, "DEVKIT_HTTPS_PORT='443'\n") {
				t.Fatalf(".env = %q, want line %q", got, tt.want)
			}
		})
	}
}
