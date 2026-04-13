package prereqs

import (
	"strings"
	"testing"
)

func TestParseLinuxOSRelease(t *testing.T) {
	distro := parseLinuxOSRelease(`
NAME="Linux Mint"
ID=linuxmint
ID_LIKE="ubuntu debian"
VERSION_CODENAME=wilma
UBUNTU_CODENAME=noble
`)

	if distro.ID != "linuxmint" {
		t.Fatalf("ID = %q, want linuxmint", distro.ID)
	}
	if got, want := distro.IDLike, []string{"ubuntu", "debian"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("IDLike = %#v, want %#v", got, want)
	}
	if distro.VersionCodename != "wilma" {
		t.Fatalf("VersionCodename = %q, want wilma", distro.VersionCodename)
	}
	if distro.UbuntuCodename != "noble" {
		t.Fatalf("UbuntuCodename = %q, want noble", distro.UbuntuCodename)
	}
}

func TestResolveDockerAPTRepo(t *testing.T) {
	tests := []struct {
		name string
		in   linuxDistro
		want dockerAPTRepo
	}{
		{
			name: "kali rolling uses current Debian stable",
			in:   linuxDistro{ID: "kali", IDLike: []string{"debian"}, VersionCodename: "kali-rolling"},
			want: dockerAPTRepo{OS: "debian", Codename: "trixie"},
		},
		{
			name: "linux mint uses ubuntu codename",
			in:   linuxDistro{ID: "linuxmint", IDLike: []string{"ubuntu", "debian"}, VersionCodename: "wilma", UbuntuCodename: "noble"},
			want: dockerAPTRepo{OS: "ubuntu", Codename: "noble"},
		},
		{
			name: "pop os uses ubuntu codename when present",
			in:   linuxDistro{ID: "pop", IDLike: []string{"ubuntu", "debian"}, VersionCodename: "jammy", UbuntuCodename: "jammy"},
			want: dockerAPTRepo{OS: "ubuntu", Codename: "jammy"},
		},
		{
			name: "debian uses own codename",
			in:   linuxDistro{ID: "debian", IDLike: []string{}, VersionCodename: "bookworm"},
			want: dockerAPTRepo{OS: "debian", Codename: "bookworm"},
		},
		{
			name: "generic ubuntu based derivative uses ubuntu codename",
			in:   linuxDistro{ID: "zorin", IDLike: []string{"ubuntu", "debian"}, VersionCodename: "zorin", UbuntuCodename: "jammy"},
			want: dockerAPTRepo{OS: "ubuntu", Codename: "jammy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDockerAPTRepo(tt.in)
			if err != nil {
				t.Fatalf("resolveDockerAPTRepo() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveDockerAPTRepo() = %#v, want %#v", got, tt.want)
			}
			if err := validateDockerAPTRepo(got); err != nil {
				t.Fatalf("validateDockerAPTRepo(%#v) error = %v", got, err)
			}
		})
	}
}

func TestValidateDockerAPTRepoRejectsUnsupportedSuite(t *testing.T) {
	err := validateDockerAPTRepo(dockerAPTRepo{OS: "debian", Codename: "kali-rolling"})
	if err == nil {
		t.Fatal("validateDockerAPTRepo() error = nil, want unsupported suite error")
	}
}

func TestDockerSocketAccessErrorGivesDutchRecoverySteps(t *testing.T) {
	msg := dockerSocketAccessError("kali", true).Error()
	for _, want := range []string{"newgrp docker", "nieuwe terminal", "sudo reboot", "devkit"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("dockerSocketAccessError() missing %q in %q", want, msg)
		}
	}
}
