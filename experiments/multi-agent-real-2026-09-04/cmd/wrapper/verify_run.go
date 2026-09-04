//go:build ignore

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

// verificationSummary is the bounded output of the single evidence reader.
type verificationSummary struct {
	RunID      string         `json:"run_id"`
	Events     map[string]int `json:"events"`
	Generation string         `json:"generation"`
}

func verifyRun(manifest runManifest) (verificationSummary, error) {
	summary := verificationSummary{RunID: manifest.RunID, Events: make(map[string]int, len(manifest.Personas))}
	for _, definition := range personaDefinitions {
		persona := manifest.Personas[definition.name]
		personaRoot := filepath.Join(manifest.HostRunRoot, definition.name)
		if terminal, exists, err := readTerminalMarker(personaRoot); err != nil {
			return verificationSummary{}, err
		} else if exists {
			if terminal.RunID != manifest.RunID || terminal.Persona != definition.name ||
				terminal.ConfigurationHash != persona.ConfigurationHash {
				return verificationSummary{}, fmt.Errorf("persona %s terminal marker identity is invalid", definition.name)
			}
			return verificationSummary{}, fmt.Errorf("persona %s is terminal: %s", definition.name, terminal.Diagnostic)
		}
		events, err := readEvents(filepath.Join(personaRoot, "events"))
		if err != nil {
			return verificationSummary{}, err
		}
		if len(events) < persona.MinimumEvents {
			return verificationSummary{}, fmt.Errorf("persona %s has %d events, requires at least %d", definition.name, len(events), persona.MinimumEvents)
		}
		hasAcceptedRefresh := false
		var firstRecorded, lastRecorded time.Time
		for index, event := range events {
			recorded, timeErr := time.Parse(time.RFC3339Nano, event.RecordedAt)
			if event.Schema != eventSchema || timeErr != nil ||
				event.RunID != manifest.RunID || event.Persona != definition.name || event.Sequence != uint64(index+1) ||
				event.ConfigurationHash != persona.ConfigurationHash {
				return verificationSummary{}, fmt.Errorf("persona %s event %d identity is invalid", definition.name, index+1)
			}
			if index == 0 {
				firstRecorded = recorded
			}
			lastRecorded = recorded
			switch event.Kind {
			case persona.ExpectedKind:
				if definition.name == "probe_consumer" && index != 0 {
					return verificationSummary{}, fmt.Errorf("persona %s event %d repeats the one-shot probe refresh", definition.name, index+1)
				}
				if event.Action != "refresh" || event.ActualOutcomes == nil || *event.ActualOutcomes != persona.ExpectedOutcomes ||
					event.Generation == "" || event.Error != "" {
					return verificationSummary{}, fmt.Errorf("persona %s event %d refresh evidence is invalid", definition.name, index+1)
				}
				hasAcceptedRefresh = true
				if summary.Generation == "" {
					summary.Generation = event.Generation
				} else if summary.Generation != event.Generation {
					return verificationSummary{}, errors.New("accepted refreshes disagree on generation")
				}
			case "noop":
				if !persona.AllowNoop || !hasAcceptedRefresh || event.Action != "noop" || event.ActualOutcomes != nil || event.Error != "" {
					return verificationSummary{}, fmt.Errorf("persona %s event %d has an invalid noop", definition.name, index+1)
				}
			case "harness_abort", "infra_error":
				return verificationSummary{}, fmt.Errorf("persona %s event %d is %s/%s", definition.name, index+1, event.Kind, event.Diagnostic)
			default:
				return verificationSummary{}, errors.New("event kind is not recognized")
			}
		}
		if lastRecorded.Sub(firstRecorded) < time.Duration(persona.MinimumSpanSeconds)*time.Second {
			return verificationSummary{}, fmt.Errorf("persona %s evidence spans %s, requires %ds", definition.name,
				lastRecorded.Sub(firstRecorded), persona.MinimumSpanSeconds)
		}
		summary.Events[definition.name] = len(events)
	}
	return summary, nil
}
