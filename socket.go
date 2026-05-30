package udsrpc

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// RuntimeDir returns the OS-appropriate directory for an app's runtime
// state (socket, PID file). The directory is per-user and per-app.
//
//   - Linux:   $XDG_RUNTIME_DIR/<appName>/   (fallback: /tmp/<appName>-<uid>/)
//   - macOS:   ~/Library/Caches/<appName>/
//   - other:   os.TempDir()/<appName>-<uid>/
//
// appName should be a short lowercase identifier — typically the
// daemon binary name.
func RuntimeDir(appName string) string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Caches", appName)
	case "linux":
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			return filepath.Join(dir, appName)
		}
		return filepath.Join(os.TempDir(), appName+"-"+uidStr())
	default:
		return filepath.Join(os.TempDir(), appName+"-"+uidStr())
	}
}

func uidStr() string {
	return fmt.Sprintf("%d", os.Getuid())
}

// SocketPath returns the conventional Unix socket path for an app:
// <RuntimeDir>/<appName>.sock.
func SocketPath(appName string) string {
	return filepath.Join(RuntimeDir(appName), appName+".sock")
}

// PIDPath returns the conventional PID file path for an app:
// <RuntimeDir>/<appName>.pid.
func PIDPath(appName string) string {
	return filepath.Join(RuntimeDir(appName), appName+".pid")
}

// EnsureRuntimeDir creates the runtime directory if it doesn't exist,
// with owner-only permissions (0700).
func EnsureRuntimeDir(appName string) error {
	return os.MkdirAll(RuntimeDir(appName), 0700)
}
