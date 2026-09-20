//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"sync"
)

// The terminal authenticates completeness against accidental truncation or
// mixing of local logs. It is a checksum, not an independent attestation.
type evidenceJournal struct {
	mu      sync.Mutex
	output  io.Writer
	digest  hash.Hash
	records uint64
	closed  bool
	failure error
}

func newEvidenceJournal(output io.Writer) *evidenceJournal {
	return &evidenceJournal{output: output, digest: sha256.New()}
}

func (journal *evidenceJournal) emit(value any) error {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if journal.closed {
		return errors.New("qualification evidence already terminated")
	}
	return journal.write(value)
}

func (journal *evidenceJournal) write(value any) error {
	if journal.failure != nil {
		return journal.failure
	}
	body, err := json.Marshal(value)
	if err != nil {
		journal.failure = err
		return err
	}
	body = append(body, '\n')
	n, err := journal.output.Write(body)
	if err == nil && n != len(body) {
		err = io.ErrShortWrite
	}
	if err != nil {
		journal.failure = err
		return err
	}
	_, _ = journal.digest.Write(body)
	journal.records++
	return nil
}

// finish is called only after all participants have returned joined cleanup.
func (journal *evidenceJournal) finish(outcome error) error {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if journal.closed {
		return errors.New("qualification evidence already terminated")
	}
	journal.closed = true
	failure := ""
	if outcome != nil {
		failure = outcome.Error()
	}
	return journal.write(evidenceTerminal{Kind: "terminal", Records: journal.records,
		SHA256: hex.EncodeToString(journal.digest.Sum(nil)), Failure: failure})
}
