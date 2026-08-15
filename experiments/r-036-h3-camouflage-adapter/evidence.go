//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func scanState(root string) (int, int64, error) {
	entries := 0
	var bytes int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		entries++
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeDevice != 0 {
			return errors.New("state contains link or device")
		}
		bytes += info.Size()
		return nil
	})
	if entries > 32 || bytes > 1<<20 {
		return entries, bytes, errors.New("state limit exceeded")
	}
	return entries, bytes, err
}

func writeReport(path string, report campaignReport, client, server *child, provenance runProvenance) error {
	secret := filepath.Join(path, "secret")
	if err := os.MkdirAll(secret, 0700); err != nil {
		return err
	}
	files := map[string][]byte{
		"client-control.txt": client.transcript,
		"client-stderr.txt":  client.stderr.Bytes(),
		"server-control.txt": server.transcript,
		"server-stderr.txt":  server.stderr.Bytes(),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(secret, name), content, 0600); err != nil {
			return err
		}
	}
	manifest, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return err
	}
	manifest = append(manifest, '\n')
	if err := os.WriteFile(filepath.Join(secret, "run-manifest.json"), manifest, 0600); err != nil {
		return err
	}
	digest := sha256.Sum256(manifest)
	report.RunManifestSHA256 = hex.EncodeToString(digest[:])
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "summary.json"), append(encoded, '\n'), 0600)
}
