//go:build ignore

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordActionRejectsNoopBeforeControlledRefresh(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	_, manifest, err := prepareRun(evidence, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x51}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	request := actionRequest{Schema: actionSchema, Action: "noop", Reason: "not started"}
	if _, _, err := recordAction(manifest, "honest_user", request, &capturingRunner{}, time.Now); err == nil {
		t.Fatal("noop was accepted before the controlled refresh")
	}
}

func TestRecordActionRejectsConcurrentPersonaWriter(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	_, manifest, err := prepareRun(evidence, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x61}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	runner := &blockingRunner{output: testSourceOutput([4]string{"valid", "valid", "not-attempted", "not-attempted"}),
		started: make(chan struct{}), release: make(chan struct{})}
	finished := make(chan error, 1)
	go func() {
		_, _, err := recordAction(manifest, "honest_user", actionRequest{Schema: actionSchema, Action: "refresh"}, runner, time.Now)
		finished <- err
	}()
	<-runner.started
	if _, _, err := recordAction(manifest, "honest_user", actionRequest{Schema: actionSchema, Action: "refresh"},
		&capturingRunner{output: runner.output}, time.Now); !errors.Is(err, errWriterActive) {
		t.Fatalf("concurrent action returned %v, want errWriterActive", err)
	}
	close(runner.release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestProbeCanNoopAfterExpectedRejection(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	_, manifest, err := prepareRun(evidence, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x71}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	probeRunner := &capturingRunner{output: testSourceOutput([4]string{"valid", "invalid-state", "not-attempted", "not-attempted"})}
	if _, _, err := recordAction(manifest, "probe_consumer", actionRequest{Schema: actionSchema, Action: "refresh"}, probeRunner, time.Now); err != nil {
		t.Fatal(err)
	}
	request := actionRequest{Schema: actionSchema, Action: "noop", Reason: "skip probe"}
	if _, event, err := recordAction(manifest, "probe_consumer", request, probeRunner, time.Now); err != nil || event.Kind != "noop" {
		t.Fatalf("probe noop after expected rejection = %#v, %v", event, err)
	}
}

func TestRecordActionStopsAfterThreeConsecutiveInfrastructureErrors(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	_, manifest, err := prepareRun(evidence, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x81}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	runner := &failingRunner{}
	request := actionRequest{Schema: actionSchema, Action: "refresh"}
	for attempt := 0; attempt < 3; attempt++ {
		if _, event, err := recordAction(manifest, "honest_user", request, runner, time.Now); err == nil || event.Kind != "infra_error" {
			t.Fatalf("attempt %d = %#v, %v", attempt+1, event, err)
		}
	}
	if _, _, err := recordAction(manifest, "honest_user", request, runner, time.Now); err == nil {
		t.Fatal("fourth consecutive infrastructure retry was executed")
	}
	if runner.calls != 3 {
		t.Fatalf("runner calls = %d, want 3", runner.calls)
	}
}

func TestRecordActionPersistsTerminalMarkerOnDuplicatePublication(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	base := time.Date(2026, 9, 4, 3, 0, 0, 0, time.UTC)
	_, manifest, err := prepareRun(evidence, clockAt(base), bytes.NewReader(bytes.Repeat([]byte{0x91}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	persona := manifest.Personas["honest_user"]
	eventsDir := filepath.Join(manifest.HostRunRoot, "honest_user", "events")
	outcomes := persona.ExpectedOutcomes
	conflicting := eventRecord{Schema: eventSchema, RunID: manifest.RunID, Persona: "honest_user", Sequence: 1,
		RecordedAt: base.Format(time.RFC3339Nano), Action: "refresh", Kind: "accept",
		ConfigurationHash: persona.ConfigurationHash, Generation: "generation-a", ActualOutcomes: &outcomes}
	runner := &publishingRunner{output: testSourceOutput(outcomes), beforeReturn: func() error {
		_, err := publishEvent(eventsDir, conflicting)
		return err
	}}
	_, event, err := recordAction(manifest, "honest_user", actionRequest{Schema: actionSchema, Action: "refresh"}, runner, clockAt(base))
	if err == nil || runner.err != nil || event.Kind != "harness_abort" || event.Diagnostic != "duplicate_sequence" {
		t.Fatalf("duplicate publication = %#v, runner error %v, record error %v", event, runner.err, err)
	}
	marker := filepath.Join(manifest.HostRunRoot, "honest_user", "_meta", "terminal.json")
	raw, readErr := os.ReadFile(marker)
	if readErr != nil || !strings.Contains(string(raw), `"diagnostic": "duplicate_sequence"`) {
		t.Fatalf("terminal marker = %s, %v", raw, readErr)
	}
	second := &capturingRunner{output: testSourceOutput(outcomes)}
	if _, _, err := recordAction(manifest, "honest_user", actionRequest{Schema: actionSchema, Action: "refresh"}, second, clockAt(base)); err == nil {
		t.Fatal("action continued after duplicate publication")
	}
	if len(second.calls) != 0 {
		t.Fatalf("terminal persona executed %d additional refreshes", len(second.calls))
	}
	if _, err := verifyRun(manifest); err == nil || !strings.Contains(err.Error(), "duplicate_sequence") {
		t.Fatalf("verifier did not reject the terminal marker: %v", err)
	}
}
