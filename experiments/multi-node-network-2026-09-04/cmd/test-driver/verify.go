//go:build ignore

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// runVerify is the slice 1 entrypoint. It reads the expected generation from
// evidence/fixtures/current, calls VerifyConvergence over the six per-node
// event logs, prints the verdict, and exits non-zero on reject so the
// docker compose health chain (test-driver depends_on all nodes) surfaces
// the failure.
func runVerify(ctx context.Context, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: test-driver verify EVIDENCE_DIR")
	}
	evidenceDir, err := filepath.Abs(arguments[0])
	if err != nil {
		return fmt.Errorf("evidence dir abs: %w", err)
	}
	generationPath := filepath.Join(evidenceDir, "fixtures", "current")
	raw, err := os.ReadFile(generationPath)
	if err != nil {
		return fmt.Errorf("read expected generation: %w", err)
	}
	expected := string(raw)
	if len(expected) > 0 && expected[len(expected)-1] == '\n' {
		expected = expected[:len(expected)-1]
	}
	verdict, err := VerifyConvergence(evidenceDir, expected)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	fmt.Printf("test-driver: verify accept=%v distinct=%d reason=%q\n",
		verdict.Accept, verdict.DistinctResults, verdict.Reason)
	_ = ctx
	if !verdict.Accept {
		os.Exit(3)
	}
	return nil
}

// runVerifyAdversary is the slice 2 entrypoint. It mirrors runVerify but
// calls VerifyAdversaryScenario, which tightens the acceptance criteria to:
// per-node generation match, exact [4]string source_outcomes signature,
// specific node-id assignment (node-1..5 honest, node-6 probe), and the
// same distinct=1 convergence check.
func runVerifyAdversary(ctx context.Context, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: test-driver verify_adversary EVIDENCE_DIR")
	}
	evidenceDir, err := filepath.Abs(arguments[0])
	if err != nil {
		return fmt.Errorf("evidence dir abs: %w", err)
	}
	generationPath := filepath.Join(evidenceDir, "fixtures", "current")
	raw, err := os.ReadFile(generationPath)
	if err != nil {
		return fmt.Errorf("read expected generation: %w", err)
	}
	expected := string(raw)
	if len(expected) > 0 && expected[len(expected)-1] == '\n' {
		expected = expected[:len(expected)-1]
	}
	verdict, err := VerifyAdversaryScenario(evidenceDir, expected)
	if err != nil {
		return fmt.Errorf("verify_adversary: %w", err)
	}
	fmt.Printf("test-driver: verify_adversary accept=%v honest=%d probe=%d distinct=%d reason=%q\n",
		verdict.Accept, verdict.HonestNodeCount, verdict.ProbeNodeCount,
		verdict.DistinctResults, verdict.Reason)
	_ = ctx
	if !verdict.Accept {
		os.Exit(3)
	}
	return nil
}
