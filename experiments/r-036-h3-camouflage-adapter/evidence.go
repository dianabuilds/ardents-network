//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
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

func writeEvidence(path string, client, server *child, provenance runProvenance) (string, error) {
	secret := filepath.Join(path, "secret")
	if err := os.MkdirAll(secret, 0700); err != nil {
		return "", err
	}
	files := map[string][]byte{
		"client-control.txt": client.transcript,
		"client-stderr.txt":  client.stderr.Bytes(),
		"server-control.txt": server.transcript,
		"server-stderr.txt":  server.stderr.Bytes(),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(secret, name), content, 0600); err != nil {
			return "", err
		}
	}
	manifest, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return "", err
	}
	manifest = append(manifest, '\n')
	if err := os.WriteFile(filepath.Join(secret, "run-manifest.json"), manifest, 0600); err != nil {
		return "", err
	}
	digest := sha256.Sum256(manifest)
	return hex.EncodeToString(digest[:]), nil
}

func publishSummary(path string, report *campaignReport, started time.Time) error {
	final := filepath.Join(path, "summary.json")
	temporary := final + ".tmp"
	_ = os.Remove(final)
	_ = os.Remove(temporary)
	for range 2 {
		report.CleanupMilliseconds = time.Since(started).Milliseconds()
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(temporary, append(encoded, '\n'), 0600); err != nil {
			return err
		}
		if elapsed := time.Since(started); elapsed > 6*time.Second {
			_ = os.Remove(temporary)
			return fmt.Errorf("cleanup and evidence exceeded 6 seconds: %d ms", elapsed.Milliseconds())
		}
	}
	if err := os.Rename(temporary, final); err != nil {
		return err
	}
	if elapsed := time.Since(started); elapsed > 6*time.Second {
		_ = os.Remove(final)
		return fmt.Errorf("summary publication exceeded 6 seconds: %d ms", elapsed.Milliseconds())
	}
	return nil
}
