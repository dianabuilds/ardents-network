package issuance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

const resultMarker = "ardents-issuer-results-v1\n"

type resultBinding struct {
	Binding   admission.LedgerBinding
	Inventory [32]byte
}

func resultPin(b admission.LedgerBinding, digest [32]byte) ([]byte, error) {
	if _, err := admission.BindingDigest(b); err != nil {
		return nil, ErrInvalid
	}
	raw, err := json.Marshal(resultBinding{b, digest})
	return append([]byte(resultMarker), raw...), err
}
func resultFrame(previous [32]byte, body []byte) ([]byte, [32]byte) {
	h := sha256.New()
	_, _ = h.Write(previous[:])
	_, _ = h.Write(body)
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	raw := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	raw = append(raw, body...)
	return append(raw, digest[:]...), digest
}
func nextResultFrame(raw []byte, previous [32]byte) ([]byte, []byte, [32]byte, error) {
	if len(raw) < 36 {
		return nil, nil, previous, ErrUnavailable
	}
	n := int(binary.BigEndian.Uint32(raw))
	if n > 16348 || n < 1 || len(raw) < n+36 {
		return nil, nil, previous, ErrUnavailable
	}
	body := raw[4 : 4+n]
	encoded, digest := resultFrame(previous, body)
	if !bytes.Equal(encoded, raw[:n+36]) {
		return nil, nil, previous, ErrUnavailable
	}
	return body, raw[n+36:], digest, nil
}
func resultFloor(header [32]byte, n uint64) []byte {
	raw := binary.BigEndian.AppendUint64(nil, n)
	digest := sha256.Sum256(append(append([]byte(nil), header[:]...), raw...))
	return append(raw, digest[:]...)
}
func initializeResults(ctx context.Context, path string, b admission.LedgerBinding, digest [32]byte, fault func(string) error) (err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrInvalid
	}
	pin, err := resultPin(b, digest)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil || parent != filepath.Dir(path) {
		return ErrUnavailable
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err = os.Mkdir(path, 0700); err != nil {
		return ErrUnavailable
	}
	defer func() {
		if err != nil {
			err = errors.Join(ErrUncertain, err)
		}
	}()
	root, identity, err := ownedRoot(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	header := sha256.Sum256(pin)
	files := map[string]os.FileInfo{}
	for _, file := range []struct {
		name string
		raw  []byte
	}{{"results.pin", pin}, {"results.lock", nil}, {"results.journal", header[:]}, {"results.floor", resultFloor(header, uint64(b.Start.Unix()))}} {
		if err = writeExclusive(root, file.name, file.raw, fault); err != nil {
			return err
		}
		saved, info, e := readOwned(root, file.name, 32<<10)
		if e != nil || !bytes.Equal(saved, file.raw) {
			return ErrUnavailable
		}
		files[file.name] = info
	}
	if err = errors.Join(syncDirectory(root, fault), syncParent(path)); err != nil {
		return err
	}
	current, err := os.Lstat(path)
	if err != nil || !privateFile(current, true) || !os.SameFile(identity, current) {
		return ErrUnavailable
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := directory.Readdirnames(-1)
	if err = errors.Join(readErr, directory.Close()); err != nil {
		return err
	}
	if len(names) != len(files) {
		return ErrUnavailable
	}
	for _, name := range names {
		before, ok := files[name]
		after, e := root.Lstat(name)
		if !ok || e != nil || !sameIdentity(before, after) {
			return ErrUnavailable
		}
	}
	return nil
}
func (s *resultState) check() error {
	info, e := os.Lstat(s.path)
	if e != nil || !privateFile(info, true) || !os.SameFile(info, s.identity) {
		return ErrUnavailable
	}
	d, e := s.root.Open(".")
	if e != nil {
		return ErrUnavailable
	}
	names, e := d.Readdirnames(-1)
	closeErr := d.Close()
	if e != nil || closeErr != nil || len(names) != 4 {
		return ErrUnavailable
	}
	for _, name := range names {
		before, ok := s.files[name]
		if !ok {
			return ErrUnavailable
		}
		after, e := s.root.Lstat(name)
		if e != nil || !sameIdentity(before, after) {
			return ErrUnavailable
		}
	}
	if _, e := s.store.Inventory(); e != nil {
		return e
	}
	return nil
}
func openResults(ctx context.Context, path string, store Store, b admission.LedgerBinding, pin []byte, fault func(string) error) (_ ResultStore, err error) {
	root, identity, err := ownedRoot(path)
	if err != nil {
		return ResultStore{}, err
	}
	var protected resultBinding
	if json.Unmarshal(pin[len(resultMarker):], &protected) != nil {
		return ResultStore{}, errors.Join(ErrInvalid, root.Close())
	}
	s := &resultState{root: root, path: path, identity: identity, store: store, binding: protected.Binding, files: map[string]os.FileInfo{}, records: map[[32]byte]savedResult{}, fault: fault, header: sha256.Sum256(pin)}
	defer func() {
		if err != nil {
			err = errors.Join(err, releaseLock(s.lock), root.Close())
		}
	}()
	for _, name := range resultFiles {
		info, e := root.Lstat(name)
		if e != nil || !privateFile(info, false) {
			return ResultStore{}, ErrUnavailable
		}
		s.files[name] = info
	}
	if err = s.check(); err != nil {
		return ResultStore{}, err
	}
	retained, _, err := readOwned(root, "results.pin", 32<<10)
	if err != nil || !bytes.Equal(retained, pin) {
		return ResultStore{}, ErrUnavailable
	}
	s.lock, err = acquireNamedLock(root, "results.lock")
	if err != nil {
		return ResultStore{}, err
	}
	floor, _, err := readOwned(root, "results.floor", 40)
	if err != nil || len(floor) != 40 {
		return ResultStore{}, ErrUnavailable
	}
	s.floor = binary.BigEndian.Uint64(floor)
	if s.floor < uint64(b.Start.Unix()) || s.floor > uint64(b.End.Unix()) || !bytes.Equal(floor, resultFloor(s.header, s.floor)) {
		return ResultStore{}, ErrUnavailable
	}
	journal, _, err := readOwned(root, "results.journal", maximumJournal)
	if err != nil || len(journal) < 32 || !bytes.Equal(journal[:32], s.header[:]) {
		return ResultStore{}, ErrUnavailable
	}
	s.size = int64(len(journal))
	s.tail = s.header
	for remainder := journal[32:]; len(remainder) > 0; {
		if ctx.Err() != nil {
			return ResultStore{}, ctx.Err()
		}
		request, rest, tail, e := nextResultFrame(remainder, s.tail)
		if e != nil || len(request) < 9+625+259+64 {
			return ResultStore{}, ErrUnavailable
		}
		response, rest, last, e := nextResultFrame(rest, tail)
		if e != nil {
			return ResultStore{}, ErrUnavailable
		}
		kind := admission.Kind(request[0])
		checkedRaw := binary.BigEndian.Uint64(request[1:9])
		if checkedRaw > s.floor {
			return ResultStore{}, ErrUnavailable
		}
		checked := time.Unix(int64(checkedRaw), 0).UTC()
		raw := request[9:]
		if kind != admission.Bootstrap && kind != admission.Admitted {
			return ResultStore{}, ErrUnavailable
		}
		if outcome := admission.ValidateBatch(ctx, raw, s.binding, checked); outcome != admission.Accepted {
			if outcome == admission.Canceled {
				return ResultStore{}, ctx.Err()
			}
			return ResultStore{}, ErrUnavailable
		}
		expected, e := store.sign(ctx, raw)
		if e != nil {
			return ResultStore{}, e
		}
		if !bytes.Equal(expected, response) {
			return ResultStore{}, ErrUnavailable
		}
		var id [32]byte
		copy(id[:], raw[236:268])
		if _, ok := s.records[id]; ok {
			return ResultStore{}, ErrUnavailable
		}
		s.signatures += int(raw[624])
		if len(s.records) >= maximumResults || s.signatures > maximumResults {
			return ResultStore{}, ErrUnavailable
		}
		s.records[id] = savedResult{raw: raw, response: response, kind: kind, checked: checked}
		s.tail = last
		remainder = rest
	}
	for _, name := range resultFiles {
		f, e := root.OpenFile(name, os.O_RDWR, 0)
		if e != nil {
			return ResultStore{}, ErrUnavailable
		}
		opened, e := f.Stat()
		syncErr := failAt(fault, "open-sync")
		if syncErr == nil {
			syncErr = f.Sync()
		}
		e = errors.Join(e, syncErr, f.Close())
		if e != nil || !sameIdentity(s.files[name], opened) {
			return ResultStore{}, ErrUnavailable
		}
	}
	if err = errors.Join(syncDirectory(root, fault), syncParent(path), s.check()); err != nil {
		return ResultStore{}, ErrUnavailable
	}
	return ResultStore{state: s}, nil
}

func (s *resultState) saveFloor(n uint64) error {
	raw := resultFloor(s.header, n)
	if err := writeExclusive(s.root, "results.pending", raw, s.fault); err != nil {
		return err
	}
	if err := failAt(s.fault, "floor-rename"); err != nil {
		return err
	}
	if err := s.root.Rename("results.pending", "results.floor"); err != nil {
		return err
	}
	saved, info, err := readOwned(s.root, "results.floor", 40)
	if err != nil || !bytes.Equal(saved, raw) {
		return ErrUnavailable
	}
	s.files["results.floor"] = info
	if err = syncDirectory(s.root, s.fault); err != nil {
		return err
	}
	s.floor = n
	return s.check()
}
func (s *resultState) appendPair(first, second []byte) (err error) {
	f, err := s.root.OpenFile("results.journal", os.O_RDWR|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	opened, err := f.Stat()
	if err != nil || !sameIdentity(s.files["results.journal"], opened) {
		return errors.Join(ErrUnavailable, f.Close())
	}
	defer func() { err = errors.Join(err, f.Close(), failAt(s.fault, "append-close")) }()
	for i, raw := range [][]byte{first, second} {
		phase := "request-write"
		if i == 1 {
			phase = "response-write"
		}
		if err = failAt(s.fault, phase); err != nil {
			return err
		}
		n, e := f.Write(raw)
		if e != nil {
			return e
		}
		if n != len(raw) {
			return io.ErrShortWrite
		}
	}
	if err = failAt(s.fault, "append-sync"); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if _, err = f.Seek(s.size, io.SeekStart); err != nil {
		return err
	}
	saved := make([]byte, len(first)+len(second))
	if _, err = io.ReadFull(f, saved); err != nil || !bytes.Equal(saved, append(append([]byte(nil), first...), second...)) {
		return ErrUnavailable
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	s.files["results.journal"] = info
	return s.check()
}
