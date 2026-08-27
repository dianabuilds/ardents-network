package replacement

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	markerName   = ".ardents-endpoint-replacement-v1"
	markerValue  = "ardents-endpoint-replacement-v1\n"
	currentName  = "current"
	preparedName = "prepared"
	journalName  = "journal"
	rollbackName = "rollback-program"
	maximumText  = 4096
)

var errStoreAbsent = errors.New("endpoint replacement state is absent")

type store struct {
	root string
	lock *os.File
}

func openStore(root string, create bool) (*store, error) {
	if root == "" {
		return nil, errors.New("endpoint replacement state root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve Endpoint replacement state root: %w", err)
	}
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		if !create {
			return nil, errStoreAbsent
		}
		if err := os.MkdirAll(abs, 0o700); err != nil {
			return nil, fmt.Errorf("create Endpoint replacement state root: %w", err)
		}
		info, err = os.Lstat(abs)
	}
	if err != nil {
		return nil, fmt.Errorf("inspect Endpoint replacement state root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("endpoint replacement state root is not a direct directory")
	}
	lock, err := acquireLock(abs)
	if err != nil {
		return nil, err
	}
	store := &store{root: abs, lock: lock}
	if err := validateRoot(abs, create); err != nil {
		return nil, errors.Join(err, store.close())
	}
	return store, nil
}

// openReadStore validates an existing replacement root without acquiring its
// writer lease. The foreground transaction intentionally holds that lease
// while it launches the candidate self-test; the candidate can therefore read
// its immutable prepared record without competing for the writer lock. Read
// callers never create, repair, or delete any state.
func openReadStore(root string) (*store, error) {
	if root == "" {
		return nil, errors.New("endpoint replacement state root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve Endpoint replacement state root: %w", err)
	}
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errStoreAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("inspect Endpoint replacement state root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("endpoint replacement state root is not a direct directory")
	}
	if err := validateRoot(abs, false); err != nil {
		return nil, err
	}
	return &store{root: abs}, nil
}

func (store *store) close() error {
	if store == nil || store.lock == nil {
		return nil
	}
	err := releaseLock(store.lock)
	store.lock = nil
	return err
}

func validateRoot(root string, create bool) error {
	marker := filepath.Join(root, markerName)
	info, err := os.Lstat(marker)
	if errors.Is(err, os.ErrNotExist) {
		entries, readErr := os.ReadDir(root)
		if readErr != nil {
			return readErr
		}
		if len(entries) != 1 || entries[0].Name() != ".lock" || !create {
			return errors.New("endpoint replacement state root is unowned")
		}
		return writeAtomic(root, markerName, []byte(markerValue))
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("endpoint replacement state marker is invalid")
	}
	value, err := os.ReadFile(marker)
	if err != nil || string(value) != markerValue {
		return errors.New("endpoint replacement state marker is invalid")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != markerName && entry.Name() != currentName && entry.Name() != preparedName && entry.Name() != journalName && entry.Name() != rollbackName && entry.Name() != ".lock" &&
			!strings.HasPrefix(entry.Name(), ".current-") && !strings.HasPrefix(entry.Name(), ".prepared-") {
			return errors.New("endpoint replacement state has an unknown entry")
		}
	}
	return nil
}

func (store *store) current() (Record, error) {
	raw, err := readDirectFile(filepath.Join(store.root, currentName), maximumText)
	if err != nil {
		return Record{}, fmt.Errorf("read Endpoint replacement current record: %w", err)
	}
	return decodeRecord(raw)
}

func (store *store) prepared() (Record, error) {
	raw, err := readDirectFile(filepath.Join(store.root, preparedName), maximumText)
	if err != nil {
		return Record{}, fmt.Errorf("read Endpoint replacement prepared record: %w", err)
	}
	return decodeRecord(raw)
}

func (store *store) prepare(record Record) error {
	if journalRecord, err := store.readJournal(); err == nil {
		if journalRecord.phase != "committed" && journalRecord.phase != "rollback-committed" {
			return errors.New("endpoint replacement has an incomplete journal")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if existing, err := store.prepared(); err == nil {
		if existing != record {
			return errors.New("endpoint replacement already has a different prepared successor")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	encoded, err := encodeRecord(record)
	if err != nil {
		return err
	}
	return writeAtomic(store.root, preparedName, encoded)
}

// replacePreparedForRollback substitutes a failed candidate's prepared record
// only after its digest was authenticated against the journal. The durable
// current record remains untouched until the restored predecessor has passed
// its own candidate-side self-test and CommitPrepared succeeds.
func (store *store) replacePreparedForRollback(record Record, failedCandidate [sha256.Size]byte) error {
	existing, err := store.prepared()
	if err != nil {
		return fmt.Errorf("read failed Endpoint replacement candidate: %w", err)
	}
	if existing.Digest != failedCandidate {
		return errors.New("endpoint replacement prepared record does not match failed candidate")
	}
	if err := os.Remove(filepath.Join(store.root, preparedName)); err != nil {
		return err
	}
	if err := syncDirectory(store.root); err != nil {
		return err
	}
	encoded, err := encodeRecord(record)
	if err != nil {
		return err
	}
	return writeAtomic(store.root, preparedName, encoded)
}

// retireCompletedRollback releases only the previous transaction's immediate
// predecessor reserve. A new authorized successor may replace a current
// program only after the old reserve and its journal prove one completed
// transaction for that exact current program; failed/interrupted evidence is
// never discarded by the next replacement attempt.
func (store *store) retireCompletedRollback(current Record, programPath string) error {
	journalRecord, err := store.readJournal()
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Lstat(filepath.Join(store.root, rollbackName)); errors.Is(statErr, os.ErrNotExist) {
			return nil
		} else if statErr != nil {
			return statErr
		}
		return errors.New("endpoint replacement retains a predecessor without a completed journal")
	}
	if err != nil {
		return err
	}
	if (journalRecord.phase != "committed" && journalRecord.phase != "rollback-committed") ||
		journalRecord.candidate != current.Digest || !sameProgramPath(programPath, journalRecord.programPath) {
		return errors.New("endpoint replacement has incomplete predecessor retention")
	}
	rollbackPath := filepath.Join(store.root, rollbackName)
	if retained, readErr := readProgram(rollbackPath); readErr == nil {
		digest := sha256.Sum256(retained)
		if digest != journalRecord.predecessor {
			return errors.New("endpoint replacement retained predecessor does not match completed journal")
		}
		if removeErr := os.Remove(rollbackPath); removeErr != nil {
			return removeErr
		}
		if syncErr := syncDirectory(store.root); syncErr != nil {
			return syncErr
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if err := os.Remove(filepath.Join(store.root, journalName)); err != nil {
		return err
	}
	return syncDirectory(store.root)
}

func (store *store) commitPrepared(record Record) error {
	encoded, err := encodeRecord(record)
	if err != nil {
		return err
	}
	if err := writeAtomic(store.root, currentName, encoded); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(store.root, preparedName)); err != nil {
		return err
	}
	directory, err := os.Open(store.root)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	return errors.Join(err, closeErr)
}

func readProgram(path string) ([]byte, error) {
	return readDirectFile(path, maximumProgramBytes)
}

func readDirectFile(path string, maximum int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > int64(maximum) {
		return nil, errors.New("file is not a bounded direct regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil || len(contents) == 0 || len(contents) > maximum {
		return nil, errors.New("file cannot be read within its declared bound")
	}
	return contents, nil
}

func writeAtomic(root, name string, contents []byte) error {
	return writeAtomicMode(root, name, contents, 0o600)
}

func writeExecutableAtomic(root, name string, contents []byte) error {
	return writeAtomicMode(root, name, contents, 0o700)
}

func writeAtomicMode(root, name string, contents []byte, mode os.FileMode) error {
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	temporary := filepath.Join(root, "."+name+"-"+hex.EncodeToString(token[:])+".tmp")
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, filepath.Join(root, name)); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	return errors.Join(err, closeErr)
}

func encodeRecord(record Record) ([]byte, error) {
	values := []string{record.TargetPath, strconv.FormatInt(record.Length, 10), hex.EncodeToString(record.Digest[:]),
		record.Platform, record.Architecture, record.Environment, record.Network,
		record.ReleaseID, strconv.FormatInt(record.ReleaseVersion, 10)}
	for _, value := range values {
		if value == "" || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("endpoint replacement record is incomplete")
		}
	}
	if record.Length < 1 || record.ReleaseVersion < 1 {
		return nil, errors.New("endpoint replacement record is invalid")
	}
	keys := []string{"schema", "target_path", "length", "digest", "platform", "architecture", "environment", "network", "release_id", "release_version"}
	values = append([]string{"ardents-endpoint-replacement-v1"}, values...)
	var builder strings.Builder
	for index, key := range keys {
		builder.WriteString(key + "=" + values[index] + "\n")
	}
	return []byte(builder.String()), nil
}

func decodeRecord(raw []byte) (Record, error) {
	keys := []string{"schema", "target_path", "length", "digest", "platform", "architecture", "environment", "network", "release_id", "release_version"}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(raw) == 0 || len(raw) > maximumText || raw[len(raw)-1] != '\n' || len(lines) != len(keys) {
		return Record{}, errors.New("endpoint replacement current record is not canonical")
	}
	values := make([]string, len(keys))
	for index, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || parts[0] != keys[index] || parts[1] == "" {
			return Record{}, errors.New("endpoint replacement current record is not canonical")
		}
		values[index] = parts[1]
	}
	if values[0] != "ardents-endpoint-replacement-v1" {
		return Record{}, errors.New("endpoint replacement current record schema is invalid")
	}
	length, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil || length < 1 || length > maximumProgramBytes {
		return Record{}, errors.New("endpoint replacement current record length is invalid")
	}
	digest, err := hex.DecodeString(values[3])
	if err != nil || len(digest) != 32 || strings.ToLower(values[3]) != values[3] {
		return Record{}, errors.New("endpoint replacement current record digest is invalid")
	}
	version, err := strconv.ParseInt(values[9], 10, 64)
	if err != nil || version < 1 {
		return Record{}, errors.New("endpoint replacement current record release version is invalid")
	}
	for _, value := range append(values[1:3], values[4:9]...) {
		if strings.ContainsAny(value, "\r\n") {
			return Record{}, errors.New("endpoint replacement current record is invalid")
		}
	}
	var sum [32]byte
	copy(sum[:], digest)
	return Record{TargetPath: values[1], Length: length, Digest: sum, Platform: values[4], Architecture: values[5],
		Environment: values[6], Network: values[7], ReleaseID: values[8], ReleaseVersion: version}, nil
}
