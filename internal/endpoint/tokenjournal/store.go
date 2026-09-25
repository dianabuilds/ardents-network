//go:build linux

package tokenjournal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/durableroot"
)

const journalMarker = "ardents-token-attempts-v1\n"

// Open claims one existing private root and refuses ambiguous retained state.
func Open(root string, network [32]byte, clock func() time.Time) (*Journal, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || network == [32]byte{} || clock == nil || clock().IsZero() {
		return nil, errors.New("text token journal configuration invalid")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("text token journal root unavailable")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) > 8 {
		return nil, errors.New("text token journal root shape unavailable")
	}
	fresh := len(entries) == 0
	for _, entry := range entries {
		name := entry.Name()
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !info.Mode().IsRegular() || name != "owner.lock" && name != "root.marker" && name != "attempts" && !strings.HasPrefix(name, ".attempts-") {
			return nil, errors.New("text token journal root entry invalid")
		}
	}
	if !fresh {
		marker, err := os.ReadFile(filepath.Join(root, "root.marker"))
		if err != nil || string(marker) != journalMarker {
			return nil, errors.New("text token journal marker unavailable")
		}
	}
	if err := durableroot.Secure(root); err != nil {
		return nil, err
	}
	lease, err := durableroot.Acquire(filepath.Join(root, "owner.lock"))
	if err != nil {
		return nil, err
	}
	journal := &Journal{root: root, network: network, clock: clock, lease: lease, records: make(map[[32]byte]Attempt)}
	fail := func(cause error) (*Journal, error) { return nil, errors.Join(cause, lease.Release()) }
	if fresh {
		current, err := os.ReadDir(root)
		if err != nil || len(current) != 1 || current[0].Name() != "owner.lock" {
			return fail(errors.New("text token journal changed before claim"))
		}
		if err := writeJournalFile(filepath.Join(root, "root.marker"), []byte(journalMarker)); err != nil {
			return fail(err)
		}
		journal.floor = clock().UTC().Truncate(time.Second)
		if err := writeJournalFile(filepath.Join(root, "attempts"), journal.header(journal.floor)); err != nil {
			return fail(err)
		}
		if err := durableroot.SyncDirectory(root); err != nil {
			return fail(err)
		}
	} else if err := journal.load(); err != nil {
		return fail(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".attempts-") {
			if err := os.Remove(filepath.Join(root, entry.Name())); err != nil {
				return fail(err)
			}
		}
	}
	journal.identity, err = pinJournalFile(root)
	if err != nil {
		return fail(err)
	}
	return journal, nil
}

func (journal *Journal) header(floor time.Time) []byte {
	raw := append([]byte(journalMagic), journal.network[:]...)
	return binary.BigEndian.AppendUint64(raw, uint64(floor.Unix()))
}

func (journal *Journal) load() error {
	file, err := os.Open(filepath.Join(journal.root, "attempts"))
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(file, journalHeader+maximumAttempts*attemptSize+1))
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if len(raw) < journalHeader || len(raw) > journalHeader+maximumAttempts*attemptSize ||
		(len(raw)-journalHeader)%attemptSize != 0 || string(raw[:8]) != journalMagic || !bytes.Equal(raw[8:40], journal.network[:]) {
		return errors.New("text token journal is incomplete or foreign")
	}
	journal.floor = time.Unix(int64(binary.BigEndian.Uint64(raw[40:48])), 0).UTC()
	for offset := journalHeader; offset < len(raw); offset += attemptSize {
		record, err := decodeAttempt(raw[offset : offset+attemptSize])
		if err != nil {
			return err
		}
		if _, exists := journal.records[record.digest]; exists {
			return errors.New("text token journal duplicate")
		}
		journal.records[record.digest] = record
		if record.observed.After(journal.floor) {
			journal.floor = record.observed
		}
	}
	if journal.floor.IsZero() || journal.clock().UTC().Before(journal.floor) {
		return errors.New("text token journal time regressed")
	}
	return nil
}

func (journal *Journal) replace(records map[[32]byte]Attempt, floor time.Time) error {
	raw := journal.header(floor)
	keys := make([][32]byte, 0, len(records))
	for hash := range records {
		keys = append(keys, hash)
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i][:], keys[j][:]) < 0 })
	for _, hash := range keys {
		raw = append(raw, encodeAttempt(records[hash])...)
	}
	file, err := os.CreateTemp(journal.root, ".attempts-")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if err = file.Chmod(0o600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err := os.Rename(path, filepath.Join(journal.root, "attempts")); err != nil {
		return err
	}
	if err := durableroot.SyncDirectory(journal.root); err != nil {
		return err
	}
	info, err := pinJournalFile(journal.root)
	if err != nil {
		return err
	}
	journal.identity = info
	return nil
}

func writeJournalFile(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

// File.Stat pins Windows file identity now; pathname Stat can resolve its
// identity lazily only after the pathname has already been replaced.
func pinJournalFile(root string) (os.FileInfo, error) {
	file, err := os.Open(filepath.Join(root, "attempts"))
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	return info, errors.Join(err, file.Close())
}
