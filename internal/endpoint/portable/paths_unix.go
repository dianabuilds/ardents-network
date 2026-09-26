//go:build !windows

package portable

import (
	"errors"
	"os"
	"path/filepath"
)

// DefaultConfig resolves the Ubuntu Portable profile. Its runtime root must be
// explicitly supplied by the session; there is no /tmp or loopback fallback.
func DefaultConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	runtimeHome := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeHome == "" {
		return Config{}, errors.New("XDG_RUNTIME_DIR is required for the Portable Endpoint")
	}
	return Config{StateHome: filepath.Join(stateHome, "ardents"),
		RuntimeHome: filepath.Join(runtimeHome, "ardents")}, nil
}
