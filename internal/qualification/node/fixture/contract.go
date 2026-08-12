package fixture

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

const nodeOwner = "ardents-h3-node-fixture-v1\n"

// Validate verifies that root is an owned immutable Node qualification fixture.
func Validate(root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("node fixture root must be absolute")
	}
	owner, err := byteio.ReadFile(filepath.Join(root, ".ardents-node-owned"), 64)
	if err != nil || string(owner) != nodeOwner {
		return errors.New("node fixture ownership marker is invalid")
	}
	return nil
}

// Prepare freezes one bounded E/S1/S2/N1/N2 fixture without starting candidates.
func Prepare(root string, now time.Time, ardentsPath string) error {
	if root == "" || !filepath.IsAbs(root) || now.IsZero() {
		return errors.New("node fixture requires an absolute root and verification time")
	}
	if _, err := os.Stat(root); err == nil {
		return errors.New("node fixture root already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, ".ardents-node-owned"), []byte(nodeOwner), 0o600); err != nil {
		return err
	}
	fixture, err := newNodeFixture(now.UTC())
	if err != nil {
		return err
	}
	if err := fixture.write(root); err != nil {
		return err
	}
	if ardentsPath != "" {
		if err := seedNode(root, ardentsPath); err != nil {
			return err
		}
	}
	return assignNodeOwnership(root)
}
