//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

var (
	errWriterActive = errors.New("persona already has an active writer")
	errEventExists  = errors.New("event sequence already exists")
	errPersonaEnded = errors.New("persona is terminal after harness abort")
)

var eventNamePattern = regexp.MustCompile(`^[0-9]{6}\.json$`)

type writerLease struct {
	file *os.File
	path string
}

func acquireWriter(personaRoot string) (*writerLease, error) {
	lockPath := filepath.Join(personaRoot, ".writer.lock")
	file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil, errWriterActive
	}
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		_ = file.Close()
		_ = os.Remove(lockPath)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(lockPath)
		return nil, err
	}
	return &writerLease{file: file, path: lockPath}, nil
}

func (lease *writerLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	err := lease.file.Close()
	lease.file = nil
	return errors.Join(err, os.Remove(lease.path))
}

func publishEvent(eventsDir string, event eventRecord) (string, error) {
	if event.Schema != eventSchema || event.RunID == "" || event.Persona == "" || event.Sequence == 0 || event.Kind == "" {
		return "", errors.New("event is incomplete")
	}
	if event.Sequence > 999999 {
		return "", errors.New("event sequence exceeds its bound")
	}
	raw, err := json.MarshalIndent(event, "", "  ")
	if err != nil || len(raw) > 32<<10 {
		return "", errors.New("event exceeds its encoding bound")
	}
	temporaryName, err := writeEventTemporary(eventsDir, append(raw, '\n'))
	if err != nil {
		return "", err
	}
	defer os.Remove(temporaryName)
	finalName := filepath.Join(eventsDir, fmt.Sprintf("%06d.json", event.Sequence))
	if err := os.Link(temporaryName, finalName); os.IsExist(err) {
		return "", errEventExists
	} else if err != nil {
		return "", err
	}
	return finalName, nil
}

func writeEventTemporary(eventsDir string, raw []byte) (string, error) {
	metaDir := filepath.Join(filepath.Dir(eventsDir), "_meta")
	temporary, err := os.CreateTemp(metaDir, "event-")
	if err != nil {
		return "", err
	}
	temporaryName := temporary.Name()
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if _, err = temporary.Write(raw); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	succeeded = true
	return temporaryName, nil
}

func readEvents(eventsDir string) ([]eventRecord, error) {
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !eventNamePattern.MatchString(entry.Name()) {
			return nil, fmt.Errorf("event directory contains unexpected entry %q", entry.Name())
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	events := make([]eventRecord, 0, len(names))
	for _, name := range names {
		event, err := readEvent(filepath.Join(eventsDir, name))
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func readEvent(name string) (eventRecord, error) {
	raw, err := os.ReadFile(name)
	if err != nil || len(raw) == 0 || len(raw) > 32<<10 {
		return eventRecord{}, errors.New("event is unavailable or unbounded")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var event eventRecord
	if err := decoder.Decode(&event); err != nil {
		return eventRecord{}, fmt.Errorf("decode event %s: %w", filepath.Base(name), err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return eventRecord{}, fmt.Errorf("event %s has trailing content", filepath.Base(name))
	}
	sequence, _ := strconv.ParseUint(filepath.Base(name)[:6], 10, 64)
	if event.Sequence != sequence {
		return eventRecord{}, fmt.Errorf("event %s sequence disagrees with its name", filepath.Base(name))
	}
	return event, nil
}

func writeTerminalMarker(personaRoot string, event eventRecord) (string, error) {
	name := filepath.Join(personaRoot, "_meta", "terminal.json")
	if event.Schema != eventSchema || event.RunID == "" || event.Persona == "" || event.Sequence == 0 ||
		event.Kind != "harness_abort" || event.Diagnostic == "" {
		return "", errors.New("terminal marker is incomplete")
	}
	if err := writeNewJSON(name, event); err != nil {
		return "", err
	}
	return name, nil
}

func readTerminalMarker(personaRoot string) (eventRecord, bool, error) {
	name := filepath.Join(personaRoot, "_meta", "terminal.json")
	raw, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return eventRecord{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > 32<<10 {
		return eventRecord{}, false, errors.New("terminal marker is unavailable or unbounded")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var event eventRecord
	if err := decoder.Decode(&event); err != nil {
		return eventRecord{}, false, errors.New("terminal marker is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return eventRecord{}, false, errors.New("terminal marker has trailing content")
	}
	if event.Schema != eventSchema || event.RunID == "" || event.Persona == "" || event.Sequence == 0 ||
		event.Kind != "harness_abort" || event.Diagnostic == "" {
		return eventRecord{}, false, errors.New("terminal marker identity is invalid")
	}
	return event, true, nil
}

func nextSequence(events []eventRecord) (uint64, error) {
	for index, event := range events {
		expected := uint64(index + 1)
		if event.Sequence != expected {
			return 0, errors.New("event sequence has a gap or duplicate")
		}
		if event.Kind == "harness_abort" {
			return 0, errors.New("persona is terminal after harness_abort")
		}
	}
	if len(events) >= 999999 {
		return 0, errors.New("event sequence is exhausted")
	}
	return uint64(len(events) + 1), nil
}
