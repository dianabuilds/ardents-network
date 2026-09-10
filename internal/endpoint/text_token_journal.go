//go:build linux

package endpoint

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	textTokenJournalMagic    = "ARDTPS01"
	textTokenJournalHeader   = 48
	textTokenAttemptSize     = 153
	maximumTextTokenAttempts = 131072
)

// No holder, permission, token, context identity, Target or document persists.
type textTokenAttempt struct {
	digest, profile, receiver [32]byte
	duty                      uint64
	window                    time.Time
	class                     uint8
	attempt                   [32]byte
	observed                  time.Time
}

type textTokenJournal struct {
	mu       sync.Mutex
	root     string
	identity os.FileInfo
	network  [32]byte
	lease    transitAcquisitionLease
	clock    func() time.Time
	floor    time.Time
	records  map[[32]byte]textTokenAttempt
	closed   bool
	failure  error
}

func (journal *textTokenJournal) mark(token []byte, record textTokenAttempt) error {
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
	if !validTextTokenAttempt(record) || now.Before(journal.floor) || now.Before(record.window) || !now.Before(record.window.Add(time.Hour)) {
		return errors.New("text token attempt binding unavailable")
	}
	if _, exists := journal.records[record.digest]; exists {
		return errors.New("text token already potentially spent")
	}
	if err := journal.prune(now); err != nil {
		journal.failure = err
		return err
	}
	if len(journal.records) >= maximumTextTokenAttempts {
		return errors.New("text token attempt journal exhausted")
	}
	// An ambiguous write poisons this owner. The caller burns its stock item
	// before calling mark and cannot present it until this flush succeeds.
	file, err := os.OpenFile(filepath.Join(journal.root, "attempts"), os.O_WRONLY|os.O_APPEND, 0)
	if err == nil {
		info, inspectErr := file.Stat()
		if inspectErr != nil || journal.identity == nil || !os.SameFile(info, journal.identity) || info.Size() != int64(textTokenJournalHeader+len(journal.records)*textTokenAttemptSize) {
			err = errors.Join(errors.New("text token journal file changed"), inspectErr, file.Close())
			journal.failure = err
			return err
		}
		raw := encodeTextTokenAttempt(record)
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

func (journal *textTokenJournal) prune(now time.Time) error {
	retained := make(map[[32]byte]textTokenAttempt, len(journal.records))
	for hash, record := range journal.records {
		if now.Before(record.window.Add(time.Hour + time.Minute)) {
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

func (journal *textTokenJournal) Close() error {
	if journal == nil {
		return nil
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if !journal.closed {
		journal.closed = true
		journal.failure = errors.Join(journal.failure, journal.lease.release())
	}
	return journal.failure
}

func validTextTokenAttempt(record textTokenAttempt) bool {
	return record.digest != [32]byte{} && record.profile != [32]byte{} && record.receiver != [32]byte{} && record.duty != 0 &&
		record.class >= 1 && record.class <= 3 && record.attempt != [32]byte{} && !record.window.IsZero() &&
		record.window == record.window.UTC().Truncate(time.Hour) && !record.observed.IsZero() &&
		!record.observed.Before(record.window) && record.observed.Before(record.window.Add(time.Hour))
}

func encodeTextTokenAttempt(record textTokenAttempt) []byte {
	raw := make([]byte, 0, textTokenAttemptSize)
	for _, field := range [][32]byte{record.digest, record.profile, record.receiver} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, record.duty)
	raw = binary.BigEndian.AppendUint64(raw, uint64(record.window.Unix()))
	raw = append(raw, record.class)
	raw = append(raw, record.attempt[:]...)
	return binary.BigEndian.AppendUint64(raw, uint64(record.observed.Unix()))
}

func decodeTextTokenAttempt(raw []byte) (textTokenAttempt, error) {
	if len(raw) != textTokenAttemptSize {
		return textTokenAttempt{}, errors.New("text token attempt length invalid")
	}
	var record textTokenAttempt
	copy(record.digest[:], raw[:32])
	copy(record.profile[:], raw[32:64])
	copy(record.receiver[:], raw[64:96])
	record.duty = binary.BigEndian.Uint64(raw[96:104])
	record.window = time.Unix(int64(binary.BigEndian.Uint64(raw[104:112])), 0).UTC()
	record.class = raw[112]
	copy(record.attempt[:], raw[113:145])
	record.observed = time.Unix(int64(binary.BigEndian.Uint64(raw[145:153])), 0).UTC()
	if !validTextTokenAttempt(record) {
		return textTokenAttempt{}, errors.New("text token attempt invalid")
	}
	return record, nil
}
