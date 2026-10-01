//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
)

func verifyCompletedRun(arguments []string, output io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("usage: ardents-qualification verify-run <runner-jsonl>")
	}
	if err := readCompletedEvidence(arguments[0], func([]byte) error { return nil }); err != nil {
		return err
	}
	body, err := os.ReadFile(arguments[0])
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	return json.NewEncoder(output).Encode(struct {
		Kind, SHA256 string
		Bytes        int
	}{"completed-evidence", hex.EncodeToString(sum[:]), len(body)})
}
