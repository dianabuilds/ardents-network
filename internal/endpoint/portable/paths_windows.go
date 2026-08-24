//go:build windows

package portable

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// DefaultConfig resolves the Windows experimental Portable profile from the
// current user's Local AppData known folder rather than a hard-coded profile.
func DefaultConfig() (Config, error) {
	base, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return Config{}, err
	}
	endpoint := filepath.Join(base, "Ardents", "Endpoint")
	return Config{ConfigHome: filepath.Join(endpoint, "config"), StateHome: endpoint,
		CacheHome: filepath.Join(endpoint, "cache"), RuntimeHome: filepath.Join(endpoint, "r")}, nil
}
