package generation

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
)

// Snapshot owns an independently opened read-only sealed generation. It takes
// no Installation lease and grants no selection, Release or startup authority.
// Calls are serialized by its original operation, whose context cannot renew.
type Snapshot struct {
	ctx                      context.Context
	parent, root             *os.Root
	file                     *os.File
	parentPath, name         string
	parentIdentity, identity os.FileInfo
	files                    map[string]fileObservation
	gid                      uint32
	terminal                 error
	observationBytes         []byte
}

func OpenSnapshot(ctx context.Context, parentPath string, expectedParent, expectedDirectory os.FileInfo, name string, gid uint32) (result *Snapshot, returnedErr error) {
	digest, err := hex.DecodeString(name)
	if ctx == nil || gid == 0 || gid == ^uint32(0) || !filepath.IsAbs(parentPath) || filepath.Clean(parentPath) != parentPath || filepath.Base(parentPath) != "generations" || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != name || !sealedDirectory(expectedParent, gid) || !sealedDirectory(expectedDirectory, gid) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := &Snapshot{ctx: ctx, parentPath: parentPath, name: name, gid: gid, files: make(map[string]fileObservation)}
	defer func() {
		if returnedErr != nil {
			s.terminal = returnedErr
			returnedErr = s.Close()
			result = nil
		}
	}()
	s.parent, err = os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	s.parentIdentity, err = s.parent.Stat(".")
	if err != nil || !sameDirectory(expectedParent, s.parentIdentity) || !sealedDirectory(s.parentIdentity, gid) {
		return nil, errors.Join(ErrBinding, err)
	}
	s.file, err = s.parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	s.identity, err = s.file.Stat()
	if err != nil || !sameDirectory(expectedDirectory, s.identity) || !sealedDirectory(s.identity, gid) {
		return nil, errors.Join(ErrBinding, err)
	}
	s.root, err = s.parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	if err := s.observeDirectories(); err != nil {
		return nil, err
	}
	if err := s.inventory(); err != nil {
		return nil, err
	}
	for _, name := range Names() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, err := s.read(name)
		if err != nil {
			return nil, err
		}
		s.files[name] = record
	}
	if err := s.Observe(); err != nil {
		return nil, err
	}
	return s, nil
}

func sealedDirectory(info os.FileInfo, gid uint32) bool {
	if info == nil || info.Mode() != os.ModeDir|0750 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == gid
}

func (s *Snapshot) observeDirectories() error {
	if s.parent == nil || s.root == nil || s.file == nil {
		return ErrBinding
	}
	parentPath, a := os.Lstat(s.parentPath)
	parentFD, b := s.parent.Stat(".")
	path, c := s.parent.Lstat(s.name)
	root, d := s.root.Stat(".")
	file, e := s.file.Stat()
	if err := errors.Join(a, b, c, d, e); err != nil {
		return errors.Join(ErrBinding, err)
	}
	for _, info := range []os.FileInfo{parentPath, parentFD} {
		if !sameDirectory(s.parentIdentity, info) || !sealedDirectory(info, s.gid) {
			return ErrBinding
		}
	}
	for _, info := range []os.FileInfo{path, root, file} {
		if !sameDirectory(s.identity, info) || !sealedDirectory(info, s.gid) {
			return ErrBinding
		}
	}
	return nil
}

func (s *Snapshot) inventory() (returnedErr error) {
	file, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	names, err := file.Readdirnames(len(Names()) + 1)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if err != nil || len(names) != len(Names()) {
		return errors.Join(ErrBinding, err)
	}
	for _, name := range names {
		if _, allowed := fileMode(name); !allowed {
			return ErrBinding
		}
	}
	return nil
}

func (s *Snapshot) read(name string) (fileObservation, error) {
	return s.readInto(name, nil, nil)
}

func (s *Snapshot) readInto(name string, body []byte, expected *fileObservation) (record fileObservation, returnedErr error) {
	mode, allowed := fileMode(name)
	if !allowed {
		return fileObservation{}, ErrInput
	}
	maximum := int64(64 << 10)
	if mode == 0555 {
		maximum = 64 << 20
	} else if name == "binding.json" {
		maximum = 32 << 10
	}
	file, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fileObservation{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || info == nil || !info.Mode().IsRegular() || info.Mode() != mode || info.Size() < 1 || info.Size() > maximum {
		return fileObservation{}, errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != s.gid || native.Nlink != 1 {
		return fileObservation{}, ErrBinding
	}
	if expected != nil {
		if info.Size() != int64(len(expected.body)) || len(body) == 0 {
			return fileObservation{}, ErrBinding
		}
		if err := s.compareBytes(file, expected.body, body); err != nil {
			return fileObservation{}, err
		}
		// Metadata matching also binds the observed size to these original
		// bytes. Their equality was established by the full fresh read above;
		// EOF and original path/descriptor metadata still follow below.
		body = expected.body
	} else {
		body = make([]byte, info.Size())
		if _, err := io.ReadFull(file, body); err != nil {
			return fileObservation{}, errors.Join(ErrBinding, err)
		}
	}
	var trailing [1]byte
	if n, err := file.ReadAt(trailing[:], int64(len(body))); n != 0 || err != io.EOF {
		return fileObservation{}, errors.Join(ErrBinding, err)
	}
	record = fileObservation{identity: info, body: body, gid: s.gid, mode: mode}
	path, pathErr := s.root.Lstat(name)
	final, fdErr := file.Stat()
	if pathErr != nil || fdErr != nil || !snapshotFileMatches(record, path) || !snapshotFileMatches(record, final) {
		return fileObservation{}, errors.Join(ErrBinding, pathErr, fdErr)
	}
	return record, s.ctx.Err()
}

// compareBytes divides a large file between at most sixteen bounded readers.
// Each compares disjoint fresh ranges through the one retained descriptor;
// all join before EOF and final path/descriptor observations. Small files
// remain sequential so no nested file/range fan-out or extra scratch is needed.
func (s *Snapshot) compareBytes(file *os.File, expected, scratch []byte) error {
	readers := 1
	if len(expected) >= 1<<20 {
		readers = 16
	}
	var failures [16]error
	read := func(index int) {
		buffer := scratch[len(scratch)*index/readers : len(scratch)*(index+1)/readers]
		end := len(expected) * (index + 1) / readers
		for offset := len(expected) * index / readers; offset < end; {
			if err := s.ctx.Err(); err != nil {
				failures[index] = err
				return
			}
			n := min(len(buffer), end-offset)
			if _, err := file.ReadAt(buffer[:n], int64(offset)); err != nil {
				failures[index] = errors.Join(ErrBinding, err)
				return
			}
			if !bytes.Equal(buffer[:n], expected[offset:offset+n]) {
				failures[index] = ErrBinding
				return
			}
			offset += n
		}
	}
	if readers == 1 {
		read(0)
	} else {
		var joined sync.WaitGroup
		for index := 0; index < readers-1; index++ {
			joined.Go(func() { read(index) })
		}
		read(readers - 1)
		joined.Wait()
	}
	return errors.Join(failures[:]...)
}

func snapshotFileMatches(record fileObservation, info os.FileInfo) bool {
	if !fileObservationMatches(record, info) {
		return false
	}
	a, aOK := record.identity.Sys().(*syscall.Stat_t)
	b, bOK := info.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Ctim == b.Ctim
}

func (s *Snapshot) Observe() (returnedErr error) {
	if s == nil || s.ctx == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	defer func() {
		if returnedErr != nil {
			s.terminal = returnedErr
		}
	}()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.observeDirectories(); err != nil {
		return err
	}
	if err := s.inventory(); err != nil {
		return err
	}
	if len(s.files) != len(Names()) {
		return ErrBinding
	}
	names := make([]string, 0, len(s.files))
	for name := range s.files {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return len(s.files[names[i]].body) > len(s.files[names[j]].body) })
	// Large files share bounded scratch across readers instead of leaving one idle after the
	// smaller files finish. Custody and final inventory remain serialized.
	if s.observationBytes == nil {
		s.observationBytes = make([]byte, 2*(256<<10))
	}
	for _, name := range names {
		original := s.files[name]
		current, err := s.readInto(name, s.observationBytes, &original)
		if err != nil {
			return err
		}
		if !snapshotFileMatches(original, current.identity) {
			return ErrBinding
		}
	}
	return errors.Join(s.observeDirectories(), s.inventory(), s.ctx.Err())
}

// Bytes and Matches return detached bytes and a physical comparison, not handles
// or shared observations. The original operation must Observe before effects.
func (s *Snapshot) Bytes(name string) []byte {
	if s == nil || s.terminal != nil || s.root == nil {
		return nil
	}
	return bytes.Clone(s.files[name].body)
}
func (s *Snapshot) Matches(name string, info os.FileInfo) bool {
	return s != nil && s.root != nil && s.terminal == nil && snapshotFileMatches(s.files[name], info)
}
func (s *Snapshot) Close() error {
	if s != nil {
		clear(s.observationBytes)
		s.observationBytes = nil
	}
	if s == nil {
		return nil
	}
	if s.root != nil {
		s.terminal = errors.Join(s.terminal, s.root.Close())
		s.root = nil
	}
	if s.file != nil {
		s.terminal = errors.Join(s.terminal, s.file.Close())
		s.file = nil
	}
	if s.parent != nil {
		s.terminal = errors.Join(s.terminal, s.parent.Close())
		s.parent = nil
	}
	return s.terminal
}
