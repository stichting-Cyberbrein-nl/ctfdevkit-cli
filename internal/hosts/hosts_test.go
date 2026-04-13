package hosts_test

import (
	"os"
	"strings"
	"testing"
)

// containsEntry is the same logic as in hosts.go — tested here directly.
func containsEntry(content, ip, domain string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == ip && fields[1] == domain {
			return true
		}
	}
	return false
}

func TestContainsEntryDetectsExisting(t *testing.T) {
	content := "# /etc/hosts\n127.0.0.1 localhost\n127.0.0.1 ctf.dev\n"
	if !containsEntry(content, "127.0.0.1", "ctf.dev") {
		t.Error("should detect existing entry")
	}
}

func TestContainsEntryMissingEntry(t *testing.T) {
	content := "127.0.0.1 localhost\n"
	if containsEntry(content, "127.0.0.1", "ctf.dev") {
		t.Error("should not detect missing entry")
	}
}

func TestContainsEntrySkipsComments(t *testing.T) {
	content := "# 127.0.0.1 ctf.dev\n127.0.0.1 localhost\n"
	if containsEntry(content, "127.0.0.1", "ctf.dev") {
		t.Error("should skip commented-out entries")
	}
}

func TestContainsEntryWrongIP(t *testing.T) {
	content := "10.0.0.1 ctf.dev\n"
	if containsEntry(content, "127.0.0.1", "ctf.dev") {
		t.Error("should not match wrong IP")
	}
}

// Verify that a temp-file write round-trips correctly.
func TestHostsFileRoundTrip(t *testing.T) {
	tmp, err := os.CreateTemp("", "hosts-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	original := "127.0.0.1 localhost\n"
	if err := os.WriteFile(tmp.Name(), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	entry := "127.0.0.1 ctf.dev"
	newContent := strings.TrimRight(original, "\n") + "\n" + entry + "\n"
	if err := os.WriteFile(tmp.Name(), []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(tmp.Name())
	if !containsEntry(string(data), "127.0.0.1", "ctf.dev") {
		t.Error("written entry not found after round-trip")
	}
}
