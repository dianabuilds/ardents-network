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

// PrepareConfig declares one immutable fixture preparation mode.
type PrepareConfig struct {
	Root              string
	Now               time.Time
	ArdentsPath       string
	LinuxUIDOwnership bool
}

// Prepare freezes one bounded E/S1/S2/N1/N2 fixture without starting candidates.
func Prepare(config PrepareConfig) error {
	if config.Root == "" || !filepath.IsAbs(config.Root) || config.Now.IsZero() {
		return errors.New("node fixture requires an absolute root and verification time")
	}
	if _, err := os.Stat(config.Root); err == nil {
		return errors.New("node fixture root already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(config.Root, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(config.Root, ".ardents-node-owned"), []byte(nodeOwner), 0o600); err != nil {
		return err
	}
	fixture, err := newNodeFixture(config.Now.UTC())
	if err != nil {
		return err
	}
	if err := fixture.write(config.Root); err != nil {
		return err
	}
	if config.ArdentsPath != "" {
		if err := seedNode(config.Root, config.ArdentsPath); err != nil {
			return err
		}
	}
	if config.LinuxUIDOwnership {
		return assignNodeOwnership(config.Root)
	}
	return nil
}
