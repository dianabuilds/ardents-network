package reachability

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const storeRecordVersion = byte(1)

func (store *Store) restore() error {
	directory := filepath.Join(store.path, storeRecords)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read reachability store records: %w", err)
	}
	if len(entries) > maximumTargets {
		return errors.New("reachability store exceeds Target bound")
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) != 64 {
			return errors.New("reachability store record name is invalid")
		}
		raw, err := readStoreFile(filepath.Join(directory, entry.Name()), MaximumDescriptorSize+4)
		if err != nil {
			return err
		}
		record, err := decodeStored(raw, store.network)
		if err != nil || targetName(record.verified.Descriptor.Target) != entry.Name() {
			return errors.New("reachability store record is invalid")
		}
		if _, duplicate := store.records[record.verified.Descriptor.Target]; duplicate {
			return errors.New("reachability store contains duplicate Target")
		}
		store.records[record.verified.Descriptor.Target] = record
	}
	return nil
}

func (store *Store) write(record storedDescriptor) error {
	raw, err := encodeStored(record)
	if err != nil {
		return err
	}
	return replaceStoreFile(filepath.Join(store.path, storeRecords), targetName(record.verified.Descriptor.Target), raw)
}

func encodeStored(record storedDescriptor) ([]byte, error) {
	if len(record.raw) == 0 || len(record.raw) > MaximumDescriptorSize || record.verified.Descriptor.Target == [32]byte{} {
		return nil, errors.New("reachability stored descriptor is incomplete")
	}
	flags := byte(0)
	if record.conflicting {
		flags = 1
	}
	return append([]byte{storeRecordVersion, flags}, record.raw...), nil
}

func decodeStored(raw []byte, network [32]byte) (storedDescriptor, error) {
	if len(raw) < 2 || raw[0] != storeRecordVersion || raw[1] > 1 {
		return storedDescriptor{}, errors.New("reachability stored descriptor header is invalid")
	}
	descriptor, _, err := decode(raw[2:])
	if err != nil {
		return storedDescriptor{}, err
	}
	// A stored descriptor may be expired now, but it was valid when accepted.
	// Its own whole-second slot expiry is within the Credential interval, so one
	// second before it is the stable proof decision time after publication.
	at := descriptor.Introduction.NotAfter.Add(-time.Second)
	current, err := publication.Decode(descriptor.Publication, ed25519.PublicKey(descriptor.AuthorityPublic[:]), network, at)
	if err != nil || current.Credential.Target != descriptor.Target || current.Digest != descriptor.PublicationDigest {
		return storedDescriptor{}, errors.New("reachability stored Publication is invalid")
	}
	verified, err := Verify(raw[2:], descriptor.Target, network, at)
	if err != nil {
		return storedDescriptor{}, err
	}
	return storedDescriptor{raw: append([]byte(nil), raw[2:]...), verified: verified, digest: sha256.Sum256(raw[2:]), conflicting: raw[1] == 1}, nil
}

func targetName(target [32]byte) string { return fmt.Sprintf("%x", target) }

func prepareStoreRoot(root string) error {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return fmt.Errorf("create reachability store root: %w", err)
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("reachability store root is invalid")
	}
	if err := ensureStoreFile(filepath.Join(root, storeLockName), nil); err != nil {
		return err
	}
	if err := ensureStoreFile(filepath.Join(root, storeMarkerName), []byte(storeMarker)); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(root, storeRecords), 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

func ensureStoreFile(path string, expected []byte) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return createErr
		}
		if len(expected) > 0 {
			_, createErr = file.Write(expected)
		}
		if createErr == nil {
			createErr = file.Sync()
		}
		return errors.Join(createErr, file.Close())
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("reachability store root file is invalid")
	}
	if len(expected) == 0 {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, expected) {
		return errors.New("reachability store marker is invalid")
	}
	return nil
}

func readStoreFile(path string, maximum int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > int64(maximum) {
		return nil, errors.New("reachability store record is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(raw) == 0 || len(raw) > maximum {
		return nil, errors.New("read reachability store record")
	}
	return raw, nil
}

func replaceStoreFile(root, name string, raw []byte) error {
	file, err := os.CreateTemp(root, ".record-")
	if err != nil {
		return err
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()
	if err = file.Chmod(0o600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(path, filepath.Join(root, name))
	}
	return err
}
