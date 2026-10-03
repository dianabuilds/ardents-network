package quota

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const ledgerMarker = "ardents-admission-ledger-v1\n"

var ledgerFiles = []string{"admission.pin", "admission.lock", "admission.journal", "admission.floor"}

func ledgerRoot(path string) (*os.Root, os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, nil, ErrInvalid
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || !privateAdmissionFile(before, true) {
		return nil, nil, ErrUnavailable
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = root.Close()
		return nil, nil, ErrUnavailable
	}
	return root, before, nil
}
func createLedgerFile(root *os.Root, name string, raw []byte) error {
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(raw)
	if n != len(raw) && writeErr == nil {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(writeErr, f.Sync(), f.Close())
}
func syncLedgerRoot(root *os.Root) error {
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}
func initializeLedger(path string, binding LedgerBinding) (err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrInvalid
	}
	if err = os.Mkdir(path, 0700); err != nil {
		return ErrUnavailable
	}
	defer func() {
		if err != nil {
			err = errors.Join(ErrUncertain, err)
		}
	}()
	root, _, err := ledgerRoot(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	pin, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(pin)
	if err = createLedgerFile(root, "admission.pin", append([]byte(ledgerMarker), pin...)); err != nil {
		return err
	}
	if err = createLedgerFile(root, "admission.lock", nil); err != nil {
		return err
	}
	if err = createLedgerFile(root, "admission.journal", digest[:]); err != nil {
		return err
	}
	if err = createLedgerFile(root, "admission.floor", floorBytes(uint64(binding.Start.Unix()), digest)); err != nil {
		return err
	}
	if err = syncLedgerRoot(root); err != nil {
		return err
	}
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	return errors.Join(parent.Sync(), parent.Close())
}

func readLedgerFile(root *os.Root, name string, limit int64) ([]byte, os.FileInfo, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || !privateAdmissionFile(before, false) || before.Size() > limit {
		return nil, nil, ErrUnavailable
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, nil, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, nil, ErrUnavailable
	}
	after, err := root.Lstat(name)
	if err != nil || !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return nil, nil, ErrUnavailable
	}
	return raw, after, nil
}

func openLedger(path string, binding LedgerBinding, fault func(string) error) (_ *Ledger, err error) {
	root, identity, err := ledgerRoot(path)
	if err != nil {
		return nil, err
	}
	s := &ledgerState{root: root, path: path, identity: identity, binding: binding, files: map[string]os.FileInfo{}, records: map[[32]byte]debitRecord{}, duty: map[uint64]uint64{}, used: map[quotaKey]uint64{}, permissions: map[[32]byte]permissionQuota{}, fault: fault}
	defer func() {
		if err != nil {
			err = errors.Join(err, releaseAdmissionLock(s.lock), root.Close())
		}
	}()
	// Refuse foreign roots and pending transactions before acquiring/mutating.
	entries, err := root.Open(".")
	if err != nil {
		return nil, ErrUnavailable
	}
	names, readErr := entries.Readdirnames(-1)
	closeErr := entries.Close()
	if readErr != nil || closeErr != nil || len(names) != len(ledgerFiles) {
		return nil, ErrUnavailable
	}
	for _, name := range names {
		ok := false
		for _, want := range ledgerFiles {
			ok = ok || name == want
		}
		if !ok {
			return nil, ErrUnavailable
		}
	}
	pin, info, err := readLedgerFile(root, "admission.pin", 16<<10)
	if err != nil {
		return nil, ErrUnavailable
	}
	canonical, err := json.Marshal(binding)
	if err != nil || !bytes.Equal(pin, append([]byte(ledgerMarker), canonical...)) {
		return nil, ErrUnavailable
	}
	s.files["admission.pin"] = info
	s.header = sha256.Sum256(canonical)
	s.tail = s.header
	// Detach all caller-owned mutable SPKI slices by decoding our checked pin.
	if json.Unmarshal(canonical, &s.binding) != nil {
		return nil, ErrUnavailable
	}
	s.lock, err = acquireAdmissionLock(root)
	if err != nil {
		return nil, err
	}
	s.files["admission.lock"], err = s.lock.Stat()
	if err != nil {
		return nil, ErrUnavailable
	}
	floor, info, err := readLedgerFile(root, "admission.floor", 40)
	if err != nil || !validFloor(floor, s.header) {
		return nil, ErrUnavailable
	}
	s.files["admission.floor"] = info
	s.floor = binary.BigEndian.Uint64(floor[:8])
	if s.floor < uint64(binding.Start.Unix()) || s.floor >= uint64(binding.End.Unix()) {
		return nil, ErrUnavailable
	}
	journal, info, err := readLedgerFile(root, "admission.journal", 32+maximumDebits*recordSize)
	if err != nil || len(journal) < 32 || !bytes.Equal(journal[:32], s.header[:]) || (len(journal)-32)%recordSize != 0 {
		return nil, ErrUnavailable
	}
	s.files["admission.journal"] = info
	var last uint64
	for offset := 32; offset < len(journal); offset += recordSize {
		raw := journal[offset : offset+recordSize]
		r, ok := decodeDebit(raw, s.tail)
		b := r.batch
		if !ok || r.now < last || r.now > s.floor || r.now < b.window || r.now >= b.window+3600 || b.window < uint64(binding.Start.Unix()) || b.window >= uint64(binding.End.Unix()) {
			return nil, ErrUnavailable
		}
		if _, found := s.records[b.request]; found {
			return nil, ErrUnavailable
		}
		if prior, found := s.permissions[b.permission]; found && prior.commitment != b.commitment {
			return nil, ErrUnavailable
		}
		if !s.capacity(b, r.kind) {
			return nil, ErrUnavailable
		}
		s.accept(r)
		copy(s.tail[:], raw[len(raw)-33:len(raw)-1])
		last = r.now
	}
	if err = s.checkFiles(); err != nil {
		return nil, ErrUnavailable
	}
	// A complete marker can remain visible after an earlier failed fsync. A
	// verified reopen must flush it (and any completed floor rename/root
	// creation) before an exact retry can acknowledge the retained debit.
	if err = s.syncRetained(); err != nil {
		return nil, ErrUnavailable
	}
	return &Ledger{s}, nil
}

func (s *ledgerState) syncRetained() error {
	for _, name := range ledgerFiles {
		file, err := s.root.Open(name)
		if err != nil {
			return err
		}
		info, statErr := file.Stat()
		if statErr != nil || !os.SameFile(info, s.files[name]) {
			return errors.Join(ErrUnavailable, file.Close())
		}
		if err := s.inject("reopen-sync"); err != nil {
			return errors.Join(err, file.Close())
		}
		if err := errors.Join(file.Sync(), file.Close()); err != nil {
			return err
		}
	}
	if err := s.inject("reopen-directory-sync"); err != nil {
		return err
	}
	if err := syncLedgerRoot(s.root); err != nil {
		return err
	}
	parent, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	if err := errors.Join(parent.Sync(), parent.Close()); err != nil {
		return err
	}
	return s.checkFiles()
}

func decodeDebit(raw []byte, tail [32]byte) (debitRecord, bool) {
	var r debitRecord
	if len(raw) != recordSize || raw[len(raw)-1] != 1 {
		return r, false
	}
	offset := 0
	for _, id := range []*[32]byte{&r.batch.request, &r.batch.digest, &r.batch.permission, &r.batch.commitment} {
		copy(id[:], raw[offset:offset+32])
		offset += 32
		if *id == [32]byte{} {
			return r, false
		}
	}
	b := &r.batch
	b.window = binary.BigEndian.Uint64(raw[offset:])
	offset += 8
	b.class = raw[offset]
	r.kind = Kind(raw[offset+1])
	offset += 2
	b.count = binary.BigEndian.Uint16(raw[offset:])
	offset += 2
	if b.class < 1 || b.class > 3 || b.count < 1 || b.count > 32 || b.window%3600 != 0 || r.kind != Bootstrap && r.kind != Admitted {
		return r, false
	}
	for i := range b.maxima {
		b.maxima[i] = binary.BigEndian.Uint32(raw[offset:])
		offset += 4
		if b.maxima[i] > 65536 {
			return r, false
		}
	}
	r.now = binary.BigEndian.Uint64(raw[offset:])
	canonical := recordBytes(r, tail)
	return r, bytes.Equal(canonical[:len(canonical)-1], raw[:len(raw)-1])
}

func floorBytes(now uint64, header [32]byte) []byte {
	raw := binary.BigEndian.AppendUint64(nil, now)
	sum := sha256.Sum256(append(append([]byte(nil), header[:]...), raw...))
	return append(raw, sum[:]...)
}
func validFloor(raw []byte, header [32]byte) bool {
	return len(raw) == 40 && bytes.Equal(raw, floorBytes(binary.BigEndian.Uint64(raw[:8]), header))
}

func (s *ledgerState) checkFiles() error {
	current, err := os.Lstat(s.path)
	if err != nil || !os.SameFile(s.identity, current) || !privateAdmissionFile(current, true) {
		return ErrUnavailable
	}
	if _, err := s.root.Lstat("admission.pending"); !errors.Is(err, os.ErrNotExist) {
		return ErrUnavailable
	}
	for name, prior := range s.files {
		current, err := s.root.Lstat(name)
		if err != nil || !os.SameFile(prior, current) || !privateAdmissionFile(current, false) || !current.Mode().IsRegular() || current.Size() != prior.Size() || !current.ModTime().Equal(prior.ModTime()) {
			return ErrUnavailable
		}
	}
	return nil
}

func (s *ledgerState) saveFloor(now uint64) error {
	if err := s.inject("floor-write"); err != nil {
		return err
	}
	raw := floorBytes(now, s.header)
	if err := createLedgerFile(s.root, "admission.pending", raw); err != nil {
		return err
	}
	if err := s.inject("floor-rename"); err != nil {
		return err
	}
	if err := s.root.Rename("admission.pending", "admission.floor"); err != nil {
		return err
	}
	if err := s.inject("floor-sync"); err != nil {
		return err
	}
	if err := syncLedgerRoot(s.root); err != nil {
		return err
	}
	check, info, err := readLedgerFile(s.root, "admission.floor", 40)
	if err != nil || !bytes.Equal(raw, check) {
		return ErrUnavailable
	}
	s.files["admission.floor"] = info
	return nil
}

func (s *ledgerState) appendRecord(r debitRecord) (err error) {
	if err = s.checkFiles(); err != nil {
		return err
	}
	f, err := s.root.OpenFile("admission.journal", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.inject("journal-close"), f.Close()) }()
	info, err := f.Stat()
	if err != nil || !os.SameFile(info, s.files["admission.journal"]) {
		return ErrUnavailable
	}
	if _, err = f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	raw := recordBytes(r, s.tail)
	if err = s.inject("journal-write"); err != nil {
		return err
	}
	n, err := f.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	if err = s.inject("journal-sync"); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = s.inject("journal-commit"); err != nil {
		return err
	}
	if n, err = f.WriteAt([]byte{1}, info.Size()+int64(len(raw))-1); err != nil {
		return err
	}
	if n != 1 {
		return io.ErrShortWrite
	}
	if err = s.inject("commit-sync"); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	raw[len(raw)-1] = 1
	check := make([]byte, len(raw))
	if _, err = f.ReadAt(check, info.Size()); err != nil || !bytes.Equal(raw, check) {
		return ErrUnavailable
	}
	after, err := s.root.Lstat("admission.journal")
	if err != nil || !os.SameFile(info, after) || after.Size() != info.Size()+int64(len(raw)) {
		return ErrUnavailable
	}
	s.files["admission.journal"] = after
	copy(s.tail[:], raw[len(raw)-33:len(raw)-1])
	return s.checkFiles()
}
