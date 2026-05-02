package file

import (
	"fmt"
	"os"
	"path/filepath"
)

// OpenLogFile creates (or appends to) a persistent log file in the user's
// local app data directory and returns it. The caller is responsible for
// closing it.
//
// Path: %LOCALAPPDATA%\Mevius\patcher.log
func OpenLogFile() (*os.File, error) {
	dir, err := logDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}
	path := filepath.Join(dir, "patcher.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}
	return f, nil
}

// logDir returns the directory for log files.
// Uses %LOCALAPPDATA%\Mevius on Windows, ~/.local/share/Mevius elsewhere.
func logDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "BlankProjectDev", "Logs"), nil
}
