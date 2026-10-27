package forwarding

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

// startSnapshot is independent read-only custody of one first-start transcript.
// Detached archive fields are historical observations, not live process, Stop,
// domain retirement or restart permission. The root consumer separately retains
// genuine initial/Release provenance and actual process/manager/scope custody.
type startSnapshot struct {
	ctx                    context.Context
	parents                *inputs
	directory              *os.File
	directoryInfo          os.FileInfo
	files                  map[string]*initialFile
	order                  []*initialFile
	journal                *initialFile
	socket                 *os.File
	socketInfo             os.FileInfo
	path                   string
	binding                binding
	bindingSHA, initialSHA string
	number                 attemptNumber
	predecessor            string
	archive                startArchive
	id                     [16]byte
	sha256                 string
	closed                 bool
	terminal               error
	// Failure/hold-only native observation after actual reads or before Close.
	observe func(string, *os.File)
}

func readStartSnapshotAt(ctx context.Context, b binding, number attemptNumber, bindingSHA, initialSHA, predecessor string) (h *startSnapshot, returned error) {
	h = &startSnapshot{ctx: ctx, binding: b, number: number, predecessor: predecessor, bindingSHA: bindingSHA, initialSHA: initialSHA, path: number.directory(b.Request.InstallationRoot), files: map[string]*initialFile{}}
	defer func() {
		if returned != nil {
			h.terminal = returned
		}
	}()
	if ctx == nil || os.Geteuid() != 0 || number >= attemptLimit || number == 0 && predecessor != "" || number != 0 && !validStartDigest(predecessor) || !validPath(b.Request.InstallationRoot) || b.UID < 100 || b.UID > 999 || b.GID != b.UID || !validStartDigest(bindingSHA) || !validStartDigest(initialSHA) {
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
	if err := h.pendingAbsent(); err != nil {
		return h, err
	}
	journal := number.journal(b.Request.InstallationRoot)
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
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return h, err
	}
	if err := h.readFile(h.journal, 128<<10); err != nil {
		return h, err
	}
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-archived.json"} {
		file, err := openInputAt(&inputDirectory{file: h.directory}, name, false)
		if err != nil {
			return h, err
		}
		original := &initialFile{path: filepath.Join(h.path, name), file: file}
		h.files[original.path], h.order = original, append(h.order, original)
		if err := h.readFile(original, 64<<10); err != nil {
			return h, err
		}
	}
	// O_PATH retains the actual socket inode without connecting to the stale
	// first-start endpoint or obtaining any live server/peer authority.
	const linuxOPath = 0x200000
	fd, err := syscall.Openat(int(h.directory.Fd()), "start-completion.socket", linuxOPath|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return h, err
	}
	h.socket = os.NewFile(uintptr(fd), "original first-start socket metadata")
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

func (h *startSnapshot) readFile(original *initialFile, maximum int64) error {
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

func canonicalStartRecord(raw []byte, target any) bool {
	if len(raw) == 0 || len(raw) > 64<<10 || raw[len(raw)-1] != '\n' || json.Unmarshal(raw, target) != nil {
		return false
	}
	canonical, err := json.Marshal(target)
	return err == nil && bytes.Equal(raw, append(canonical, '\n'))
}

func (h *startSnapshot) validate() error {
	get := func(name string) []byte { return h.files[filepath.Join(h.path, name)].body }
	guard, socketRaw, archiveRaw := get("start-guard.json"), get("start-socket.json"), get("start-archived.json")
	if err := checkStartIntent(guard, h.binding, h.bindingSHA, h.initialSHA, h.number, h.predecessor); err != nil {
		return err
	}
	var socket startSocketRecord
	if !canonicalStartRecord(socketRaw, &socket) || socket.Schema != "ardents-forwarding-start-socket-v1" || socket.GuardSHA256 != digest(guard) || socket.GID != h.binding.GID || socket.Device == 0 || socket.Inode == 0 {
		return ErrBinding
	}
	if !canonicalStartRecord(archiveRaw, &h.archive) {
		return ErrBinding
	}
	a := h.archive
	id, err := hex.DecodeString(a.InvocationID)
	if err != nil || len(id) != 16 || hex.EncodeToString(id) != a.InvocationID {
		return ErrBinding
	}
	copy(h.id[:], id)
	if a.Schema != "ardents-forwarding-start-archive-v1" || a.BindingSHA256 != h.bindingSHA || a.GuardSHA256 != digest(guard) || a.Generation != h.binding.Generation || a.PID == 0 || h.id == [16]byte{} || a.ProcessDevice == 0 || a.ProcessInode == 0 || a.CgroupDevice == 0 || a.CgroupInode == 0 || a.Manager == "" || a.StartedRealtime == 0 || a.StartedMonotonic == 0 {
		return ErrBinding
	}
	frame, err := completion.Encode(h.id, h.binding.Generation, h.bindingSHA)
	if err != nil {
		return err
	}
	recorded, err := decodeStartRecords(h.journal.body, h.number, h.journal.path, h.path, h.bindingSHA, digest(guard), digest(socketRaw), digest(archiveRaw), digest(frame[:]), h.binding.GID)
	if err != nil {
		return err
	}
	for path, record := range recorded {
		if path == filepath.Join(h.path, "forwarding-start-pending") {
			continue
		} // original publication intentionally unlinked it
		var info os.FileInfo
		switch path {
		case h.path:
			info = h.directoryInfo
		case filepath.Join(h.path, "start-completion.socket"):
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
	native, _ := infoSys(h.socketInfo)
	if socket.Device != uint64(native.Dev) || socket.Inode != native.Ino {
		return ErrBinding
	}
	h.sha256 = digest(archiveRaw)
	return nil
}

func (h *startSnapshot) pendingAbsent() (returned error) {
	fd, err := syscall.Openat(int(h.directory.Fd()), "forwarding-start-pending", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err == nil {
		return errors.Join(ErrRepairRequired, syscall.Close(fd))
	}
	if !errors.Is(err, syscall.ENOENT) {
		return errors.Join(ErrBinding, err)
	}
	return h.ctx.Err()
}

func (h *startSnapshot) inventory() error {
	if _, err := h.directory.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := h.directory.ReadDir(5)
	if err != nil && err != io.EOF || len(entries) != 4 {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
		switch entry.Name() {
		case "start-guard.json", "start-socket.json", "start-archived.json", "start-completion.socket":
		default:
			return ErrBinding
		}
	}
	return nil
}

func (h *startSnapshot) check() (returned error) {
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
		named, nameErr = os.Lstat(filepath.Join(h.path, "start-completion.socket"))
		if err != nil || nameErr != nil || !sameStartSocket(h.socketInfo, info, 0660, h.binding.GID) || !sameStartSocket(h.socketInfo, named, 0660, h.binding.GID) {
			return errors.Join(ErrBinding, err, nameErr)
		}
		if err := h.pendingAbsent(); err != nil {
			return err
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

func (h *startSnapshot) close() error {
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
		} // this independently opened journal lease returns last
		if h.ctx != nil {
			h.terminal = errors.Join(h.terminal, h.ctx.Err())
		}
	}
	return h.terminal
}
