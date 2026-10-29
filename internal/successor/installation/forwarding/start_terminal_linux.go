package forwarding

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

// A terminal entry is never a runtime snapshot. It owns independent original,
// cut, completion and physical resources under its parent history observation.
type startTerminalSnapshot struct {
	ctx                                                                 context.Context
	owner                                                               *completedInitial
	parents                                                             *inputs
	selection                                                           generationSelection
	prefix                                                              *neverStartedPrefix
	journal, cutControl, cutResult, control, completion, guard, pending *initialFile
	files                                                               map[string]*initialFile
	order                                                               []*initialFile
	directory, socket                                                   *os.File
	directoryInfo, socketInfo                                           os.FileInfo
	path                                                                string
	cut                                                                 transitionCutResult
	original                                                            *recoveryOriginal
	logical, nativeRaw                                                  []byte
	paths                                                               []string
	sha256                                                              string
	closed                                                              bool
	terminal                                                            error
	observe                                                             func(string, *os.File) error
}

func readStartTerminal(ctx context.Context, c *completedInitial, selected generationSelection, number attemptNumber, initial, predecessor string, everRuntime bool) (h *startTerminalSnapshot, returned error) {
	if ctx == nil || c == nil || c.files == nil || os.Geteuid() != 0 || number >= attemptLimit || !validStartDigest(initial) {
		return nil, ErrInput
	}
	h = &startTerminalSnapshot{ctx: ctx, owner: c, selection: selected, parents: &inputs{ctx: ctx, byPath: map[string]*inputDirectory{}}, files: map[string]*initialFile{}, path: number.directory(selected.binding.Request.InstallationRoot)}
	defer func() {
		if returned != nil {
			h.terminal = returned
		}
	}()
	journal := number.journal(selected.binding.Request.InstallationRoot)
	if active := c.files.interrupted; active != nil && active.cut.Boundary.Operation == "start" && active.cut.Boundary.Journal == journal {
		h.original = active
	}
	var err error
	h.journal, err = h.rootFile(journal, 128<<10, h.original == nil)
	if err != nil {
		// Even a malformed original stays the last physical file closed once
		// acquired; partial read failure does not release its lease early.
		h.journal = h.files[journal]
		return h, err
	}
	if h.original != nil {
		if err := h.original.check(); err != nil {
			return h, err
		}
		if !sameInputFile(h.journal.info, h.original.prefix.journal.info) || !bytes.Equal(h.journal.body, h.original.prefix.journal.body) {
			return h, ErrBinding
		}
	}
	cc, cr, _ := transitionCutPaths(journal)
	h.cutControl, err = h.rootFile(cc, 64<<10, true)
	if err != nil {
		return h, err
	}
	h.cutResult, err = h.rootFile(cr, 256<<10, false)
	if err != nil {
		return h, err
	}
	h.cut, err = decodeTransitionCut(h.cutControl.body, h.cutResult.body, h.journal.body, h.cutControl.info, h.cutResult.info, h.journal.info)
	if err != nil {
		return h, err
	}
	if !bytes.Equal(h.cut.Boundary.Binding, selected.raw) {
		return h, ErrBinding
	}
	if h.original != nil && !bytes.Equal(h.cutResult.body, h.original.result.body) {
		return h, ErrBinding
	}
	if err := h.openDirectory(); err != nil {
		return h, err
	}
	var legacy []byte
	if h.cut.Boundary.Schema == "ardents-forwarding-transition-boundary-v1" {
		guard, err := h.childFile("start-guard.json", 8<<10, false)
		if err != nil {
			return h, err
		}
		legacy = guard.body
	}
	h.prefix, err = classifyNeverStarted(h.journal.body, h.cut, selected, legacy)
	if err != nil {
		return h, err
	}
	wanted := uint8(0)
	if everRuntime {
		wanted = 1
	}
	if h.prefix.number != number || h.prefix.history.initial != initial || h.prefix.history.predecessor != predecessor || h.prefix.history.history != wanted {
		return h, ErrBinding
	}
	h.nativeRaw = bytes.Clone(h.cut.Boundary.NativeContext)
	h.paths = []string{cc, cr}
	known := c.files.recoveryReadback
	if known != nil && (known.completion.Operation != "start" || known.completion.Journal != journal) {
		known = nil
	}
	for path, record := range h.prefix.owned {
		if path == journal || path == h.path || path == filepath.Join(h.path, "start-completion.socket") {
			continue
		}
		if path == filepath.Join(h.path, "forwarding-start-pending") && h.original == nil && known == nil {
			continue
		}
		file := h.files[path]
		if file == nil {
			file, err = h.childFile(filepath.Base(path), 64<<10, record.Phase == "file-born")
			if err != nil {
				return h, err
			}
		}
		if filepath.Base(path) == "forwarding-start-pending" {
			h.pending = file
		}
		wanted := h.prefix.bodies[path]
		if record.Phase == "file-born" {
			wanted = nil
		}
		if !bytes.Equal(file.body, wanted) {
			return h, ErrBinding
		}
	}
	if record, ok := h.prefix.owned[filepath.Join(h.path, "start-completion.socket")]; ok {
		const oPath = 0x200000
		fd, err := syscall.Openat(int(h.directory.Fd()), "start-completion.socket", oPath|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			return h, err
		}
		h.socket = os.NewFile(uintptr(fd), "original never-started socket metadata")
		h.socketInfo, err = h.socket.Stat()
		if err != nil || !sameStartSocket(h.socketInfo, h.socketInfo, os.FileMode(record.Mode), record.GID) {
			return h, errors.Join(ErrBinding, err)
		}
	}
	if h.original != nil {
		if _, born := h.prefix.owned[h.path]; !born && h.directory != nil {
			return h, ErrBinding
		}
		if err := recoveryAbsent(journal); err != nil {
			return h, err
		}
		return h, h.check()
	}
	control, result, pending := recoveryPaths(journal)
	h.control, err = h.rootFile(control, 64<<10, true)
	if err != nil {
		return h, err
	}
	h.completion, err = h.rootFile(result, 4<<20, false)
	if err != nil {
		return h, err
	}
	envelope, logical, err := decodeRecoveryCompletion(h.completion.body, h.journal.body, h.cutResult.body, h.cut)
	if err != nil {
		return h, err
	}
	h.logical = logical
	if err := h.validateContinuation(envelope); err != nil {
		return h, err
	}
	var guardInfo os.FileInfo
	if known != nil {
		if known.births == nil || !known.births.closed || known.births.terminal != nil || known.pending == nil || !bytes.Equal(known.raw, h.completion.body) {
			return h, ErrBinding
		}
		h.guard, err = h.rootFile(pending, 128, false)
		if err != nil || !sameInputFile(h.guard.info, known.pending.info) || !bytes.Equal(h.guard.body, recoveryPendingBytes(envelope.OriginalCutSHA256)) {
			return h, errors.Join(ErrBinding, err)
		}
		if h.pending == nil && known.originalPending != nil || h.pending != nil && (known.originalPending == nil || !sameInputFile(h.pending.info, known.originalPending.info)) {
			return h, ErrBinding
		}
		guardInfo = h.guard.info
		h.paths = append(h.paths, pending)
	}
	sealed := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation)
	unit := c.files.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil || c.files.dirs[sealed] == nil {
		return h, ErrBinding
	}
	if err := decodeStartRecoveryControl(h.control.body, h.completion.body, envelope, selected, h.control.info, h.completion.info, h.journal.info, c.files.dirs[sealed].info, unit.info, guardInfo, h.directoryInfo); err != nil {
		return h, err
	}
	h.paths = append(h.paths, control, result)
	h.sha256 = digest(h.completion.body)
	return h, h.check()
}

func (h *startTerminalSnapshot) openDirectory() error {
	parent, err := h.parents.parents(h.path)
	if err != nil {
		return err
	}
	file, err := openInputAt(parent, filepath.Base(h.path), true)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	h.directory = file
	h.directoryInfo, err = file.Stat()
	if err != nil {
		return err
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (h *startTerminalSnapshot) rootFile(path string, maximum int64, exclusive bool) (*initialFile, error) {
	parent, err := h.parents.parents(path)
	if err != nil {
		return nil, err
	}
	return h.retainFile(parent, filepath.Base(path), path, maximum, exclusive, false)
}

func (h *startTerminalSnapshot) childFile(name string, maximum int64, empty bool) (*initialFile, error) {
	if h.directory == nil {
		return nil, ErrBinding
	}
	return h.retainFile(&inputDirectory{file: h.directory}, name, filepath.Join(h.path, name), maximum, false, empty)
}

func (h *startTerminalSnapshot) retainFile(parent *inputDirectory, name, path string, maximum int64, exclusive, empty bool) (*initialFile, error) {
	if h.files[path] != nil {
		return nil, ErrBinding
	}
	file, err := openInputAt(parent, name, false)
	if err != nil {
		return nil, err
	}
	owned := &initialFile{path: path, file: file}
	h.files[path] = owned
	h.order = append(h.order, owned)
	if exclusive {
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return nil, err
		}
	}
	owned.info, err = file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 || !nodeAccess(owned.info, 0600, 0, 0, false) || owned.info.Size() > maximum || owned.info.Size() < 0 || !empty && owned.info.Size() == 0 {
		return nil, errors.Join(ErrBinding, err, fsErr)
	}
	owned.body, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(owned.body)) != owned.info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	if h.observe != nil {
		if err := h.observe("read", file); err != nil {
			return nil, err
		}
	}
	return owned, errors.Join(observeInitialFile(owned), h.ctx.Err())
}

func (h *startTerminalSnapshot) validateContinuation(envelope recoveryCompletion) error {
	if h.directoryInfo == nil {
		return ErrBinding
	}
	_, originalBorn := h.prefix.owned[h.path]
	count := 1
	if !originalBorn {
		count = 4
	}
	if len(envelope.Records) != count {
		return ErrBinding
	}
	var rows []startEntry
	for _, raw := range envelope.Records {
		var row startEntry
		if json.Unmarshal(raw, &row) != nil {
			return ErrBinding
		}
		rows = append(rows, row)
	}
	native, ok := infoSys(h.directoryInfo)
	if !ok {
		return ErrBinding
	}
	position := uint32(len(h.prefix.rows))
	if !originalBorn {
		if !nodeAccess(h.directoryInfo, 0750, 0, h.selection.binding.GID, true) {
			return ErrBinding
		}
		wanted := []startEntry{{Phase: "terminal-directory-intent", Path: h.path}, {Phase: "terminal-directory-born", Path: h.path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0700}, {Phase: "terminal-directory-ready", Path: h.path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0750, GID: h.selection.binding.GID}}
		for n, row := range wanted {
			row.Schema = "ardents-forwarding-start-journal-v1"
			row.Sequence = position + uint32(n)
			if row != rows[n] {
				return ErrBinding
			}
		}
		position += 3
	}
	wanted := startEntry{Schema: "ardents-forwarding-start-journal-v1", Sequence: position, Phase: "stopped-before-start", Path: h.path, Device: uint64(native.Dev), Inode: native.Ino, SHA256: digest(h.prefix.intent), Mode: uint32(h.directoryInfo.Mode().Perm()), UID: native.Uid, GID: native.Gid}
	if rows[len(rows)-1] != wanted {
		return ErrBinding
	}
	return nil
}

func (h *startTerminalSnapshot) check() (returned error) {
	if h == nil || h.closed || h.prefix == nil {
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
	if err := errors.Join(h.ctx.Err(), h.parents.links()); err != nil {
		return err
	}
	if h.original != nil {
		if err := h.original.check(); err != nil {
			return err
		}
		if !sameInputFile(h.journal.info, h.original.prefix.journal.info) || !bytes.Equal(h.journal.body, h.original.prefix.journal.body) {
			return ErrBinding
		}
	} else if h.completion == nil || h.sha256 == "" {
		return ErrBinding
	}
	for _, file := range h.order {
		if err := observeInitialFile(file); err != nil {
			return err
		}
	}
	for path, record := range h.prefix.owned {
		if path == filepath.Join(h.path, "forwarding-start-pending") && h.pending == nil {
			continue
		}
		var info os.FileInfo
		if file := h.files[path]; file != nil {
			info = file.info
		} else if path == h.path {
			info = h.directoryInfo
		} else if path == filepath.Join(h.path, "start-completion.socket") {
			info = h.socketInfo
		}
		native, ok := infoSys(info)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode || uint32(info.Mode().Perm()) != record.Mode || native.Uid != record.UID || native.Gid != record.GID {
			return ErrBinding
		}
	}
	if h.directory != nil {
		info, err := h.directory.Stat()
		named, nameErr := os.Lstat(h.path)
		if err != nil || nameErr != nil || !sameInputFile(h.directoryInfo, info) || !sameInputFile(h.directoryInfo, named) {
			return errors.Join(ErrBinding, err, nameErr)
		}
	} else if err := h.checkUnbornDirectory(); err != nil {
		return err
	}
	if h.socket != nil {
		info, err := h.socket.Stat()
		named, nameErr := os.Lstat(filepath.Join(h.path, "start-completion.socket"))
		if err != nil || nameErr != nil || !sameStartSocket(h.socketInfo, info, h.socketInfo.Mode().Perm(), h.socketInfo.Sys().(*syscall.Stat_t).Gid) || !os.SameFile(h.socketInfo, named) {
			return errors.Join(ErrBinding, err, nameErr)
		}
	}
	if err := h.inventory(); err != nil {
		return err
	}
	_, _, cutPending := transitionCutPaths(h.journal.path)
	for _, path := range []string{cutPending, h.journal.path + ".recovery-native-context"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	if h.original == nil && h.guard == nil {
		_, _, pending := recoveryPaths(h.journal.path)
		if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	return errors.Join(h.parents.links(), h.ctx.Err())
}

func (h *startTerminalSnapshot) checkUnbornDirectory() error {
	p := h.owner.files.recoveryPublishing
	if h.original != nil && p != nil && p.births != nil && p.completion.Journal == h.journal.path && p.births.dirs[h.path] != nil {
		return p.births.check()
	}
	if _, err := os.Lstat(h.path); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (h *startTerminalSnapshot) inventory() error {
	dir := h.directory
	if dir == nil {
		p := h.owner.files.recoveryPublishing
		if h.original != nil && p != nil && p.births != nil && p.completion.Journal == h.journal.path {
			dir = p.births.dirs[h.path]
		}
		if dir == nil {
			return nil
		}
	}
	if _, err := dir.Seek(0, io.SeekStart); err != nil {
		return err
	}
	wanted := map[string]bool{}
	for path := range h.prefix.owned {
		if filepath.Dir(path) == h.path && (filepath.Base(path) != "forwarding-start-pending" || h.pending != nil) {
			wanted[filepath.Base(path)] = true
		}
	}
	entries, err := dir.ReadDir(len(wanted) + 1)
	if err != nil && err != io.EOF || len(entries) != len(wanted) {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if !wanted[entry.Name()] || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return ErrBinding
		}
	}
	return nil
}

func (h *startTerminalSnapshot) close() error {
	if h == nil {
		return nil
	}
	if !h.closed {
		h.closed = true
		closeFile := func(f *initialFile) {
			if h.observe != nil {
				h.terminal = errors.Join(h.terminal, h.observe("before-close", f.file))
			}
			h.terminal = errors.Join(h.terminal, f.file.Close())
			clear(f.body)
		}
		for n := len(h.order) - 1; n >= 0; n-- {
			if h.order[n] != h.journal {
				closeFile(h.order[n])
			}
		}
		if h.socket != nil {
			h.terminal = errors.Join(h.terminal, h.socket.Close())
		}
		if h.directory != nil {
			h.terminal = errors.Join(h.terminal, h.directory.Close())
		}
		h.terminal = errors.Join(h.terminal, (Inputs{original: h.parents}).Close())
		if h.journal != nil {
			closeFile(h.journal)
		}
		h.prefix.clear()
		clear(h.logical)
		clear(h.nativeRaw)
		h.terminal = errors.Join(h.terminal, h.ctx.Err())
	}
	return h.terminal
}
