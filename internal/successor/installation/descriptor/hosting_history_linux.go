package descriptor

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// A completed allowance is independently observed under its own journal lease.
// Only immutable birth pin/lock files are retained; runtime budget mutation and
// accounting remain Hosting policy. An incomplete or uncertain prefix refuses.
func (c *completedInitial) readHostingPreparation() error {
	b := c.binding
	directory, journal, _ := hostingPreparationPaths(b.Request.InstallationRoot)
	if err := c.directory(directory, 0700, 0, 0); err != nil {
		return err
	}
	j, err := c.read(journal, 128<<10, 0600, 0, 0)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(j.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	if err := observeInitialFile(j); err != nil {
		return err
	}
	i, err := c.read(filepath.Join(directory, "intent.json"), 4096, 0600, 0, 0)
	if err != nil {
		return err
	}
	r, err := c.read(filepath.Join(directory, "completed.json"), 4096, 0600, 0, 0)
	if err != nil {
		return err
	}
	var intent hostingBirth
	var result hostingCompletion
	if json.Unmarshal(i.body, &intent) != nil || json.Unmarshal(r.body, &result) != nil {
		return ErrBinding
	}
	iraw, _ := json.Marshal(intent)
	rraw, _ := json.Marshal(result)
	if !bytes.Equal(i.body, append(iraw, '\n')) || !bytes.Equal(r.body, append(rraw, '\n')) || intent.Schema != "ardents-descriptor-hosting-intent-v1" || result.Schema != "ardents-descriptor-hosting-completed-v1" || intent.Binding != digest(c.files[filepath.Join(b.Request.InstallationRoot, "binding.json")].body) || result.Intent != digest(i.body) || !initialSHA(intent.Policy) {
		return ErrBinding
	}
	native, ok := infoSys(c.directoryAt(b.Request.HostingRoot).info)
	if !ok || intent.Device != uint64(native.Dev) || intent.Inode != native.Ino {
		return ErrBinding
	}
	for name, wanted := range map[string]hostingFileIdentity{"budget.pin": result.Pin, "budget.lock": result.Lock} {
		size := int64(0)
		if name == "budget.pin" {
			size = 64
		}
		f, err := c.readAt(c.directoryAt(b.Request.HostingRoot), filepath.Join(b.Request.HostingRoot, name), size, 0600, b.UID, b.GID)
		if err != nil {
			return err
		}
		native, ok := infoSys(f.info)
		if !ok || uint64(native.Dev) != wanted.Device || native.Ino != wanted.Inode || int64(len(f.body)) != size || name == "budget.pin" && string(f.body) != intent.Policy {
			return ErrBinding
		}
	}
	records, err := decodeHostingRecords(j.body, b, i.body, r.body)
	if err != nil {
		return err
	}
	for path, record := range records {
		if path == b.Request.InstallationRoot+".descriptor-hosting-pending" {
			continue
		}
		var info os.FileInfo
		if f := c.files[path]; f != nil {
			info = f.info
		} else if d := c.directoryAt(path); d != nil {
			info = d.info
		}
		native, ok := infoSys(info)
		if !ok || native.Ino != record.Inode || uint64(native.Dev) != record.Device || !nodeAccess(info, os.FileMode(record.Mode), record.UID, record.GID, info.IsDir()) {
			return ErrBinding
		}
	}
	c.hostingPrepared = true
	return nil
}

func (c *completedInitial) checkHostingInventory() error {
	if !c.hostingPrepared {
		return nil
	}
	directory, _, _ := hostingPreparationPaths(c.binding.Request.InstallationRoot)
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if entry.Name() != "intent.json" && entry.Name() != "completed.json" {
			return ErrBinding
		}
	}
	return nil
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
	first, err := next(initialRecord{Phase: "descriptor-hosting-v1", Path: journal, Mode: 0600}, true)
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
	if err := write(pending, []byte("ardents-descriptor-hosting-pending-v1\n"+digest(intentRaw)+"\n")); err != nil {
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
