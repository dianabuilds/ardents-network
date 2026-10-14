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
	root := store.custody.records
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := directory.ReadDir(MaximumTargets + 2)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	err = errors.Join(readErr, directory.Close())
	if err != nil {
		return err
	}
	if len(entries) > MaximumTargets+1 {
		return errors.New("reachability directory population exceeds bound")
	}
	stage := false
	for _, entry := range entries {
		if entry.Name() == storeStage {
			info, err := root.Lstat(storeStage)
			if stage || err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > MaximumDescriptorSize+2 {
				return errors.New("reachability staging invalid")
			}
			stage = true
			continue
		}
		if len(store.floors) >= MaximumTargets || len(entry.Name()) != 64 {
			return errors.New("reachability record population or name invalid")
		}
		raw, err := readOriginalStoreFile(root, entry.Name(), MaximumDescriptorSize+2)
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
		if err := store.custody.remember(entry.Name(), raw); err != nil {
			return err
		}
	}
	// Validate every committed floor before touching the interrupted transaction.
	// Never rename/promote unacknowledged staging, reset the root or delete foreign
	// entries. Sync its removal under the same exclusive lease before reopening.
	if stage {
		if err := root.Remove(storeStage); err != nil {
			return err
		}
		if err := store.custody.syncDirectory.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) write(floor retainedFloor) (result error) {
	store.custody.mu.Lock()
	defer store.custody.mu.Unlock()
	if err := store.inspectCustody(); err != nil {
		return store.fail(err)
	}
	if err := store.localCheck(); err != nil {
		return err
	}
	// Latch actual native failures before releasing the physical mutation gate.
	// An earlier caller-denial notice never masks a failed write/sync/readback.
	defer func() {
		if result != nil {
			result = store.fail(result)
		}
	}()
	flags := byte(0)
	if floor.publicationConflict {
		flags |= 1
	}
	if floor.revisionConflict {
		flags |= 2
	}
	raw := append([]byte{3, flags}, floor.descriptor.raw...)
	file, err := store.custody.records.OpenFile(storeStage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
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
	name := storeTargetName(floor.descriptor.Target)
	if err = store.custody.records.Rename(storeStage, name); err != nil {
		return err
	}
	if err := store.custody.syncDirectory.Sync(); err != nil {
		return err
	}
	if err := store.custody.remember(name, raw); err != nil {
		return err
	}
	// A fresh Lookup cannot pass the native custody gate after this physical
	// commit but before observing its retained floor. Close/denial may already
	// have happened; the durable floor is still retained and no ACK is granted.
	store.mu.Lock()
	store.floors[floor.descriptor.Target] = floor
	store.mu.Unlock()
	return nil
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

func initializeStoreRoot(root *os.Root) error {
	if _, err := root.Lstat(storeMarkerName); err == nil {
		raw, err := readOriginalStoreFile(root, storeMarkerName, len(storeMarker))
		if err != nil || !bytes.Equal(raw, []byte(storeMarker)) {
			return errors.Join(errors.New("reachability root format unsupported"), err)
		}
		info, err := root.Lstat(storeRecords)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("reachability initialized records missing or invalid")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if info, err := root.Lstat(storeRecords); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("reachability records directory invalid")
		}
		directory, err := root.Open(storeRecords)
		if err != nil {
			return err
		}
		entries, readErr := directory.ReadDir(1)
		if errors.Is(readErr, io.EOF) {
			readErr = nil
		}
		if err := errors.Join(readErr, directory.Close()); err != nil || len(entries) != 0 {
			return errors.Join(errors.New("reachability unmarked root retains records"), err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := root.Mkdir(storeRecords, 0700); err != nil {
			return err
		}
	} else {
		return err
	}
	records, err := root.OpenRoot(storeRecords)
	if err != nil {
		return err
	}
	err = errors.Join(syncRetainedStoreDirectory(records), records.Close())
	if err != nil {
		return err
	}
	if err := syncRetainedStoreDirectory(root); err != nil {
		return err
	}
	file, err := root.OpenFile(storeMarkerName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, writeErr := file.Write([]byte(storeMarker))
	if writeErr == nil && n != len(storeMarker) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return syncRetainedStoreDirectory(root)
}

func syncRetainedStoreDirectory(root *os.Root) error {
	file, err := retainStoreSyncDirectory(root)
	if err != nil {
		return err
	}
	expected, expectedErr := root.Stat(".")
	actual, actualErr := file.Stat()
	if expectedErr != nil || actualErr != nil || !os.SameFile(expected, actual) {
		return errors.Join(errors.New("reachability original directory sync differs"), expectedErr, actualErr, file.Close())
	}
	return errors.Join(file.Sync(), file.Close())
}

func readOriginalStoreFile(root *os.Root, name string, maximum int) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(maximum) {
		return nil, errors.New("reachability original file type or length invalid")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(info, opened) {
		return nil, errors.Join(errors.New("reachability original file changed during open"), statErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	err = errors.Join(readErr, file.Close())
	if err != nil || len(raw) > maximum {
		return nil, errors.Join(errors.New("reachability original file read unavailable"), err)
	}
	return raw, nil
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
