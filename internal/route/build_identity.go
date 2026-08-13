package route

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

func buildIdentity(existingSource string, existingDigest [32]byte, runErr error) (string, [32]byte, error) {
	if existingSource != "" && existingDigest != [32]byte{} {
		return existingSource, existingDigest, runErr
	}
	source := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		source = info.Main.Path + "@" + info.Main.Version
	}
	path, err := os.Executable()
	if err != nil {
		return source, [32]byte{}, errors.Join(runErr, fmt.Errorf("locate route build: %w", err))
	}
	file, err := os.Open(path)
	if err != nil {
		return source, [32]byte{}, errors.Join(runErr, fmt.Errorf("open route build: %w", err))
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return source, [32]byte{}, errors.Join(runErr, fmt.Errorf("digest route build: %w", err))
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return source, digest, runErr
}
