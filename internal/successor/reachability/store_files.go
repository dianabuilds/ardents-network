package reachability

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	storeMarkerName = ".ardents-reachability-store-v3"
	storeMarker     = "ardents-reachability-store-v3\n"
	storeLockName   = ".ardents-reachability-store-lock"
	storeRecords    = "records"
	// The single serialized writer owns this exact reserved staging name. Its
	// contents are never a committed proof, including a complete synced write.
	storeStage = ".record-stage"
)

func (store *Store) restore() error {
	root := filepath.Join(store.root, storeRecords)
	entries, err := boundedStoreEntries(root, MaximumTargets+1)
	if err != nil {
		return err
	}
	stage := false
	for _, entry := range entries {
		if entry.Name() == storeStage {
			if stage || validateStoreStage(filepath.Join(root, storeStage)) != nil {
				return errors.New("reachability staging invalid")
			}
			stage = true
			continue
		}
		if len(store.floors) >= MaximumTargets || len(entry.Name()) != 64 {
			return errors.New("reachability record population or name invalid")
		}
		raw, err := readStoreFile(filepath.Join(root, entry.Name()), MaximumDescriptorSize+2)
		if err != nil {
			return err
		}
		floor, err := decodeStoredFloor(raw, store.network)
		if err != nil {
			return err
		}
		target := floor.descriptor.Target
		if entry.Name() != storeTargetName(target) {
			return errors.New("reachability retained Target name differs")
		}
		if _, exists := store.floors[target]; exists {
			return errors.New("reachability duplicate Target")
		}
		store.floors[target] = floor
	}
	// Validate every committed floor before touching the interrupted transaction.
	// Never rename/promote unacknowledged staging, reset the root or delete foreign
	// entries. Sync its removal under the same exclusive lease before reopening.
	if stage {
		if err := os.Remove(filepath.Join(root, storeStage)); err != nil {
			return err
		}
		if err := syncStoreDirectory(root); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) write(floor retainedFloor) error {
	flags := byte(0)
	if floor.publicationConflict {
		flags |= 1
	}
	if floor.revisionConflict {
		flags |= 2
	}
	raw := append([]byte{3, flags}, floor.descriptor.raw...)
	root := filepath.Join(store.root, storeRecords)
	stage := filepath.Join(root, storeStage)
	file, err := os.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err = file.Chmod(0600); err == nil {
		var n int
		n, err = file.Write(raw)
		if err == nil && n != len(raw) {
			err = io.ErrShortWrite
		}
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err = os.Rename(stage, filepath.Join(root, storeTargetName(floor.descriptor.Target))); err != nil {
		return err
	}
	return syncStoreDirectory(root)
}

func decodeStoredFloor(raw []byte, network [32]byte) (retainedFloor, error) {
	if len(raw) < 2+284+64 || raw[0] != 3 || raw[1] > 3 {
		return retainedFloor{}, errors.New("reachability stored envelope invalid")
	}
	// Expired signed floors still restore; only actual lookup may establish live
	// currentness. The validity fields have fixed positions in the v3 envelope.
	seconds := uint64(0)
	for _, value := range raw[2+274 : 2+282] {
		seconds = seconds<<8 | uint64(value)
	}
	if seconds == 0 || seconds > 1<<63-1 {
		return retainedFloor{}, errors.New("reachability retained expiry invalid")
	}
	var profile [32]byte
	copy(profile[:], raw[2+130:2+162])
	proof, err := VerifyPublish(raw[2:], network, profile, time.Unix(int64(seconds)-1, 0).UTC())
	if err != nil {
		return retainedFloor{}, err
	}
	return retainedFloor{descriptor: proof, publicationConflict: raw[1]&1 != 0, revisionConflict: raw[1]&2 != 0}, nil
}

func storeTargetName(target [32]byte) string { return fmt.Sprintf("%x", target) }

func validateStoreStage(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > MaximumDescriptorSize+2 {
		return errors.New("reachability staging type or size invalid")
	}
	return nil
}

func readStoreFile(path string, maximum int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > int64(maximum) {
		return nil, errors.New("reachability file type or size invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	err = errors.Join(readErr, file.Close())
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw) > maximum {
		return nil, errors.New("reachability file length invalid")
	}
	return raw, nil
}

func boundedStoreEntries(path string, maximum int) ([]os.DirEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	entries, readErr := file.ReadDir(maximum + 1)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err = errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	if len(entries) > maximum {
		return nil, errors.New("reachability directory population exceeds bound")
	}
	return entries, nil
}

func prepareStoreRoot(root string) error {
	if err := createStoreDirectory(root); err != nil {
		return err
	}
	entries, err := boundedStoreEntries(root, 3)
	if err != nil {
		return err
	}
	marked := false
	for _, entry := range entries {
		switch entry.Name() {
		case storeMarkerName:
			marked = true
		case storeLockName, storeRecords:
		default:
			return errors.New("reachability root contains foreign entry")
		}
	}
	lock := filepath.Join(root, storeLockName)
	if marked {
		info, err := os.Lstat(lock)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("reachability initialized lock missing or invalid")
		}
	} else {
		if info, err := os.Lstat(filepath.Join(root, storeRecords)); err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("reachability records directory invalid")
			}
			entries, err := boundedStoreEntries(filepath.Join(root, storeRecords), 0)
			if err != nil || len(entries) != 0 {
				return errors.New("reachability unmarked root retains records")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := ensureStoreFile(lock, nil); err != nil {
			return err
		}
	}
	return syncStoreDirectory(root)
}

func createStoreDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		parent := filepath.Dir(path)
		if parent == path {
			return errors.New("reachability root has no parent")
		}
		if err := createStoreDirectory(parent); err != nil {
			return err
		}
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("reachability directory invalid")
	}
	return syncStoreDirectory(filepath.Dir(path))
}

func initializeStoreRoot(root string) error {
	marker, records := filepath.Join(root, storeMarkerName), filepath.Join(root, storeRecords)
	if _, err := os.Lstat(marker); err == nil {
		if err := ensureStoreFile(marker, []byte(storeMarker)); err != nil {
			return err
		}
		info, err := os.Lstat(records)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("reachability initialized records missing or invalid")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if info, err := os.Lstat(records); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("reachability records directory invalid")
		}
		if entries, err := boundedStoreEntries(records, 0); err != nil || len(entries) != 0 {
			return errors.New("reachability unmarked root retains records")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(records, 0700); err != nil {
			return err
		}
	} else {
		return err
	}
	if err := syncStoreDirectory(records); err != nil {
		return err
	}
	if err := syncStoreDirectory(root); err != nil {
		return err
	}
	if err := ensureStoreFile(marker, []byte(storeMarker)); err != nil {
		return err
	}
	return syncStoreDirectory(root)
}

func ensureStoreFile(path string, expected []byte) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		if len(expected) != 0 {
			var n int
			n, err = file.Write(expected)
			if err == nil && n != len(expected) {
				err = io.ErrShortWrite
			}
		}
		if err == nil {
			err = file.Sync()
		}
		return errors.Join(err, file.Close())
	} else if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("reachability root file invalid")
	}
	if len(expected) == 0 {
		return nil
	}
	raw, err := readStoreFile(path, len(expected))
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, expected) {
		return errors.New("reachability root format unsupported")
	}
	return nil
}
