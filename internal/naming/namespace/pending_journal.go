package namespace

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	pendingEntrySchema    uint16 = 1
	maximumPendingEntries        = 127
	maximumPendingBytes          = 64 << 10
	pendingDomain                = "ardents-namespace-pending-v1"
)

// pendingEntry is Namespace's private durable bridge between a submitted
// control operation and a later threshold materialization.  Its immutable
// bytes bind the canonical submission, Authority-signed successor and the
// Gateway decision time; callers never receive or construct this state.
type pendingEntry struct {
	name       string
	sequence   uint64
	decisionAt int64
	submission []byte
	successor  []byte
}

func (entry pendingEntry) digest() [32]byte {
	out := append([]byte(pendingDomain+"\x00"), byte(pendingEntrySchema>>8), byte(pendingEntrySchema))
	out = binary.BigEndian.AppendUint64(out, entry.sequence)
	out = binary.BigEndian.AppendUint64(out, uint64(entry.decisionAt))
	out = binary.BigEndian.AppendUint32(out, uint32(len(entry.submission)))
	out = append(out, entry.submission...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(entry.successor)))
	out = append(out, entry.successor...)
	return sha256.Sum256(out)
}

func (entry pendingEntry) encode() ([]byte, error) {
	if entry.sequence == 0 || entry.decisionAt <= 0 || len(entry.submission) == 0 ||
		len(entry.successor) == 0 || len(entry.submission) > maximumPendingBytes ||
		len(entry.successor) > maximumPendingBytes {
		return nil, errors.New("naming pending entry is invalid")
	}
	out := make([]byte, 0, 2+8+8+8+len(entry.submission)+len(entry.successor))
	out = binary.BigEndian.AppendUint16(out, pendingEntrySchema)
	out = binary.BigEndian.AppendUint64(out, entry.sequence)
	out = binary.BigEndian.AppendUint64(out, uint64(entry.decisionAt))
	out = binary.BigEndian.AppendUint32(out, uint32(len(entry.submission)))
	out = append(out, entry.submission...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(entry.successor)))
	out = append(out, entry.successor...)
	return out, nil
}

func decodePendingEntry(raw []byte) (pendingEntry, error) {
	if len(raw) < 2+8+8+4+4 || len(raw) > maximumPendingBytes*2+64 ||
		binary.BigEndian.Uint16(raw[:2]) != pendingEntrySchema {
		return pendingEntry{}, errors.New("naming pending entry is invalid")
	}
	entry := pendingEntry{sequence: binary.BigEndian.Uint64(raw[2:10]), decisionAt: int64(binary.BigEndian.Uint64(raw[10:18]))}
	offset := 18
	read := func() ([]byte, bool) {
		if len(raw)-offset < 4 {
			return nil, false
		}
		length := int(binary.BigEndian.Uint32(raw[offset : offset+4]))
		offset += 4
		if length <= 0 || length > maximumPendingBytes || len(raw)-offset < length {
			return nil, false
		}
		value := append([]byte(nil), raw[offset:offset+length]...)
		offset += length
		return value, true
	}
	var ok bool
	if entry.submission, ok = read(); !ok {
		return pendingEntry{}, errors.New("naming pending entry is invalid")
	}
	if entry.successor, ok = read(); !ok || offset != len(raw) {
		return pendingEntry{}, errors.New("naming pending entry is invalid")
	}
	if _, err := entry.encode(); err != nil {
		return pendingEntry{}, err
	}
	return entry, nil
}

func (store *Store) appendPending(submission, successor []byte, decisionAt int64) (pendingEntry, error) {
	if store == nil || store.root == nil {
		return pendingEntry{}, errors.New("naming state store is unavailable")
	}
	operation, operationErr := decodeControlOperation(submission)
	record, recordErr := VerifyRecord(store.policy.Network, successor)
	if decisionAt <= 0 || operationErr != nil || recordErr != nil || operation.Name != record.Name {
		return pendingEntry{}, errors.New("naming pending submission is invalid")
	}
	return store.root.appendPending(pendingEntry{decisionAt: decisionAt,
		submission: append([]byte(nil), submission...), successor: append([]byte(nil), successor...)})
}

func (store *Store) pending() ([]pendingEntry, error) {
	if store == nil || store.root == nil {
		return nil, errors.New("naming state store is unavailable")
	}
	entries, err := store.root.pending()
	if err != nil {
		return nil, err
	}
	for index, entry := range entries {
		operation, operationErr := decodeControlOperation(entry.submission)
		record, recordErr := VerifyRecord(store.policy.Network, entry.successor)
		if operationErr != nil || recordErr != nil || operation.Name != record.Name {
			return nil, errors.New("naming pending journal is tampered")
		}
		entries[index].submission = append([]byte(nil), entry.submission...)
		entries[index].successor = append([]byte(nil), entry.successor...)
	}
	return entries, nil
}

// pendingCursorFor admits a materialization only when its complete signed
// corpus is exactly the next prefix of the one durable pending chain. A caller
// cannot splice an independently supplied Record into current state.
func (store *Store) pendingCursorFor(records [][]byte) (uint64, error) {
	current, _, rootErr := store.root.load()
	if rootErr != nil {
		return 0, errors.New("naming state is tampered")
	}
	baseline := make(map[string][]byte)
	cursor := uint64(0)
	if current != "" {
		snapshot, loadErr := store.load(0)
		if loadErr != nil {
			return 0, loadErr
		}
		cursor = snapshot.pending
		for _, signed := range snapshot.records {
			record, verifyErr := VerifyRecord(store.policy.Network, signed)
			if verifyErr != nil {
				return 0, errors.New("naming current record is invalid")
			}
			baseline[record.Name] = signed
		}
	}
	entries, err := store.pending()
	if err != nil {
		return 0, err
	}
	if cursor > uint64(len(entries)) {
		return 0, errors.New("naming state pending cursor is invalid")
	}
	if len(entries) == 0 {
		return 0, nil
	}
	candidate, mapErr := signedRecordMap(store.policy.Network, records)
	if mapErr != nil {
		return 0, mapErr
	}
	for index := cursor; index < uint64(len(entries)); index++ {
		entry := entries[index]
		record, verifyErr := VerifyRecord(store.policy.Network, entry.successor)
		if verifyErr != nil {
			return 0, errors.New("naming pending journal is tampered")
		}
		baseline[record.Name] = entry.successor
		if sameSignedRecordMap(baseline, candidate) {
			return entry.sequence, nil
		}
	}
	return 0, errors.New("naming materialization is not selected from pending state")
}

func signedRecordMap(network [32]byte, signed [][]byte) (map[string][]byte, error) {
	values := make(map[string][]byte, len(signed))
	for _, raw := range signed {
		record, err := VerifyRecord(network, raw)
		if err != nil || values[record.Name] != nil {
			return nil, errors.New("naming materialization Record corpus is invalid")
		}
		values[record.Name] = raw
	}
	return values, nil
}

func sameSignedRecordMap(left, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for name, value := range left {
		if !bytes.Equal(value, right[name]) {
			return false
		}
	}
	return true
}

func (root *namespaceRoot) appendPending(entry pendingEntry) (pendingEntry, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return pendingEntry{}, err
	}
	entries, err := loadNamespacePending(filepath.Join(root.path, "distribution", "generations"))
	if err != nil {
		return pendingEntry{}, err
	}
	if len(entries) >= maximumPendingEntries {
		return pendingEntry{}, errors.New("naming pending journal is full")
	}
	entry.sequence = uint64(len(entries) + 1)
	if _, err := entry.encode(); err != nil {
		return pendingEntry{}, err
	}
	digest := entry.digest()
	entry.name = hex.EncodeToString(digest[:])
	if err := writeNamespacePending(filepath.Join(root.path, "distribution", "generations"), entry); err != nil {
		return pendingEntry{}, err
	}
	return entry, nil
}

func (root *namespaceRoot) pending() ([]pendingEntry, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return nil, err
	}
	return loadNamespacePending(filepath.Join(root.path, "distribution", "generations"))
}

func loadNamespacePending(directory string) ([]pendingEntry, error) {
	entries, err := readNamespaceDirectory(directory, maximumPendingEntries)
	if err != nil {
		return nil, fmt.Errorf("scan naming pending journal: %w", err)
	}
	values := make([]pendingEntry, 0, len(entries))
	for _, item := range entries {
		if !item.IsDir() || !namespaceGenerationName.MatchString(item.Name()) {
			return nil, errors.New("naming pending journal directory is invalid")
		}
		raw, readErr := readNamespaceFile(filepath.Join(directory, item.Name(), "entry.bin"), maximumPendingBytes*2+64)
		if readErr != nil {
			return nil, fmt.Errorf("read naming pending entry: %w", readErr)
		}
		entry, decodeErr := decodePendingEntry(raw)
		if decodeErr != nil {
			return nil, decodeErr
		}
		digest := entry.digest()
		if hex.EncodeToString(digest[:]) != item.Name() {
			return nil, errors.New("naming pending entry digest is invalid")
		}
		entry.name = item.Name()
		values = append(values, entry)
	}
	sort.Slice(values, func(first, second int) bool { return values[first].sequence < values[second].sequence })
	for index, entry := range values {
		if entry.sequence != uint64(index+1) {
			return nil, errors.New("naming pending journal sequence is invalid")
		}
	}
	return values, nil
}

func writeNamespacePending(directory string, entry pendingEntry) error {
	raw, err := entry.encode()
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp(directory, ".stage-")
	if err != nil {
		return fmt.Errorf("create naming pending staging: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeNamespaceFile(filepath.Join(staging, "entry.bin"), raw); err != nil {
		return err
	}
	if err := syncNamespaceDirectory(staging); err != nil {
		return err
	}
	final := filepath.Join(directory, entry.name)
	if info, statErr := os.Stat(final); statErr == nil {
		persisted, readErr := readNamespaceFile(filepath.Join(final, "entry.bin"), maximumPendingBytes*2+64)
		if !info.IsDir() || readErr != nil || !bytes.Equal(persisted, raw) {
			return errors.New("existing naming pending entry disagrees with supplied bytes")
		}
		if err := os.RemoveAll(staging); err != nil {
			return err
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	} else if err := os.Rename(staging, final); err != nil {
		return fmt.Errorf("publish naming pending entry: %w", err)
	}
	committed = true
	return syncNamespaceDirectory(directory)
}
