package forwarding

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Independent native custody, never a borrowed stop writer or live runtime.
// Readback still requires its original blocker; committed history requires an
// absent blocker and its own exclusive journal lease before reading any bytes.
type retirementSnapshot struct {
	ctx                       context.Context
	parents                   *inputs
	directory, socket         *os.File
	directoryInfo, socketInfo os.FileInfo
	files                     map[string]*initialFile
	order                     []*initialFile
	journal, pending          *initialFile
	path                      string
	binding                   binding
	number                    attemptNumber
	bindingSHA, startSHA      string
	archive                   startArchive
	id                        [16]byte
	result                    retirementResult
	sha256                    string
	purpose                   retirementReadPurpose
	closed                    bool
	terminal                  error
	// Negative physical controls only; cannot supply successful observations.
	observe func(string, *os.File)
}

type retirementReadPurpose uint8

const (
	retirementReadback retirementReadPurpose = iota
	retirementCommitted
)

func readRetirementSnapshot(ctx context.Context, b binding, number attemptNumber, bindingSHA, startSHA string, archive startArchive, id [16]byte, purpose retirementReadPurpose) (h *retirementSnapshot, returned error) {
	h = &retirementSnapshot{ctx: ctx, binding: b, number: number, bindingSHA: bindingSHA, startSHA: startSHA, archive: archive, id: id, purpose: purpose, files: map[string]*initialFile{}, path: number.retirement(b.Request.InstallationRoot)}
	defer func() {
		if returned != nil {
			h.terminal = returned
		}
	}()
	if ctx == nil || os.Geteuid() != 0 || number >= attemptLimit || !validPath(b.Request.InstallationRoot) || b.UID < 100 || b.UID > 999 || b.GID != b.UID || !validStartDigest(bindingSHA) || !validStartDigest(startSHA) || purpose > retirementCommitted {
		return h, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return h, err
	}
	h.parents = &inputs{ctx: ctx, byPath: map[string]*inputDirectory{}}
	parent, err := h.parents.parents(h.path)
	if err != nil {
		return h, err
	}
	h.directory, err = openInputAt(parent, filepath.Base(h.path), true)
	if err != nil {
		return h, err
	}
	h.directoryInfo, err = h.directory.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(h.directory.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 || !nodeAccess(h.directoryInfo, 0750, 0, b.GID, true) {
		return h, errors.Join(ErrBinding, err, fsErr)
	}
	journal := number.retirementJournal(b.Request.InstallationRoot)
	journalParent, err := h.parents.parents(journal)
	if err != nil {
		return h, err
	}
	file, err := openInputAt(journalParent, filepath.Base(journal), false)
	if err != nil {
		return h, err
	}
	h.journal = &initialFile{path: journal, file: file}
	h.files[journal], h.order = h.journal, append(h.order, h.journal)
	if purpose == retirementCommitted {
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return h, err
		}
		if err := h.pendingAbsent(); err != nil {
			return h, err
		}
	}
	if err := h.readFile(h.journal, 128<<10); err != nil {
		return h, err
	}
	names := []string{"retirement-result.json"}
	if purpose == retirementReadback {
		names = append(names, "retirement-pending")
	}
	for _, name := range names {
		file, err := openInputAt(&inputDirectory{file: h.directory}, name, false)
		if err != nil {
			return h, err
		}
		original := &initialFile{path: filepath.Join(h.path, name), file: file}
		h.files[original.path], h.order = original, append(h.order, original)
		if name == "retirement-pending" {
			h.pending = original
		}
		if err := h.readFile(original, 64<<10); err != nil {
			return h, err
		}
	}
	const linuxOPath = 0x200000
	fd, err := syscall.Openat(int(h.directory.Fd()), "stop-completion.socket", linuxOPath|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return h, err
	}
	h.socket = os.NewFile(uintptr(fd), "original retirement socket metadata")
	h.socketInfo, err = h.socket.Stat()
	fsErr = syscall.Fstatfs(int(h.socket.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || !sameStartSocket(h.socketInfo, h.socketInfo, 0660, b.GID) {
		return h, errors.Join(ErrBinding, err, fsErr)
	}
	if err := h.validate(); err != nil {
		return h, err
	}
	return h, h.check()
}

func (h *retirementSnapshot) readFile(original *initialFile, maximum int64) error {
	var err error
	original.info, err = original.file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(original.file.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || !nodeAccess(original.info, 0600, 0, 0, false) || original.info.Size() < 1 || original.info.Size() > maximum {
		return errors.Join(ErrBinding, err, fsErr)
	}
	original.body, err = io.ReadAll(io.LimitReader(original.file, maximum+1))
	if err != nil || int64(len(original.body)) != original.info.Size() {
		return errors.Join(ErrBinding, err)
	}
	if h.observe != nil {
		h.observe("read", original.file)
	}
	return errors.Join(observeInitialFile(original), h.ctx.Err())
}

func (h *retirementSnapshot) validate() error {
	raw := h.files[filepath.Join(h.path, "retirement-result.json")].body
	var err error
	h.result, err = decodeRetirementResult(raw, h.number, h.binding, h.bindingSHA, h.startSHA, h.archive, h.id)
	if err != nil {
		return err
	}
	challenge, err := retirementChallenge(h.result.Challenge)
	if err != nil {
		return err
	}
	recorded, err := decodeRetirementRecords(h.journal.body, h.journal.path, h.path, h.startSHA, digest(challenge[:]), h.result.CompletionSHA256, digest(raw), h.binding.GID)
	if err != nil {
		return err
	}
	for path, record := range recorded {
		if path == filepath.Join(h.path, "retirement-pending") && h.purpose == retirementCommitted {
			continue
		}
		var info os.FileInfo
		switch path {
		case h.path:
			info = h.directoryInfo
		case filepath.Join(h.path, "stop-completion.socket"):
			info = h.socketInfo
		default:
			if file := h.files[path]; file != nil {
				info = file.info
			}
		}
		native, ok := infoSys(info)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode || uint32(info.Mode().Perm()) != record.Mode || native.Uid != record.UID || native.Gid != record.GID {
			return ErrBinding
		}
	}
	if h.pending != nil && string(h.pending.body) != "ardents-forwarding-retirement-pending-v1\n"+h.startSHA+"\n" {
		return ErrBinding
	}
	h.sha256 = digest(raw)
	return nil
}

func (h *retirementSnapshot) pendingAbsent() error {
	fd, err := syscall.Openat(int(h.directory.Fd()), "retirement-pending", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err == nil {
		return errors.Join(ErrRepairRequired, syscall.Close(fd))
	}
	if !errors.Is(err, syscall.ENOENT) {
		return errors.Join(ErrBinding, err)
	}
	return h.ctx.Err()
}

func (h *retirementSnapshot) inventory() error {
	if _, err := h.directory.Seek(0, io.SeekStart); err != nil {
		return err
	}
	wanted := 2
	if h.purpose == retirementReadback {
		wanted = 3
	}
	entries, err := h.directory.ReadDir(wanted + 1)
	if err != nil && err != io.EOF || len(entries) != wanted {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
		switch entry.Name() {
		case "retirement-result.json", "stop-completion.socket":
		case "retirement-pending":
			if h.purpose != retirementReadback {
				return ErrBinding
			}
		default:
			return ErrBinding
		}
	}
	return nil
}

func (h *retirementSnapshot) check() (returned error) {
	if h == nil || h.ctx == nil || h.closed || h.parents == nil || h.directory == nil || h.journal == nil || h.socket == nil {
		return ErrInput
	}
	if h.terminal != nil {
		return h.terminal
	}
	defer func() {
		if returned != nil {
			h.terminal = returned
		}
	}()
	if err := h.ctx.Err(); err != nil {
		return err
	}
	if err := h.parents.links(); err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		info, err := h.directory.Stat()
		named, nameErr := os.Lstat(h.path)
		if err != nil || nameErr != nil || !os.SameFile(h.directoryInfo, info) || !os.SameFile(h.directoryInfo, named) || !nodeAccess(info, 0750, 0, h.binding.GID, true) || !nodeAccess(named, 0750, 0, h.binding.GID, true) {
			return errors.Join(ErrBinding, err, nameErr)
		}
		info, err = h.socket.Stat()
		named, nameErr = os.Lstat(filepath.Join(h.path, "stop-completion.socket"))
		if err != nil || nameErr != nil || !sameStartSocket(h.socketInfo, info, 0660, h.binding.GID) || !sameStartSocket(h.socketInfo, named, 0660, h.binding.GID) {
			return errors.Join(ErrBinding, err, nameErr)
		}
		if h.purpose == retirementCommitted {
			if err := h.pendingAbsent(); err != nil {
				return err
			}
		}
		if err := h.inventory(); err != nil {
			return err
		}
		if round == 0 {
			for _, file := range h.order {
				if err := observeInitialFile(file); err != nil {
					return err
				}
				if h.observe != nil {
					h.observe("read", file.file)
				}
			}
		}
	}
	return errors.Join(h.parents.links(), h.ctx.Err())
}

func (h *retirementSnapshot) close() error {
	if h == nil {
		return nil
	}
	if !h.closed {
		h.closed = true
		closeFile := func(file *initialFile) {
			if h.observe != nil {
				h.observe("before-close", file.file)
			}
			h.terminal = errors.Join(h.terminal, file.file.Close())
			clear(file.body)
		}
		for i := len(h.order) - 1; i >= 0; i-- {
			if h.order[i] != h.journal {
				closeFile(h.order[i])
			}
		}
		if h.socket != nil {
			h.terminal = errors.Join(h.terminal, h.socket.Close())
		}
		if h.directory != nil {
			h.terminal = errors.Join(h.terminal, h.directory.Close())
		}
		if h.parents != nil {
			h.terminal = errors.Join(h.terminal, (Inputs{original: h.parents}).Close())
		}
		if h.journal != nil {
			closeFile(h.journal)
		}
		if h.ctx != nil {
			h.terminal = errors.Join(h.terminal, h.ctx.Err())
		}
	}
	return h.terminal
}
