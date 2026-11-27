package custody

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	recordPrefix        = "record-"
	recordSuffix        = ".json"
	recordIDBytes       = 16
	maximumVaultRecords = 1024
	maximumVaultBytes   = 1 << 30
)

// recordPath uses the same filename grammar for creation and reopening.
func recordPath(directory, recordID string) string {
	return filepath.Join(directory, recordPrefix+recordID+recordSuffix)
}

func (vault *Vault) publishActiveRecord(recordID string, body []byte) error {
	return vault.publishRecord(vault.records, recordID, body)
}

func (vault *Vault) publishQuarantineRecord(recordID string, body []byte) error {
	return vault.publishRecord(vault.quarantine, recordID, body)
}

func (vault *Vault) publishRecord(directory, recordID string, body []byte) (resultErr error) {
	if err := vault.check(); err != nil {
		return err
	}
	path := recordPath(directory, recordID)
	if err := vault.reserveRecordSpace(int64(len(body))); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".record-")
	if err != nil {
		return fmt.Errorf("create encrypted vault temporary: %w", err)
	}
	temporary := file.Name()
	defer func() {
		if err := os.Remove(temporary); err != nil && !os.IsNotExist(err) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		err = errors.Join(err, file.Close())
		return fmt.Errorf("protect vault temporary: %w", err)
	}
	if _, err := file.Write(body); err != nil {
		err = errors.Join(err, file.Close())
		return fmt.Errorf("write vault record: %w", err)
	}
	if err := file.Sync(); err != nil {
		err = errors.Join(err, file.Close())
		return fmt.Errorf("flush vault record: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close vault record: %w", err)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("vault record collision: %w", ErrInvalid)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect vault record destination: %w", err)
	}
	if err := vault.check(); err != nil {
		return err
	}
	if err := os.Link(temporary, path); err != nil {
		return fmt.Errorf("publish vault record: %w", err)
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("flush vault record directory: %w", err)
	}
	if err := verifyPersistedEnvelope(path, body); err != nil {
		return err
	}
	return nil
}

// verifyPersistedEnvelope confirms that the completed encrypted bytes were
// published unchanged and still meet the canonical envelope profile. It does
// not run another KDF: one custody operation has one explicit password attempt.
func verifyPersistedEnvelope(path string, expected []byte) error {
	persisted, err := readEnvelopeFile(path)
	if err != nil {
		return fmt.Errorf("reopen encrypted vault record: %w", err)
	}
	defer clear(persisted)
	if !bytes.Equal(persisted, expected) {
		return ErrInvalid
	}
	if _, err := inspectEnvelope(persisted); err != nil {
		return err
	}
	return nil
}

func (vault *Vault) reserveRecordSpace(incoming int64) error {
	var count int
	var total int64
	for _, directory := range []string{vault.records, vault.quarantine} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("list vault records: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !validRecordFilename(entry.Name()) {
				return fmt.Errorf("vault record root: %w", ErrInvalid)
			}
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("inspect vault record: %w", err)
			}
			count++
			total += info.Size()
		}
	}
	if count >= maximumVaultRecords || incoming > maximumVaultBytes || total > maximumVaultBytes-incoming {
		return ErrInvalid
	}
	return nil
}

func validRecordFilename(value string) bool {
	if len(value) != len(recordPrefix)+2*recordIDBytes+len(recordSuffix) || !strings.HasPrefix(value, recordPrefix) || !strings.HasSuffix(value, recordSuffix) {
		return false
	}
	return validRecordID(value[len(recordPrefix) : len(value)-len(recordSuffix)])
}

func readEnvelopeFile(path string) ([]byte, error) {
	return readRegularFile(path, maximumEnvelopeBytes)
}

// readRegularFile excludes directories, symlinks, devices, and other special
// objects before bounded reading. Platform handle/reparse resistance remains a
// separate qualification obligation; this is the portable lexical admission
// guard used by every custody file reader.
func readRegularFile(path string, maximum uint64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(body)) > maximum {
		clear(body)
		return nil, ErrInvalid
	}
	return body, nil
}

func freshRecordID() (string, error) {
	bytes := make([]byte, recordIDBytes)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", fmt.Errorf("record identifier: %w", err)
	}
	defer clear(bytes)
	return hex.EncodeToString(bytes), nil
}

func validRecordID(value string) bool {
	if len(value) != 2*recordIDBytes {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}
