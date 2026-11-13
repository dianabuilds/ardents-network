package join

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Independent physical custody while the actual coordinator is stopped. No
// flock is taken until its original scope joins; a later recovery must acquire
// its own fresh EX lease and perform the complete operation-specific validation.
type transitionPrefix struct {
	message   transitionMessage
	parents   *nativeDirectories
	journal   *initialFile
	exclusive bool
	closed    bool
	terminal  error
	// Failure-only original proof-writer Close control, never replacement I/O.
	beforeProofClose func(*os.File) error
}

func transitionRecordLimit(operation string) uint32 {
	if operation == "replacement" {
		return 62
	}
	return 0
}

// This checks only canonical framing and native opening identity. It never
// interprets a prefix as an accepting completion or recovery continuation.
func decodeTransitionPrefix(raw []byte, operation string) ([]initialRecord, error) {
	limit := transitionRecordLimit(operation)
	if limit == 0 || len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' {
		return nil, ErrBinding
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) == 0 || uint32(len(lines)) > limit {
		return nil, ErrBinding
	}
	records := make([]initialRecord, len(lines))
	for i, line := range lines {
		if len(line) == 0 || len(line)+1 > 32<<10 {
			return nil, ErrBinding
		}
		if json.Unmarshal(line, &records[i]) != nil {
			return nil, ErrBinding
		}
		canonical, _ := json.Marshal(records[i])
		if records[i].Sequence != i || !bytes.Equal(line, canonical) {
			return nil, ErrBinding
		}
	}
	if operation != "replacement" || records[0].Phase != "join-replacement-v1" {
		return nil, ErrBinding
	}
	return records, nil
}

func openTransitionPrefix(ctx context.Context, message transitionMessage) (result *transitionPrefix, returned error) {
	s := &transitionPrefix{message: message, parents: &nativeDirectories{ctx: context.WithoutCancel(ctx), byPath: map[string]*inputDirectory{}}}
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, s.close())
			result = nil
		}
	}()
	selection, err := decodeSelection(message.Binding)
	if err != nil || selection.binding.Generation != message.Generation || digest(message.Binding) != message.BindingSHA256 {
		return nil, errors.Join(ErrBinding, err)
	}
	root := selection.binding.Request.InstallationRoot
	matched := message.Operation == "initial" && message.Journal == root+".join-initial-journal" || message.Operation == "replacement" && message.Journal == replacementJournal(root, message.Generation)
	for n := attemptNumber(0); n < attemptLimit; n++ {
		matched = matched || message.Operation == "start" && message.Journal == n.journal(root) || message.Operation == "retirement" && message.Journal == n.retirementJournal(root)
	}
	if !matched || message.Sequence == 0 || message.Records != message.Sequence || message.Records > transitionRecordLimit(message.Operation) || message.Device == 0 || message.Inode == 0 || !validStartDigest(message.PrefixSHA256) {
		return nil, ErrBinding
	}
	if _, err := s.parents.parents(message.Journal); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(message.Journal, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	s.journal = &initialFile{path: message.Journal, file: file}
	s.journal.info, err = file.Stat()
	if err != nil || !nodeAccess(s.journal.info, 0600, 0, 0, false) || s.journal.info.Size() <= 0 || s.journal.info.Size() > 4<<20 {
		return nil, errors.Join(ErrBinding, err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return nil, errors.Join(ErrBinding, err)
	}
	s.journal.body, err = io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil || int64(len(s.journal.body)) != s.journal.info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := errors.Join(s.check(), ctx.Err()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *transitionPrefix) check() error {
	if s == nil || s.closed || s.journal == nil {
		return ErrInput
	}
	if err := errors.Join(s.parents.links(), observeInitialFile(s.journal)); err != nil {
		return err
	}
	records, err := decodeTransitionPrefix(s.journal.body, s.message.Operation)
	native, ok := infoSys(s.journal.info)
	if err != nil || !ok || uint32(len(records)) != s.message.Records || digest(s.journal.body) != s.message.PrefixSHA256 || uint64(native.Dev) != s.message.Device || native.Ino != s.message.Inode {
		return errors.Join(ErrBinding, err)
	}
	first := records[0]
	if first.Path != s.journal.path || first.Device != s.message.Device || first.Inode != s.message.Inode || first.Mode != 0600 || first.UID != 0 || first.GID != 0 {
		return ErrBinding
	}
	return errors.Join(checkTransitionContexts(s.message, records), s.parents.links())
}

func (s *transitionPrefix) parent() os.FileInfo {
	if s == nil || s.parents == nil || s.journal == nil {
		return nil
	}
	directory := s.parents.byPath[filepath.Dir(s.journal.path)]
	if directory == nil {
		return nil
	}
	return directory.info
}

func (s *transitionPrefix) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		// Independent ancestors close before the original journal lease returns.
		s.terminal = errors.Join(s.terminal, s.parents.close())
		if s.journal != nil {
			s.terminal = errors.Join(s.terminal, s.journal.file.Close())
		}
	}
	return s.terminal
}
