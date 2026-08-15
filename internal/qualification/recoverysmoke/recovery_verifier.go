package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func (observer dockerObserver) invokeRecoveryVerifier(ctx context.Context) (result recovery.Result, returnErr error) {
	if err := os.Chmod(observer.evidenceFile, 0o444); err != nil {
		return recovery.Result{}, fmt.Errorf("make recovery evidence verifier-readable: %w", err)
	}
	defer func() {
		if err := os.Chmod(observer.evidenceFile, 0o600); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("restore recovery evidence owner mode: %w", err))
		}
	}()
	raw, err := observer.docker(ctx, time.Minute, "run", "--rm", "--network", "none", "--read-only",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--user", "65532:65532",
		"--mount", "type=bind,src="+observer.evidenceFile+",dst=/run/ardents/evidence.json,readonly",
		resultImage(observer), "/usr/local/bin/ardents-recovery-qualify", "/run/ardents/evidence.json")
	for _, line := range splitLines(raw) {
		if json.Unmarshal(line, &result) == nil && result.Verdict != "" {
			if verifierExitMatches(err, result.Verdict) {
				return result, nil
			}
			return result, errors.Join(err, errors.New("independent recovery verifier exit differs from its verdict"))
		}
	}
	return result, errors.Join(err, errors.New("independent recovery verifier verdict is missing"))
}

func verifierExitMatches(runErr error, verdict string) bool {
	if runErr == nil {
		return verdict == "pass"
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return false
	}
	return exitErr.ExitCode() == 1 && verdict == "fail" || exitErr.ExitCode() == 2 && verdict == "invalid"
}

func resultImage(observer dockerObserver) string {
	if observer.imageID != "" {
		return observer.imageID
	}
	return observer.image
}

func (observer dockerObserver) assertDockerEmpty(ctx context.Context) error {
	for _, kind := range []string{"container", "network", "volume"} {
		raw, err := observer.docker(ctx, time.Minute, dockerOwnedListArguments(kind, observer.project)...)
		if err != nil {
			return fmt.Errorf("enumerate final owned Docker %s resources: %w", kind, err)
		}
		if strings.TrimSpace(string(raw)) != "" {
			return fmt.Errorf("recovery Docker ownership retains a %s resource", kind)
		}
	}
	return nil
}

func digestText(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func recoveryReceiptPath(root string) string { return filepath.Join(root, "recovery-negative.json") }

func writeRecoveryReceipt(root string, value any) error {
	return byteio.WriteJSON(recoveryReceiptPath(root), value, 1<<20)
}
