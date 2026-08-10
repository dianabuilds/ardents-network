package siteexperiment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
)

func validImageID(value string) bool {
	algorithm, digest, found := strings.Cut(value, ":")
	decoded, err := hex.DecodeString(digest)
	return found && algorithm == "sha256" && err == nil && len(decoded) == sha256.Size
}

func writeBoundedJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 4*1024*1024 {
		return errors.New("Gate C evidence exceeds 4 MiB")
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func cleanupGateCRuntime(identity experimentrun.Layout, runDirectory string) error {
	_, _, verifiedRun, _, err := identity.OwnedPaths(true, true)
	if err != nil || verifiedRun != runDirectory {
		return errors.New("Gate C cleanup identity changed")
	}
	info, err := os.Lstat(runDirectory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || filepath.Base(runDirectory) == "" {
		return errors.New("Gate C runtime is not an owned real directory")
	}
	if err := os.RemoveAll(runDirectory); err != nil {
		return err
	}
	if _, err := os.Stat(runDirectory); !os.IsNotExist(err) {
		return errors.New("Gate C runtime remains after cleanup")
	}
	return nil
}
