//go:build linux

package endpoint

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
)

const textTokenJournalMarker = "ardents-token-attempts-v1\n"

func openTextTokenJournal(root string, network [32]byte, clock func() time.Time) (*textTokenJournal, error) {
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
		if err != nil || string(marker) != textTokenJournalMarker {
			return nil, errors.New("text token journal marker unavailable")
		}
	}
	if err := secureTransitAcquisitionRoot(root, info); err != nil {
		return nil, err
	}
	lease, err := acquireTransitAcquisitionLease(filepath.Join(root, "owner.lock"))
	if err != nil {
		return nil, err
	}
	journal := &textTokenJournal{root: root, network: network, clock: clock, lease: lease, records: make(map[[32]byte]textTokenAttempt)}
	fail := func(cause error) (*textTokenJournal, error) { return nil, errors.Join(cause, lease.release()) }
	if fresh {
		current, err := os.ReadDir(root)
		if err != nil || len(current) != 1 || current[0].Name() != "owner.lock" {
			return fail(errors.New("text token journal changed before claim"))
		}
		if err := writeTextTokenJournalFile(filepath.Join(root, "root.marker"), []byte(textTokenJournalMarker)); err != nil {
			return fail(err)
		}
		journal.floor = clock().UTC().Truncate(time.Second)
		if err := writeTextTokenJournalFile(filepath.Join(root, "attempts"), journal.header(journal.floor)); err != nil {
			return fail(err)
		}
		if err := endpointSyncDirectory(root); err != nil {
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
	journal.identity, err = pinTextTokenJournalFile(root)
	if err != nil {
		return fail(err)
	}
	return journal, nil
}

func (journal *textTokenJournal) header(floor time.Time) []byte {
	raw := append([]byte(textTokenJournalMagic), journal.network[:]...)
	return binary.BigEndian.AppendUint64(raw, uint64(floor.Unix()))
}

func (journal *textTokenJournal) load() error {
	file, err := os.Open(filepath.Join(journal.root, "attempts"))
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(file, textTokenJournalHeader+maximumTextTokenAttempts*textTokenAttemptSize+1))
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if len(raw) < textTokenJournalHeader || len(raw) > textTokenJournalHeader+maximumTextTokenAttempts*textTokenAttemptSize ||
		(len(raw)-textTokenJournalHeader)%textTokenAttemptSize != 0 || string(raw[:8]) != textTokenJournalMagic || !bytes.Equal(raw[8:40], journal.network[:]) {
		return errors.New("text token journal is incomplete or foreign")
	}
	journal.floor = time.Unix(int64(binary.BigEndian.Uint64(raw[40:48])), 0).UTC()
	for offset := textTokenJournalHeader; offset < len(raw); offset += textTokenAttemptSize {
		record, err := decodeTextTokenAttempt(raw[offset : offset+textTokenAttemptSize])
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

func (journal *textTokenJournal) replace(records map[[32]byte]textTokenAttempt, floor time.Time) error {
	raw := journal.header(floor)
	keys := make([][32]byte, 0, len(records))
	for hash := range records {
		keys = append(keys, hash)
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i][:], keys[j][:]) < 0 })
	for _, hash := range keys {
		raw = append(raw, encodeTextTokenAttempt(records[hash])...)
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
	if err := endpointSyncDirectory(journal.root); err != nil {
		return err
	}
	info, err := pinTextTokenJournalFile(journal.root)
	if err != nil {
		return err
	}
	journal.identity = info
	return nil
}

func writeTextTokenJournalFile(path string, raw []byte) error {
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
func pinTextTokenJournalFile(root string) (os.FileInfo, error) {
	file, err := os.Open(filepath.Join(root, "attempts"))
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	return info, errors.Join(err, file.Close())
}
