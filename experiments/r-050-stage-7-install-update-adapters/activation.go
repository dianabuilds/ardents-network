//go:build ignore

package r050

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	activationFile       = "activation"
	maximumActivationLen = 16 * 1024
	maximumPathBytes     = 512
	maximumComponentLen  = 64

	afterCreate     = "after-create"
	afterWrite      = "after-write"
	afterFileSync   = "after-file-sync"
	afterReplace    = "after-replace"
	afterDurability = "after-durability"
)

var (
	errActivationBusy        = errors.New("activation-busy")
	errActivationUnsupported = errors.New("activation-unsupported")
)

type interruptionHook func(string)

// replaceActivation exercises the candidate primitive only. The payload is an
// opaque fixture; R-054, not this experiment, owns activation serialization.
func replaceActivation(root string, next []byte, hook interruptionHook) error {
	if len(next) == 0 || len(next) > maximumActivationLen {
		return fmt.Errorf("activation length %d: %w", len(next), errActivationUnsupported)
	}
	absRoot, err := validatePath(root)
	if err != nil {
		return err
	}
	if err := validatePlatformRoot(absRoot); err != nil {
		return err
	}
	current := filepath.Join(absRoot, activationFile)
	if err := validatePlatformCommitted(current); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	temporary, err := os.CreateTemp(absRoot, ".activation-next-")
	if err != nil {
		return fmt.Errorf("create activation temp: %w", err)
	}
	temporaryName := temporary.Name()
	replaced := false
	defer func() {
		_ = temporary.Close()
		if !replaced {
			_ = os.Remove(temporaryName)
		}
	}()
	callHook(hook, afterCreate)

	if err := platformSecureTemporary(temporaryName); err != nil {
		return err
	}
	if _, err := io.Copy(temporary, bytes.NewReader(next)); err != nil {
		return fmt.Errorf("write activation temp: %w", err)
	}
	callHook(hook, afterWrite)
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync activation temp: %w", err)
	}
	callHook(hook, afterFileSync)
	if err := validatePlatformTemporary(temporary); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close activation temp: %w", err)
	}

	if err := platformReplace(temporaryName, current); err != nil {
		return err
	}
	replaced = true
	callHook(hook, afterReplace)
	if err := platformSyncParent(absRoot); err != nil {
		return err
	}
	callHook(hook, afterDurability)

	got, err := platformReadActivation(current)
	if err != nil {
		return fmt.Errorf("read committed activation: %w", err)
	}
	if !bytes.Equal(got, next) {
		return fmt.Errorf("committed activation mismatch")
	}
	return validatePlatformCommitted(current)
}

func validatePath(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("absolute root: %w", err)
	}
	if len([]byte(absRoot)) > maximumPathBytes {
		return "", fmt.Errorf("root exceeds %d bytes: %w", maximumPathBytes, errActivationUnsupported)
	}
	for _, component := range strings.FieldsFunc(filepath.Clean(absRoot), func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if len([]byte(component)) > maximumComponentLen {
			return "", fmt.Errorf("path component exceeds %d bytes: %w", maximumComponentLen, errActivationUnsupported)
		}
	}
	return absRoot, nil
}

func activationBytes(root string) ([]byte, error) {
	return platformReadActivation(filepath.Join(root, activationFile))
}

func activationTemps(root string) ([]string, error) {
	return filepath.Glob(filepath.Join(root, ".activation-next-*"))
}

func callHook(hook interruptionHook, point string) {
	if hook != nil {
		hook(point)
	}
}

func hostManifest(root string) (string, error) {
	platform, err := platformManifest(root)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("goos=%s goarch=%s %s", runtime.GOOS, runtime.GOARCH, platform), nil
}
