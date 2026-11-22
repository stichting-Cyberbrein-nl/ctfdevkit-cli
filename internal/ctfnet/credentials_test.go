package ctfnet

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCredentialsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), credentialsFile)
	want := Credentials{
		PlatformURL:  "https://ctf.cyberbrein.net",
		AccessToken:  "acc",
		RefreshToken: "ref",
		ExpiresAt:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		User:         User{ID: 1, Name: "Admin", Username: "admin"},
	}
	if err := SaveCredentialsTo(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCredentialsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}
	got.ExpiresAt = want.ExpiresAt
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("mode = %o, want 600", mode)
		}
	}
	if err := DeleteCredentialsAt(path); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCredentialsAt(path); err != nil {
		t.Fatalf("deleting a missing file: %v", err)
	}
}
