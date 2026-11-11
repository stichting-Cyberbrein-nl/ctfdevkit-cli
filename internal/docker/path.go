package docker

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// EnsureOnPath makes sure the docker binary is reachable through PATH. AI tools
// launched from a GUI on macOS and Windows often start without the shell PATH, so
// well-known install locations are probed and prepended when docker is missing.
func EnsureOnPath() error {
	if _, err := exec.LookPath("docker"); err == nil {
		return nil
	}
	for _, c := range dockerCandidates(runtime.GOOS) {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			dir := filepath.Dir(c)
			return os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	return errors.New("docker not found — install Docker Desktop and start it")
}

func dockerCandidates(goos string) []string {
	switch goos {
	case "darwin":
		home, _ := os.UserHomeDir()
		return []string{
			"/usr/local/bin/docker",
			"/opt/homebrew/bin/docker",
			filepath.Join(home, ".docker", "bin", "docker"),
			"/Applications/Docker.app/Contents/Resources/bin/docker",
		}
	case "linux":
		return []string{"/usr/bin/docker", "/usr/local/bin/docker"}
	case "windows":
		return []string{`C:\Program Files\Docker\Docker\resources\bin\docker.exe`}
	}
	return nil
}
