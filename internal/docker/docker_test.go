package docker

import "testing"

func TestIsPermissionError(t *testing.T) {
	output := `permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock`
	if !IsPermissionError(output) {
		t.Fatal("IsPermissionError() = false, want true")
	}
}

func TestIsPermissionErrorIgnoresOtherPermissionErrors(t *testing.T) {
	output := `open /tmp/devkit: permission denied`
	if IsPermissionError(output) {
		t.Fatal("IsPermissionError() = true, want false")
	}
}
