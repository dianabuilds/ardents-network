//go:build ignore

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

func recordAction(manifest runManifest, personaName string, request actionRequest, runner refreshRunner,
	now func() time.Time) (name string, event eventRecord, resultErr error) {
	if request.Schema != actionSchema || request.Action != "refresh" && request.Action != "noop" {
		return "", eventRecord{}, errors.New("agent action is not supported")
	}
	persona, ok := manifest.Personas[personaName]
	if !ok || persona.Name != personaName {
		return "", eventRecord{}, errors.New("persona is not owned by this run")
	}
	personaRoot := filepath.Join(manifest.HostRunRoot, personaName)
	lease, err := acquireWriter(personaRoot)
	if err != nil {
		return "", eventRecord{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, lease.release()) }()
	if terminal, exists, err := readTerminalMarker(personaRoot); err != nil {
		return "", eventRecord{}, err
	} else if exists {
		return filepath.Join(personaRoot, "_meta", "terminal.json"), terminal, errPersonaEnded
	}

	actualPlanHash, err := fileHash(filepath.Join(manifest.HostRunRoot, "plans", personaName+".json"))
	if err != nil || actualPlanHash != persona.SourcePlanHash {
		return "", eventRecord{}, errors.New("persona source plan changed after prepare")
	}
	eventsDir := filepath.Join(personaRoot, "events")
	events, err := readEvents(eventsDir)
	if err != nil {
		return "", eventRecord{}, err
	}
	sequence, err := nextSequence(events)
	if err != nil {
		return "", eventRecord{}, err
	}
	if consecutiveInfraErrors(events) >= 3 {
		return "", eventRecord{}, errors.New("persona exhausted its infra_error retry budget")
	}
	if request.Action == "noop" && (!persona.AllowNoop || !hasSuccessfulRefresh(events, persona.ExpectedKind)) {
		return "", eventRecord{}, errors.New("noop requires a prior successful controlled refresh")
	}
	event = eventRecord{Schema: eventSchema, RunID: manifest.RunID, Persona: personaName, Sequence: sequence,
		RecordedAt: now().UTC().Format(time.RFC3339Nano), Action: request.Action, Reason: request.Reason,
		ConfigurationHash: persona.ConfigurationHash}
	if request.Action == "noop" {
		event.Kind = "noop"
	} else {
		output, exitCode, runErr := runner.run(refreshArguments(manifest, persona))
		classified := classifyRefresh(persona, output, exitCode, runErr)
		event.Kind, event.Diagnostic, event.Generation = classified.Kind, classified.Diagnostic, classified.Generation
		event.ActualOutcomes, event.ExitCode, event.Error = classified.ActualOutcomes, classified.ExitCode, classified.Error
	}
	name, err = publishEvent(eventsDir, event)
	if errors.Is(err, errEventExists) {
		event.Kind = "harness_abort"
		event.Diagnostic = "duplicate_sequence"
		event.Generation = ""
		event.ActualOutcomes = nil
		event.ExitCode = 0
		event.Error = errEventExists.Error()
		marker, markerErr := writeTerminalMarker(personaRoot, event)
		if markerErr != nil {
			return "", event, errors.Join(errEventExists, markerErr)
		}
		return marker, event, fmt.Errorf("%w: %s", errPersonaEnded, event.Diagnostic)
	}
	if err != nil {
		return "", eventRecord{}, err
	}
	if event.Kind == "harness_abort" || event.Kind == "infra_error" {
		return name, event, fmt.Errorf("persona action recorded %s/%s", event.Kind, event.Diagnostic)
	}
	return name, event, nil
}

func consecutiveInfraErrors(events []eventRecord) int {
	count := 0
	for index := len(events) - 1; index >= 0 && events[index].Kind == "infra_error"; index-- {
		count++
	}
	return count
}

func hasSuccessfulRefresh(events []eventRecord, expectedKind string) bool {
	for _, event := range events {
		if event.Action == "refresh" && event.Kind == expectedKind {
			return true
		}
	}
	return false
}
