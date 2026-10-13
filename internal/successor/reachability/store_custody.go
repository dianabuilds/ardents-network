package reachability

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Native access remains relative to the original directory handles. Path
// observations can only deny that owner; they cannot replace its handles.
type storeCustody struct {
	mu                                          sync.Mutex
	root, records                               *os.Root
	rootInfo, recordsInfo, markerInfo, lockInfo os.FileInfo
	files                                       map[string]storeRecordCustody
	syncDirectory                               *os.File
	marker                                      *os.File
}

type storeRecordCustody struct {
	info   os.FileInfo
	digest [32]byte
	file   *os.File
}

func retainStoreCustody(root *os.Root) (custody *storeCustody, result error) {
	custody = &storeCustody{root: root}
	defer func() {
		if result != nil {
			result = errors.Join(result, custody.close())
		}
	}()
	var err error
	custody.rootInfo, err = custody.root.Stat(".")
	if err != nil {
		return custody, err
	}
	custody.records, err = custody.root.OpenRoot(storeRecords)
	if err != nil {
		return custody, err
	}
	custody.recordsInfo, err = custody.records.Stat(".")
	if err != nil {
		return custody, err
	}
	custody.syncDirectory, err = retainStoreSyncDirectory(custody.records)
	if err != nil {
		return custody, err
	}
	syncInfo, err := custody.syncDirectory.Stat()
	if err != nil || !os.SameFile(syncInfo, custody.recordsInfo) {
		return custody, errors.Join(errors.New("reachability original sync directory differs"), err)
	}
	custody.marker, err = custody.root.Open(storeMarkerName)
	if err != nil {
		return custody, err
	}
	custody.markerInfo, err = custody.marker.Stat()
	if err != nil {
		return custody, err
	}
	custody.lockInfo, err = custody.root.Lstat(storeLockName)
	if err != nil {
		return custody, err
	}
	custody.files = make(map[string]storeRecordCustody)
	return custody, nil
}

func (custody *storeCustody) close() error {
	if custody == nil {
		return nil
	}
	var result error
	for _, record := range custody.files {
		result = errors.Join(result, record.file.Close())
	}
	if custody.marker != nil {
		result = errors.Join(result, custody.marker.Close())
	}
	if custody.syncDirectory != nil {
		result = errors.Join(result, custody.syncDirectory.Close())
	}
	if custody.records != nil {
		result = errors.Join(result, custody.records.Close())
	}
	if custody.root != nil {
		result = errors.Join(result, custody.root.Close())
	}
	return result
}

func sameStoreObject(path string, expected os.FileInfo) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, expected) || info.Mode() != expected.Mode() {
		return errors.New("reachability original object lost or replaced")
	}
	return nil
}

func (store *Store) checkCustody() error {
	store.custody.mu.Lock()
	err := store.inspectCustody()
	store.custody.mu.Unlock()
	if err != nil {
		store.Deny(err)
		store.mu.Lock()
		err = store.failure
		store.mu.Unlock()
	}
	return err
}

// Called only under the native custody mutex, never the state mutex.
func (store *Store) inspectCustody() error {
	c := store.custody
	for _, object := range []struct {
		path string
		info os.FileInfo
	}{
		{store.root, c.rootInfo},
		{filepath.Join(store.root, storeRecords), c.recordsInfo},
		{filepath.Join(store.root, storeMarkerName), c.markerInfo},
		{filepath.Join(store.root, storeLockName), c.lockInfo},
	} {
		if err := sameStoreObject(object.path, object.info); err != nil {
			return err
		}
	}
	if err := store.lease.check(c.lockInfo); err != nil {
		return err
	}
	marker := make([]byte, len(storeMarker)+1)
	n, err := c.marker.ReadAt(marker, 0)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	marker = marker[:n]
	if err != nil || !bytes.Equal(marker, []byte(storeMarker)) {
		return errors.Join(errors.New("reachability original marker unavailable"), err)
	}
	directory, err := c.records.Open(".")
	if err != nil {
		return err
	}
	entries, err := directory.ReadDir(MaximumTargets + 1)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	err = errors.Join(err, directory.Close())
	if err != nil || len(entries) != len(c.files) {
		return errors.Join(errors.New("reachability original record inventory changed"), err)
	}
	for _, entry := range entries {
		if _, retained := c.files[entry.Name()]; !retained {
			return errors.New("reachability original record inventory foreign")
		}
	}
	for name, retained := range c.files {
		info, err := c.records.Lstat(name)
		if err != nil || info.Mode() != retained.info.Mode() || !os.SameFile(info, retained.info) {
			return errors.Join(errors.New("reachability original record lost or replaced"), err)
		}
		pinned, err := retained.file.Stat()
		if err != nil || !os.SameFile(info, pinned) {
			return errors.Join(errors.New("reachability original record pin lost"), err)
		}
		raw, err := c.readRecord(name)
		if err != nil || sha256.Sum256(raw) != retained.digest {
			return errors.Join(errors.New("reachability original record changed"), err)
		}
	}
	return nil
}

func (custody *storeCustody) readRecord(name string) ([]byte, error) {
	return readOriginalStoreFile(custody.records, name, MaximumDescriptorSize+2)
}

func (custody *storeCustody) remember(name string, raw []byte) error {
	info, err := custody.records.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return errors.Join(errors.New("reachability committed record unavailable"), err)
	}
	retained, err := custody.readRecord(name)
	if err != nil || !bytes.Equal(raw, retained) {
		return errors.Join(errors.New("reachability committed readback differs"), err)
	}
	file, err := custody.records.Open(name)
	if err != nil {
		return err
	}
	pinned, err := file.Stat()
	if err != nil || !os.SameFile(info, pinned) {
		return errors.Join(errors.New("reachability committed pin differs"), err, file.Close())
	}
	prior, exists := custody.files[name]
	custody.files[name] = storeRecordCustody{info: info, digest: sha256.Sum256(raw), file: file}
	if exists {
		return prior.file.Close()
	}
	return nil
}
