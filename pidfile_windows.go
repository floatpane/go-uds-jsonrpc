//go:build windows

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
// path is currently alive, else (pid, false).
// On Windows the check uses OpenProcess + GetExitCodeProcess; STILL_ACTIVE
// (259) indicates the process is still running.
func IsRunning(path string) (int, bool) {
	pid, err := ReadPID(path)
	if err != nil {
		return 0, false
	}

	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	h, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return pid, false
	}
	defer syscall.CloseHandle(h)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(h, &exitCode); err != nil {
		return pid, false
	}
	const STILL_ACTIVE = 259
	return pid, exitCode == STILL_ACTIVE
}

// RemovePID deletes the PID file.
func RemovePID(path string) error {
	return os.Remove(path)
}
