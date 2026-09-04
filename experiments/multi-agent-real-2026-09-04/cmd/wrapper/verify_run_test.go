//go:build ignore

package main

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyRunRejectsRepeatedProbeRefreshes(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	base := time.Date(2026, 9, 4, 2, 0, 0, 0, time.UTC)
	_, manifest, err := prepareRun(evidence, clockAt(base), bytes.NewReader(bytes.Repeat([]byte{0x45}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	intervals := map[string]time.Duration{
		"honest_user":    34 * time.Second,
		"battery_saver":  150 * time.Second,
		"probe_consumer": 45 * time.Second,
	}
	for _, definition := range personaDefinitions {
		persona := manifest.Personas[definition.name]
		for index := 0; index < persona.MinimumEvents; index++ {
			outcomes := persona.ExpectedOutcomes
			event := eventRecord{Schema: eventSchema, RunID: manifest.RunID, Persona: definition.name,
				Sequence: uint64(index + 1), RecordedAt: base.Add(time.Duration(index) * intervals[definition.name]).Format(time.RFC3339Nano),
				Action: "refresh", Kind: persona.ExpectedKind, ConfigurationHash: persona.ConfigurationHash,
				Generation: "generation-a", ActualOutcomes: &outcomes}
			if _, err := publishEvent(filepath.Join(manifest.HostRunRoot, definition.name, "events"), event); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := verifyRun(manifest); err == nil {
		t.Fatal("verifier accepted repeated probe refreshes after the first rejection")
	}
}
