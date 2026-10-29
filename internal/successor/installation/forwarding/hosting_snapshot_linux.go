package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Root Start independently attaches its own immutable preparation observation
// to Completed. No creator handle, live budget, accepting callback or mutable
// budget.json is borrowed. Native pin/lock identities and the one-use transcript
// remain retained through the original root attempt; Node opens its own budget.
func (c *completedInitial) readHostingPreparation(bindingPath string) error {
	b, f := c.initial.binding, c.files
	if len(c.initial.raw) == 0 || bindingPath != filepath.Join(c.binding.Request.InstallationRoot, "binding.json") {
		return ErrBinding
	}
	directory, journal, pending := hostingPreparationPaths(b.Request.InstallationRoot)
	if _, err := f.parents(pending); err != nil {
		return err
	}
	if err := f.absent(pending); err != nil {
		return err
	}
	f.absentPaths = append(f.absentPaths, pending)
	if err := f.directory(directory, 0700, 0, 0); err != nil {
		return err
	}
	journalRaw, err := f.read(journal, 128<<10, 0600, 0, 0)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.files[journal].file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	intentPath, resultPath := filepath.Join(directory, "intent.json"), filepath.Join(directory, "completed.json")
	intentRaw, err := f.read(intentPath, 4096, 0600, 0, 0)
	if err != nil {
		return err
	}
	resultRaw, err := f.read(resultPath, 4096, 0600, 0, 0)
	if err != nil {
		return err
	}
	var intent hostingBirth
	var result hostingCompletion
	if json.Unmarshal(intentRaw, &intent) != nil || json.Unmarshal(resultRaw, &result) != nil {
		return ErrBinding
	}
	i, _ := json.Marshal(intent)
	r, _ := json.Marshal(result)
	if !bytes.Equal(intentRaw, append(i, '\n')) || !bytes.Equal(resultRaw, append(r, '\n')) || intent.Schema != "ardents-forwarding-hosting-intent-v1" || result.Schema != "ardents-forwarding-hosting-completed-v1" || intent.Binding != digest(c.initial.raw) || result.Intent != digest(intentRaw) || !initialSHA(intent.Policy) {
		return ErrBinding
	}
	rootNative := f.dirs[b.Request.HostingRoot].info.Sys().(*syscall.Stat_t)
	if intent.Device != uint64(rootNative.Dev) || intent.Inode != rootNative.Ino {
		return ErrBinding
	}
	pinPath, lockPath := filepath.Join(b.Request.HostingRoot, "budget.pin"), filepath.Join(b.Request.HostingRoot, "budget.lock")
	pinFile, err := f.retainHostingFile(pinPath, 64, b.UID, b.GID)
	if err != nil {
		return err
	}
	pin, err := f.fill(pinFile, 64)
	if err != nil || string(pin) != intent.Policy {
		return errors.Join(ErrBinding, err)
	}
	if _, err := f.retainHostingFile(lockPath, 0, b.UID, b.GID); err != nil {
		return err
	}
	for path, wanted := range map[string]hostingFileIdentity{pinPath: result.Pin, lockPath: result.Lock} {
		native := f.files[path].info.Sys().(*syscall.Stat_t)
		if wanted.Device != uint64(native.Dev) || wanted.Inode != native.Ino {
			return ErrBinding
		}
	}
	owned, err := decodeHostingRecords(journalRaw, b, intentRaw, resultRaw)
	if err != nil {
		return err
	}
	for path, record := range owned {
		if path == pending {
			continue
		}
		var info os.FileInfo
		if file := f.files[path]; file != nil {
			info = file.info
		} else if dir := f.dirs[path]; dir != nil {
			info = dir.info
		}
		if info == nil || !nodeAccess(info, os.FileMode(record.Mode), record.UID, record.GID, info.IsDir()) {
			return ErrBinding
		}
		native := info.Sys().(*syscall.Stat_t)
		if record.Device != uint64(native.Dev) || record.Inode != native.Ino {
			return ErrBinding
		}
	}
	if f.extraInventories == nil {
		f.extraInventories = map[string]int{}
	}
	f.extraInventories[directory] = 2
	c.hostingPolicy = intent.Policy
	return f.check()
}

func (f *completedFiles) retainHostingFile(path string, size int64, uid, gid uint32) (*initialFile, error) {
	if f.files[path] != nil || size != 0 && size != 64 {
		return nil, ErrInput
	}
	parent := f.dirs[filepath.Dir(path)]
	if parent == nil || !nodeAccess(parent.info, 0700, uid, gid, true) {
		return nil, ErrBinding
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original := &initialFile{path: path, file: file}
	f.files[path] = original
	f.fileOrder = append(f.fileOrder, original)
	original.info, err = file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || !nodeAccess(original.info, 0600, uid, gid, false) || original.info.Size() != size {
		return nil, errors.Join(ErrBinding, err, fsErr)
	}
	return original, f.local()
}

func decodeHostingRecords(raw []byte, b binding, intentRaw, resultRaw []byte) (map[string]initialRecord, error) {
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' {
		return nil, ErrBinding
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 16 {
		return nil, ErrBinding
	}
	records := make([]initialRecord, len(lines))
	for n, line := range lines {
		if len(line) > 32<<10 || json.Unmarshal(line, &records[n]) != nil {
			return nil, ErrBinding
		}
		canonical, _ := json.Marshal(records[n])
		if !bytes.Equal(canonical, line) || records[n].Sequence != n {
			return nil, ErrBinding
		}
	}
	var intent hostingBirth
	if json.Unmarshal(intentRaw, &intent) != nil {
		return nil, ErrBinding
	}
	directory, journal, pending := hostingPreparationPaths(b.Request.InstallationRoot)
	position := 0
	owned := map[string]initialRecord{}
	next := func(wanted initialRecord, native bool) (initialRecord, error) {
		if position >= len(records) {
			return initialRecord{}, ErrBinding
		}
		actual := records[position]
		wanted.Sequence = position
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return initialRecord{}, ErrBinding
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
		}
		if actual != wanted {
			return initialRecord{}, ErrBinding
		}
		position++
		return actual, nil
	}
	first, err := next(initialRecord{Phase: "forwarding-hosting-v1", Path: journal, Mode: 0600}, true)
	if err != nil {
		return nil, err
	}
	owned[journal] = first
	write := func(path string, body []byte) error {
		sha := digest(body)
		if _, err := next(initialRecord{Phase: "file-intent", Path: path, SHA256: sha, Mode: 0600}, false); err != nil {
			return err
		}
		born, err := next(initialRecord{Phase: "file-born", Path: path, Mode: 0600}, true)
		if err != nil {
			return err
		}
		born.Phase, born.SHA256 = "file-ready", sha
		ready, err := next(born, false)
		if err == nil {
			owned[path] = ready
		}
		return err
	}
	if err := write(pending, []byte("ardents-forwarding-hosting-pending-v1\n"+digest(intentRaw)+"\n")); err != nil {
		return nil, err
	}
	if _, err := next(initialRecord{Phase: "directory-intent", Path: directory, Mode: 0700}, false); err != nil {
		return nil, err
	}
	born, err := next(initialRecord{Phase: "directory-born", Path: directory, Mode: 0700}, true)
	if err != nil {
		return nil, err
	}
	born.Phase = "directory-ready"
	ready, err := next(born, false)
	if err != nil {
		return nil, err
	}
	owned[directory] = ready
	if err := write(filepath.Join(directory, "intent.json"), intentRaw); err != nil {
		return nil, err
	}
	root := initialRecord{Phase: "admitted", Path: b.Request.HostingRoot, Device: intent.Device, Inode: intent.Inode, SHA256: intent.Policy, Mode: 0700, UID: b.UID, GID: b.GID}
	if _, err := next(root, false); err != nil {
		return nil, err
	}
	owned[root.Path] = root
	root.Phase, root.SHA256 = "selected", digest(resultRaw)
	if _, err := next(root, false); err != nil {
		return nil, err
	}
	if err := write(filepath.Join(directory, "completed.json"), resultRaw); err != nil {
		return nil, err
	}
	ready.Phase, ready.SHA256 = "completed", digest(resultRaw)
	if _, err := next(ready, false); err != nil {
		return nil, err
	}
	if position != len(records) {
		return nil, ErrBinding
	}
	return owned, nil
}
