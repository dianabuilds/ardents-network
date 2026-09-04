//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type capturingRunner struct {
	output []byte
	calls  [][]string
}

func (runner *capturingRunner) run(arguments []string) ([]byte, int, error) {
	runner.calls = append(runner.calls, append([]string(nil), arguments...))
	return runner.output, 0, nil
}

type blockingRunner struct {
	output  []byte
	started chan struct{}
	release chan struct{}
}

func (runner *blockingRunner) run([]string) ([]byte, int, error) {
	close(runner.started)
	<-runner.release
	return runner.output, 0, nil
}

type failingRunner struct {
	calls int
}

func (runner *failingRunner) run([]string) ([]byte, int, error) {
	runner.calls++
	return []byte("durable source cycle reached its recorded deadline"), 1, errors.New("exit status 1")
}

type publishingRunner struct {
	output       []byte
	beforeReturn func() error
	err          error
}

func (runner *publishingRunner) run([]string) ([]byte, int, error) {
	runner.err = runner.beforeReturn()
	return runner.output, 0, nil
}

func testSourceOutput(outcomes [4]string) []byte {
	raw, _ := json.Marshal(sourceEvent{Schema: "ardents-source-event-v1", Kind: "source-wave-accepted",
		Generation: "generation-a", Epoch: 1, SourceAttempts: 2, SourceOutcomes: outcomes,
		LatestCompleteness: "latest completeness unproven"})
	return append(raw, '\n')
}

func clockAt(value time.Time) func() time.Time {
	return func() time.Time { return value }
}

func writeTestFixtures(t *testing.T) string {
	t.Helper()
	evidence := t.TempDir()
	fixtures := filepath.Join(evidence, "fixtures")
	if err := os.MkdirAll(fixtures, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"client.json", "client-probe.json"} {
		raw := []byte(`{"schema":"ardents-source-plan-v1","local_role_state_root":"/old/shared"}`)
		if err := os.WriteFile(filepath.Join(fixtures, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return evidence
}
