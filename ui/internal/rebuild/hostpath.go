package rebuild

import (
	"path/filepath"
	"runtime"
	"strings"
)

// DockerVolumePath converts a host filesystem path into a form suitable for
// `docker run -v host:container` on Windows, macOS, and Linux.
//
// Docker Desktop (Windows/macOS) accepts native absolute paths; we normalize
// to absolute + forward slashes on Windows for broader CLI compatibility.
// On Linux/macOS the absolute path is returned as-is (slash-native).
func DockerVolumePath(hostPath string) string {
	if hostPath == "" {
		return hostPath
	}
	abs, err := filepath.Abs(hostPath)
	if err != nil {
		abs = hostPath
	}
	switch runtime.GOOS {
	case "windows":
		// Docker Desktop Windows: C:\foo\bar -> C:/foo/bar
		// Avoid legacy //c/ style unless needed; Desktop accepts drive-letter paths.
		return filepath.ToSlash(abs)
	default:
		// darwin, linux, freebsd, …
		return abs
	}
}

// WriteScriptUnixLF ensures shell scripts embedded for Docker use LF only.
// Writing CRLF from Windows hosts breaks `bash /work/build-only.sh` in Linux containers.
func WriteScriptUnixLF(script string) string {
	s := strings.ReplaceAll(script, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// HostPlatformLabel is a short OS/arch string for UI/logs.
func HostPlatformLabel() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// SupportsDockerSourceRebuild reports whether this host OS is a supported
// client for the Docker-based Frida rebuild feature (Windows, macOS, Linux).
func SupportsDockerSourceRebuild() bool {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		return true
	default:
		return false
	}
}
