package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

type nodeManifest struct {
	Schema          string    `json:"schema"`
	CreatedAt       time.Time `json:"created_at"`
	NetworkID       string    `json:"network_id"`
	AuthorityPublic string    `json:"authority_public"`
}

func seedNode(root, ardentsPath string) error {
	if root == "" || !filepath.IsAbs(root) || ardentsPath == "" || !filepath.IsAbs(ardentsPath) {
		return errors.New("node seed requires absolute fixture and ardents paths")
	}
	if err := nodeExecutable(ardentsPath); err != nil {
		return err
	}
	if err := Validate(root); err != nil {
		return err
	}
	raw, err := byteio.ReadFile(filepath.Join(root, "manifest.json"), 64<<10)
	if err != nil {
		return err
	}
	var manifest nodeManifest
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Schema != "ardents-h3-node-manifest-v1" {
		return errors.New("node manifest is invalid")
	}
	for _, item := range []struct {
		zone, epoch, material string
	}{
		{"e", "0001", "0000"}, {"n1", "0001", "0000"}, {"n2", "0001", "0001"},
		{"s1", "0001", "0000"}, {"s1", "0002", "0000"}, {"s2", "0001", "0000"}, {"s2", "0002", "0000"},
	} {
		if err := acceptNodeState(root, ardentsPath, manifest, item.zone, item.epoch, item.material); err != nil {
			return err
		}
	}
	return nil
}

func acceptNodeState(root, binary string, manifest nodeManifest, zone, epoch, material string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	artifacts := filepath.Join(root, "artifacts")
	command := exec.CommandContext(ctx, binary, "accept-offline",
		"--state-root", filepath.Join(root, "state", zone), "--network-id", manifest.NetworkID,
		"--authorities", manifest.AuthorityPublic, "--threshold", "1", "--at", manifest.CreatedAt.Format(time.RFC3339),
		"--epoch", filepath.Join(artifacts, "epoch-"+epoch+".bin"), "--inputs", filepath.Join(artifacts, "inputs"),
		"--materialization", filepath.Join(artifacts, "material-"+epoch+"-"+material+".bin"))
	diagnostics := byteio.NewBuffer(16 << 10)
	command.Stdout = io.Discard
	command.Stderr = diagnostics
	runErr := command.Run()
	if diagnostics.Overflowed() {
		return fmt.Errorf("seed node zone %s epoch %s: diagnostic output exceeded 16 KiB", zone, epoch)
	}
	if runErr != nil {
		return fmt.Errorf("seed node zone %s epoch %s: %w: %s", zone, epoch, runErr, diagnostics.Bytes())
	}
	return nil
}

func nodeExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("node ardents path is a directory")
	}
	return nil
}
