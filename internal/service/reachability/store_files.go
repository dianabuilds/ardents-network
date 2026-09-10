package reachability

import (
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

const (
	storeRecordVersion        = byte(1)
	privateStoreRecordVersion = byte(2)
)

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
		raw, err := readStoreFile(filepath.Join(directory, entry.Name()), MaximumPrivateDescriptorSize+4)
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
	version, maximum := storeRecordVersion, MaximumDescriptorSize
	if record.verified.Descriptor.Version == privateDescriptorVersion {
		version, maximum = privateStoreRecordVersion, MaximumPrivateDescriptorSize
	}
	if len(record.raw) == 0 || len(record.raw) > maximum || record.verified.Descriptor.Target == [32]byte{} ||
		(version == storeRecordVersion && record.revisionConflicting) {
		return nil, errors.New("reachability stored descriptor is incomplete")
	}
	flags := byte(0)
	if record.conflicting {
		flags |= 1
	}
	if record.revisionConflicting {
		flags |= 2
	}
	return append([]byte{version, flags}, record.raw...), nil
}

func decodeStored(raw []byte, network [32]byte) (storedDescriptor, error) {
	if len(raw) < 2 || (raw[0] != storeRecordVersion && raw[0] != privateStoreRecordVersion) ||
		raw[1] > 3 || (raw[0] == storeRecordVersion && raw[1] > 1) {
		return storedDescriptor{}, errors.New("reachability stored descriptor header is invalid")
	}
	var descriptor Descriptor
	var err error
	if raw[0] == privateStoreRecordVersion {
		descriptor, err = decodePrivateDescriptor(raw[2:])
	} else {
		descriptor, _, err = decode(raw[2:])
	}
	if err != nil {
		return storedDescriptor{}, err
	}
	// Restore the signed floor at a time within its original validity, even
	// when it has since expired. This does not make it currently available:
	// lookup re-verifies against the caller's actual profile and time.
	at := descriptor.Introduction.NotAfter.Add(-time.Second)
	if descriptor.Version == privateDescriptorVersion {
		at = descriptor.Private.NotAfter.Add(-time.Second)
	}
	current, err := publication.Decode(descriptor.Publication, ed25519.PublicKey(descriptor.AuthorityPublic[:]), network, at)
	if err != nil || current.Credential.Target != descriptor.Target || current.Digest != descriptor.PublicationDigest {
		return storedDescriptor{}, errors.New("reachability stored Publication is invalid")
	}
	var verified Verified
	if descriptor.Version == privateDescriptorVersion {
		verified, err = VerifyPrivate(raw[2:], descriptor.Target, network, descriptor.ProfileDigest, at)
	} else {
		verified, err = Verify(raw[2:], descriptor.Target, network, at)
	}
	if err != nil {
		return storedDescriptor{}, err
	}
	return storedDescriptor{raw: append([]byte(nil), raw[2:]...), verified: verified, digest: sha256.Sum256(raw[2:]),
		conflicting: raw[1]&1 != 0, revisionConflicting: raw[1]&2 != 0}, nil
}
func targetName(target [32]byte) string { return fmt.Sprintf("%x", target) }

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
	if err == nil {
		err = syncStoreDirectory(root)
	}
	return err
}
