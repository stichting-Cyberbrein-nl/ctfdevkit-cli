// Package platform provides OS/architecture detection and platform-specific utilities.
package platform

import (
	"os"
	"runtime"
	"strings"
)

// OS represents the detected operating system type.
type OS int

const (
	OSLinux   OS = iota
	OSMacOS   OS = iota
	OSWindows OS = iota
	OSWSL     OS = iota // Linux under Windows Subsystem for Linux
)

func (o OS) String() string {
	switch o {
	case OSLinux:
		return "linux"
	case OSMacOS:
		return "macos"
	case OSWindows:
		return "windows"
	case OSWSL:
		return "wsl"
	default:
		return "unknown"
	}
}

// IsUnix returns true for Linux, macOS, and WSL.
func (o OS) IsUnix() bool {
	return o == OSLinux || o == OSMacOS || o == OSWSL
}

// Arch represents the CPU architecture.
type Arch int

const (
	ArchAMD64 Arch = iota
	ArchARM64 Arch = iota
	ArchUnknown Arch = iota
)

func (a Arch) String() string {
	switch a {
	case ArchAMD64:
		return "amd64"
	case ArchARM64:
		return "arm64"
	default:
		return "unknown"
	}
}

// Platform holds detected environment information.
type Platform struct {
	OS    OS
	Arch  Arch
	IsWSL bool
}

// Detect returns the current platform information.
func Detect() Platform {
	p := Platform{
		Arch:  detectArch(),
		IsWSL: detectWSL(),
	}
	p.OS = detectOS(p.IsWSL)
	return p
}

func detectArch() Arch {
	switch runtime.GOARCH {
	case "amd64":
		return ArchAMD64
	case "arm64":
		return ArchARM64
	default:
		return ArchUnknown
	}
}

func detectWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func detectOS(isWSL bool) OS {
	if isWSL {
		return OSWSL
	}
	switch runtime.GOOS {
	case "darwin":
		return OSMacOS
	case "windows":
		return OSWindows
	case "linux":
		return OSLinux
	default:
		return OSLinux
	}
}

// AssetKey returns the platform key used in release manifests (e.g. "linux-amd64").
func (p Platform) AssetKey() string {
	osName := p.OS.String()
	if p.IsWSL {
		osName = "linux" // WSL uses Linux binaries
	}
	return osName + "-" + p.Arch.String()
}

// NeedsSudo returns true if the platform requires sudo for privileged operations.
func (p Platform) NeedsSudo() bool {
	return p.OS.IsUnix()
}

// WindowsHostsPath returns the Windows hosts file path (relevant for WSL).
func (p Platform) WindowsHostsPath() string {
	if p.IsWSL {
		return "/mnt/c/Windows/System32/drivers/etc/hosts"
	}
	return `C:\Windows\System32\drivers\etc\hosts`
}

// UnixHostsPath returns the Unix hosts file path.
func (p Platform) UnixHostsPath() string {
	return "/etc/hosts"
}
