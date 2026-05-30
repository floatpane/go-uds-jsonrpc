//go:build !windows

package udsrpc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes the current process ID to path.
func WritePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// ReadPID reads the process ID from a PID file.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file: %w", err)
	}
	return pid, nil
}

// IsRunning returns (pid, true) if a process with the PID stored at
// path is currently alive, else (pid, false). A signal-0 probe is used
// on Unix; the call is non-destructive.
func IsRunning(path string) (int, bool) {
	pid, err := ReadPID(path)
	if err != nil {
		return 0, false
	}
	err = syscall.Kill(pid, 0)
	return pid, err == nil
}

// RemovePID deletes the PID file.
func RemovePID(path string) error {
	return os.Remove(path)
}
