//go:build linux

package tokenjournal

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/durableroot"
)

const (
	journalMagic    = "ARDTPS01"
	journalHeader   = 48
	attemptSize     = 153
	maximumAttempts = 131072
)

// Attempt binds one token spend to a profile, receiver, duty, window, class,
// and Route nonce. Mark computes the digest and observation time; no token
// or private context identity persists.
type Attempt struct {
	digest            [32]byte
	Profile, Receiver [32]byte
	Duty              uint64
	Window            time.Time
	Class             uint8
	Nonce             [32]byte
	observed          time.Time
}

// Journal retains one exclusive durable attempt owner with replay and time floors.
type Journal struct {
	mu       sync.Mutex
	root     string
	identity os.FileInfo
	network  [32]byte
	lease    *durableroot.Lease
	clock    func() time.Time
	floor    time.Time
	records  map[[32]byte]Attempt
	closed   bool
	failure  error
}

// Mark durably records a potentially spent token before its caller may present it.
func (journal *Journal) Mark(token []byte, record Attempt) error {
	if journal == nil || len(token) != 354 {
		return errors.New("text token journal unavailable")
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if journal.closed || journal.failure != nil {
		return errors.New("text token journal closed or failed")
	}
	now := journal.clock().UTC().Truncate(time.Second)
	record.digest, record.observed = sha256.Sum256(token), now
	if !validAttempt(record) || now.Before(journal.floor) || now.Before(record.Window) || !now.Before(record.Window.Add(time.Hour)) {
		return errors.New("text token attempt binding unavailable")
	}
	if _, exists := journal.records[record.digest]; exists {
		return errors.New("text token already potentially spent")
	}
	if err := journal.prune(now); err != nil {
		journal.failure = err
		return err
	}
	if len(journal.records) >= maximumAttempts {
		return errors.New("text token attempt journal exhausted")
	}
	// An ambiguous write poisons this owner. The caller burns its stock item
	// before calling Mark and cannot present it until this flush succeeds.
	file, err := os.OpenFile(filepath.Join(journal.root, "attempts"), os.O_WRONLY|os.O_APPEND, 0)
	if err == nil {
		info, inspectErr := file.Stat()
		if inspectErr != nil || journal.identity == nil || !os.SameFile(info, journal.identity) || info.Size() != int64(journalHeader+len(journal.records)*attemptSize) {
			err = errors.Join(errors.New("text token journal file changed"), inspectErr, file.Close())
			journal.failure = err
			return err
		}
		raw := encodeAttempt(record)
		var written int
		written, err = file.Write(raw)
		if err == nil && written != len(raw) {
			err = errors.New("short text token attempt write")
		}
		if err == nil {
			err = file.Sync()
		}
		err = errors.Join(err, file.Close())
	}
	if err != nil {
		journal.failure = err
		return err
	}
	journal.records[record.digest], journal.floor = record, now
	return nil
}

func (journal *Journal) prune(now time.Time) error {
	retained := make(map[[32]byte]Attempt, len(journal.records))
	for hash, record := range journal.records {
		if now.Before(record.Window.Add(time.Hour + time.Minute)) {
			retained[hash] = record
		}
	}
	if len(retained) == len(journal.records) {
		return nil
	}
	if err := journal.replace(retained, now); err != nil {
		return err
	}
	journal.records, journal.floor = retained, now
	return nil
}

// Close releases the exclusive root lease and retains prior write failure.
func (journal *Journal) Close() error {
	if journal == nil {
		return nil
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if !journal.closed {
		journal.closed = true
		journal.failure = errors.Join(journal.failure, journal.lease.Release())
	}
	return journal.failure
}

func validAttempt(record Attempt) bool {
	return record.digest != [32]byte{} && record.Profile != [32]byte{} && record.Receiver != [32]byte{} && record.Duty != 0 &&
		record.Class >= 1 && record.Class <= 3 && record.Nonce != [32]byte{} && !record.Window.IsZero() &&
		record.Window == record.Window.UTC().Truncate(time.Hour) && !record.observed.IsZero() &&
		!record.observed.Before(record.Window) && record.observed.Before(record.Window.Add(time.Hour))
}

func encodeAttempt(record Attempt) []byte {
	raw := make([]byte, 0, attemptSize)
	for _, field := range [][32]byte{record.digest, record.Profile, record.Receiver} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, record.Duty)
	raw = binary.BigEndian.AppendUint64(raw, uint64(record.Window.Unix()))
	raw = append(raw, record.Class)
	raw = append(raw, record.Nonce[:]...)
	return binary.BigEndian.AppendUint64(raw, uint64(record.observed.Unix()))
}

func decodeAttempt(raw []byte) (Attempt, error) {
	if len(raw) != attemptSize {
		return Attempt{}, errors.New("text token attempt length invalid")
	}
	var record Attempt
	copy(record.digest[:], raw[:32])
	copy(record.Profile[:], raw[32:64])
	copy(record.Receiver[:], raw[64:96])
	record.Duty = binary.BigEndian.Uint64(raw[96:104])
	record.Window = time.Unix(int64(binary.BigEndian.Uint64(raw[104:112])), 0).UTC()
	record.Class = raw[112]
	copy(record.Nonce[:], raw[113:145])
	record.observed = time.Unix(int64(binary.BigEndian.Uint64(raw[145:153])), 0).UTC()
	if !validAttempt(record) {
		return Attempt{}, errors.New("text token attempt invalid")
	}
	return record, nil
}
