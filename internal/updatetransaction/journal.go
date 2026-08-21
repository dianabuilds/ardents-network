package updatetransaction

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type transactionState byte

const (
	stateReleaseAccepted transactionState = 1 + iota
	stateArtifactVerified
	stateStaged
	stateRollbackReserved
	stateStopNewWork
	stateDraining
	stateActivated
	stateSelfTesting
	stateCommitted
)

type adapterResult byte

const (
	adapterNotCalled adapterResult = iota
	adapterSuccess
	adapterBusy
	adapterUnavailable
	adapterFailed
)

type journalEntry struct {
	State                                           transactionState
	Generation, ElapsedNanos                        uint64
	Predecessor, ArtifactDigest, ManifestCommitment [32]byte
	AdapterResult                                   adapterResult
	Observation                                     byte
	DeadlineUnix                                    int64
}

type inspectedTuple struct {
	Generation, Length uint64
	Artifact, Manifest [32]byte
}

type predecessorInspection struct {
	CurrentRecordDigest, ArtifactObservation, ManifestObservation [32]byte
	Current                                                       inspectedTuple
	Rollback                                                      *inspectedTuple
}

var errRecordInvalid, errJournalChain = errors.New("update transaction record is invalid"), errors.New("update transaction journal chain is invalid")

func encodeRecord(kind byte, body []byte, maximum int) ([]byte, error) {
	if kind < recordManifest || kind > recordJournal || len(body) > maximum ||
		uint64(len(body)) > uint64(^uint32(0)) {
		return nil, errRecordInvalid
	}
	record := make([]byte, recordHeaderBytes, recordHeaderBytes+len(body))
	copy(record[:8], "ARDUPD01")
	record[8] = kind
	record[9] = 1
	binary.BigEndian.PutUint32(record[12:16], uint32(len(body)))
	return append(record, body...), nil
}

func decodeRecord(raw []byte, kind byte, maximum int) ([]byte, error) {
	if len(raw) < recordHeaderBytes || len(raw) > recordHeaderBytes+maximum ||
		string(raw[:8]) != "ARDUPD01" || raw[8] != kind || raw[9] != 1 ||
		raw[10] != 0 || raw[11] != 0 ||
		int(binary.BigEndian.Uint32(raw[12:16])) != len(raw)-recordHeaderBytes {
		return nil, errRecordInvalid
	}
	return raw[recordHeaderBytes:], nil
}

func encodeJournalEntry(entry journalEntry) ([]byte, error) {
	if entry.State < stateReleaseAccepted || entry.State > stateCommitted ||
		entry.AdapterResult > adapterFailed || entry.Observation != byte(entry.State) ||
		entry.DeadlineUnix == 0 {
		return nil, errRecordInvalid
	}
	body := make([]byte, journalBodyBytes)
	body[0] = byte(entry.State)
	binary.BigEndian.PutUint64(body[1:9], entry.Generation)
	copy(body[9:41], entry.Predecessor[:])
	copy(body[41:73], entry.ArtifactDigest[:])
	copy(body[73:105], entry.ManifestCommitment[:])
	body[105] = byte(entry.AdapterResult)
	body[106] = entry.Observation
	binary.BigEndian.PutUint64(body[107:115], entry.ElapsedNanos)
	binary.BigEndian.PutUint64(body[115:123], uint64(entry.DeadlineUnix))
	return encodeRecord(recordJournal, body, maximumJournalBytes)
}

func decodeJournalEntry(raw []byte) (journalEntry, error) {
	var entry journalEntry
	body, err := decodeRecord(raw, recordJournal, maximumJournalBytes)
	if err != nil || len(body) != journalBodyBytes {
		return entry, errRecordInvalid
	}
	entry.State = transactionState(body[0])
	entry.Generation = binary.BigEndian.Uint64(body[1:9])
	copy(entry.Predecessor[:], body[9:41])
	copy(entry.ArtifactDigest[:], body[41:73])
	copy(entry.ManifestCommitment[:], body[73:105])
	entry.AdapterResult = adapterResult(body[105])
	entry.Observation = body[106]
	entry.ElapsedNanos = binary.BigEndian.Uint64(body[107:115])
	entry.DeadlineUnix = int64(binary.BigEndian.Uint64(body[115:123]))
	if entry.State < stateReleaseAccepted || entry.State > stateCommitted ||
		entry.AdapterResult > adapterFailed || entry.Observation != byte(entry.State) ||
		entry.DeadlineUnix == 0 {
		return journalEntry{}, errRecordInvalid
	}
	return entry, nil
}

func encodePredecessor(inspection predecessorInspection) ([]byte, error) {
	body := append([]byte(nil), inspection.CurrentRecordDigest[:]...)
	body = appendTuple(body, inspection.Current)
	if inspection.Rollback == nil {
		body = append(body, 0)
	} else {
		body = append(body, 1)
		body = appendTuple(body, *inspection.Rollback)
	}
	body = append(body, inspection.ArtifactObservation[:]...)
	body = append(body, inspection.ManifestObservation[:]...)
	return encodeRecord(recordPredecessor, body, 4096)
}

func appendTuple(body []byte, tuple inspectedTuple) []byte {
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], tuple.Generation)
	body = append(body, number[:]...)
	binary.BigEndian.PutUint64(number[:], tuple.Length)
	body = append(body, number[:]...)
	body = append(body, tuple.Artifact[:]...)
	return append(body, tuple.Manifest[:]...)
}

func journalFileName(state transactionState) (string, error) {
	names := [...]string{"", "01-release-accepted.entry", "02-artifact-verified.entry",
		"03-staged.entry", "04-rollback-reserved.entry", "05-stop-new-work.entry",
		"06-draining.entry", "07-activated.entry", "08-self-testing.entry",
		"09-committed.entry"}
	if int(state) >= len(names) || state == 0 {
		return "", errRecordInvalid
	}
	return names[state], nil
}

func (store *ownedStore) writeEntry(entry journalEntry) ([]byte, error) {
	raw, err := encodeJournalEntry(entry)
	if err != nil {
		return nil, err
	}
	name, err := journalFileName(entry.State)
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(store.generationPath("transactions", entry.Generation), "journal")
	if err := writeNewFile(filepath.Join(directory, name), raw); err != nil {
		return nil, err
	}
	return raw, store.ops.syncDirectory(directory)
}

func inspectJournal(directory string, generation uint64, artifact,
	manifest, firstPredecessor [32]byte) ([][]byte, error) {
	entries := make([][]byte, 0, 9)
	predecessor := firstPredecessor
	var elapsed uint64
	for state := stateReleaseAccepted; state <= stateCommitted; state++ {
		name, err := journalFileName(state)
		if err != nil {
			return nil, err
		}
		raw, err := readExactFile(filepath.Join(directory, name), journalRecordBytes)
		if err != nil {
			return nil, err
		}
		entry, err := decodeJournalEntry(raw)
		if err != nil || entry.State != state || entry.Generation != generation ||
			entry.ArtifactDigest != artifact || entry.ManifestCommitment != manifest ||
			entry.Predecessor != predecessor || entry.ElapsedNanos < elapsed {
			return nil, errJournalChain
		}
		entries = append(entries, raw)
		predecessor = sha256.Sum256(raw)
		elapsed = entry.ElapsedNanos
	}
	return entries, nil
}

func readExactFile(path string, size int) ([]byte, error) {
	if err := validateOwnedEntry(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, statErr := file.Stat()
	if statErr != nil || info.Size() != int64(size) || !info.Mode().IsRegular() {
		return nil, errors.Join(errRecordInvalid, statErr, file.Close())
	}
	data := make([]byte, size)
	_, readErr := io.ReadFull(file, data)
	return data, errors.Join(readErr, file.Close())
}

func readBoundedFile(path string, maximum int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > int64(maximum) {
		return nil, errors.Join(errRecordInvalid, err)
	}
	return readExactFile(path, int(info.Size()))
}

func writeNewFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(writeErr, file.Sync(), file.Close())
}
