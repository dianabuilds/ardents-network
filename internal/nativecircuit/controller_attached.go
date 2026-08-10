package nativecircuit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
)

// RunAttached connects two already-listening, owned Unix sockets through the
// authenticated native C-5/C2 Route. The Route sees only opaque stream bytes.
func RunAttached(ctx context.Context, identity experimentrun.Layout, applicationImage, toolImage, userSocket, serviceSocket string) (evidenceDir string, runErr error) {
	_, _, runDirectory, _, err := identity.OwnedPaths(true, true)
	if err != nil {
		return "", err
	}
	attached := &attachedSpec{userSocket: userSocket, serviceSocket: serviceSocket}
	if err := validateAttachedHostSockets(runDirectory, attached); err != nil {
		return "", err
	}
	return runNative(ctx, identity, applicationImage, toolImage, "", nil, attached)
}

func ensureNativeDirectory(path string, allowExisting bool) error {
	err := os.Mkdir(path, 0o700)
	if err == nil {
		return nil
	}
	if !allowExisting || !os.IsExist(err) {
		return err
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("shared native run path is not a real directory")
	}
	return nil
}

func validateAttachedHostSockets(runDirectory string, attached *attachedSpec) error {
	if attached == nil || attached.userSocket == attached.serviceSocket {
		return errors.New("attached Route requires two distinct Application sockets")
	}
	for _, path := range []string{attached.userSocket, attached.serviceSocket} {
		if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) != "app.sock" {
			return errors.New("attached Application socket path is not canonical")
		}
		relative, err := filepath.Rel(runDirectory, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("attached Application socket is outside the owned run")
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("attached Application endpoint is not a real Unix socket")
		}
	}
	return nil
}
