package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type evidenceTerminal struct {
	Kind    string
	Records uint64
	SHA256  string
	Failure string
}

// readCompletedEvidence validates the complete byte stream before its caller
// may accept any results. It reports every independently observable structural
// and semantic defect in the stream instead of hiding later defects behind the
// first one. Cancellation, missing cleanup and a truncated final line still
// fail even when earlier participant results looked successful.
func readCompletedEvidence(path string, accept func([]byte) error) (outcome error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 256<<20 {
		return errors.New("runner evidence exceeds its bound")
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 512<<10)
	scanner.Split(evidenceLine)
	digest := sha256.New()
	var count uint64
	participants, terminal := 0, false
	results := make(map[int]bool)
	for scanner.Scan() {
		raw := scanner.Bytes()
		recordNumber := count + 1
		var record struct {
			Kind         string
			Participants int
			Participant  int
			PlanSHA256   string
			Event        json.RawMessage
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			outcome = errors.Join(outcome, fmt.Errorf("runner evidence record %d: %w", recordNumber, err))
			_, _ = digest.Write(raw)
			count++
			continue
		}
		if terminal {
			outcome = errors.Join(outcome, fmt.Errorf("runner evidence record %d follows terminal evidence", recordNumber))
			continue
		}
		if count == 0 {
			if record.Kind != "candidate" || record.Participants < 1 || record.Participants > 5 {
				outcome = errors.Join(outcome, errors.New("runner candidate or participant set absent"))
			} else {
				participants = record.Participants
			}
			if _, err := decodeIdentity(record.PlanSHA256); err != nil {
				outcome = errors.Join(outcome, fmt.Errorf("runner candidate plan identity: %w", err))
			}
		} else {
			switch record.Kind {
			case "terminal":
				var ending evidenceTerminal
				if err := json.Unmarshal(raw, &ending); err != nil {
					outcome = errors.Join(outcome, fmt.Errorf("runner terminal evidence: %w", err))
				} else {
					if ending.Failure != "" {
						outcome = errors.Join(outcome, errors.New("runner ended with failure: "+ending.Failure))
					}
					if ending.Records != count || ending.SHA256 != hex.EncodeToString(digest.Sum(nil)) {
						outcome = errors.Join(outcome, errors.New("runner evidence checksum or record count differs"))
					}
					if participants == 0 || len(results) != participants {
						outcome = errors.Join(outcome, errors.New("runner cleanup results incomplete"))
					}
				}
				terminal = true
				continue
			case "result":
				if record.Participant < 0 || record.Participant >= participants || results[record.Participant] {
					outcome = errors.Join(outcome, fmt.Errorf("runner evidence record %d has duplicated or invalid result participant", recordNumber))
				} else {
					results[record.Participant] = true
				}
			case "":
				if record.Participant < 0 || record.Participant >= participants || len(record.Event) == 0 || bytes.Equal(record.Event, []byte("null")) {
					outcome = errors.Join(outcome, fmt.Errorf("runner evidence record %d has no observation identity", recordNumber))
				}
			default:
				outcome = errors.Join(outcome, fmt.Errorf("runner evidence record %d has unexpected kind %q", recordNumber, record.Kind))
			}
		}
		if err := accept(raw); err != nil {
			outcome = errors.Join(outcome, fmt.Errorf("runner evidence record %d: %w", recordNumber, err))
		}
		_, _ = digest.Write(raw)
		count++
	}
	if err := scanner.Err(); err != nil {
		outcome = errors.Join(outcome, err)
	}
	if !terminal {
		outcome = errors.Join(outcome, errors.New("runner joined-cleanup terminal absent"))
	}
	return outcome
}

func evidenceLine(data []byte, atEOF bool) (int, []byte, error) {
	if index := bytes.IndexByte(data, '\n'); index >= 0 {
		return index + 1, data[:index+1], nil
	}
	if atEOF && len(data) != 0 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	return 0, nil, nil
}
