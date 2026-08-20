package stage6verify_test

import (
	"bytes"
	"os"
	"path/filepath"
)

func frozenArtifactMutations() map[string]func(string) error {
	return map[string]func(string) error{
		"missing manifest file": func(root string) error {
			return os.Remove(filepath.Join(root, "manifest", "cells", "00.json"))
		},
		"extra evidence file": func(root string) error {
			return os.WriteFile(filepath.Join(root, "evidence", "unexpected"), []byte("x"), 0o600)
		},
		"missing private input": func(root string) error {
			return os.Remove(filepath.Join(root, "private", "admission-secret.bin"))
		},
		"extra private input": func(root string) error {
			return os.WriteFile(filepath.Join(root, "private", "unexpected"), []byte("x"), 0o600)
		},
		"changed private input": func(root string) error {
			path := filepath.Join(root, "private", "admission-secret.bin")
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			raw[0] ^= 1
			return os.WriteFile(path, raw, 0o600)
		},
		"noncanonical campaign whitespace": func(root string) error {
			return appendMutationBytes(filepath.Join(root, "manifest", "campaign.json"), []byte(" "))
		},
		"duplicate campaign field": func(root string) error {
			path := filepath.Join(root, "manifest", "campaign.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			raw = append([]byte(`{"schema":"ardents-stage-6-campaign-v1",`), raw[1:]...)
			return os.WriteFile(path, raw, 0o600)
		},
		"trailing campaign value": func(root string) error {
			return appendMutationBytes(filepath.Join(root, "manifest", "campaign.json"), []byte("{}"))
		},
		"missing JSONL newline": func(root string) error {
			path := filepath.Join(root, "evidence", "cells", "00", "observations", "trace.jsonl")
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(path, raw[:len(raw)-1], 0o600)
		},
		"additional JSONL newline": func(root string) error {
			return appendMutationBytes(filepath.Join(root, "evidence", "cells", "00", "observations", "trace.jsonl"), []byte("\n"))
		},
		"oversized evidence index": func(root string) error {
			return os.WriteFile(filepath.Join(root, "evidence", "index.json"), bytes.Repeat([]byte(" "), (1<<20)+1), 0o600)
		},
	}
}

func appendMutationBytes(path string, suffix []byte) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw = append(raw, suffix...)
	return os.WriteFile(path, raw, 0o600)
}
