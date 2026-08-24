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
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(home, ".cache")
	}
	runtimeHome := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeHome == "" {
		return Config{}, errors.New("XDG_RUNTIME_DIR is required for the Portable Endpoint")
	}
	return Config{ConfigHome: filepath.Join(configHome, "ardents"), StateHome: filepath.Join(stateHome, "ardents"),
		CacheHome: filepath.Join(cacheHome, "ardents"), RuntimeHome: filepath.Join(runtimeHome, "ardents")}, nil
}
