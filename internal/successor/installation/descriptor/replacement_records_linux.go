package descriptor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
)

// Exact finite journal grammar. Complete bytes are only provenance: readers
// separately match original inodes, complete retirement and fresh permission.
func decodeReplacementRecords(raw []byte, next, previous generationSelection, hashes map[string]string, before, after []byte) (map[string]initialRecord, error) {
	if next.matchPredecessor(previous) != nil || len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' || compareReloadContexts(before, after) != nil {
		return nil, ErrBinding
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 62 {
		return nil, ErrBinding
	}
	records := make([]initialRecord, len(lines))
	for i, line := range lines {
		if json.Unmarshal(line, &records[i]) != nil {
			return nil, ErrBinding
		}
		canonical, _ := json.Marshal(records[i])
		if !bytes.Equal(line, canonical) || records[i].Sequence != i {
			return nil, ErrBinding
		}
	}
	pos := 0
	owned := map[string]initialRecord{}
	take := func(w initialRecord, native bool) (initialRecord, error) {
		if pos >= len(records) {
			return initialRecord{}, ErrBinding
		}
		a := records[pos]
		w.Sequence = pos
		if native {
			if a.Device == 0 || a.Inode == 0 {
				return initialRecord{}, ErrBinding
			}
			w.Device, w.Inode = a.Device, a.Inode
		}
		if a != w {
			return initialRecord{}, fmt.Errorf("descriptor replacement journal row %d (%s): %w", pos, w.Phase, ErrBinding)
		}
		pos++
		return a, nil
	}
	b := next.binding
	root := b.Request.InstallationRoot
	journal := replacementJournal(root, b.Generation)
	sealed := filepath.Join(root, "generations", b.Generation)
	first, err := take(initialRecord{Phase: "descriptor-replacement-v1", Path: journal, Mode: 0600}, true)
	if err != nil {
		return nil, err
	}
	owned[journal] = first
	history, err := take(initialRecord{Phase: "replacement-admitted", Path: b.History, SHA256: digest(next.raw), Mode: 0700}, true)
	if err != nil {
		return nil, err
	}
	owned[b.History] = history
	if _, err = take(initialRecord{Phase: "native-context", Path: root, SHA256: digest(before)}, false); err != nil {
		return nil, err
	}
	for _, v := range []struct {
		phase, path, sha string
		mode             uint32
	}{
		{"predecessor-binding", filepath.Join(root, "binding.json"), digest(previous.raw), 0644},
		{"predecessor-unit", "/etc/systemd/system/ardents-descriptor.service", previous.binding.UnitSHA256, 0644},
		{"predecessor-retired", filepath.Join(attemptNumber(next.replacement.PredecessorAttempt).retirement(root), "retirement-result.json"), next.replacement.RetirementSHA256, 0600},
	} {
		r, e := take(initialRecord{Phase: v.phase, Path: v.path, SHA256: v.sha, Mode: v.mode}, true)
		if e != nil {
			return nil, e
		}
		owned[v.path] = r
	}
	write := func(path, sha string, mode uint32) error {
		if !initialSHA(sha) {
			return ErrBinding
		}
		if _, e := take(initialRecord{Phase: "file-intent", Path: path, SHA256: sha, Mode: mode}, false); e != nil {
			return e
		}
		born, e := take(initialRecord{Phase: "file-born", Path: path, Mode: 0600}, true)
		if e != nil {
			return e
		}
		born.Phase, born.SHA256, born.Mode = "file-ready", sha, mode
		r, e := take(born, false)
		if e == nil {
			owned[path] = r
		}
		return e
	}
	beforePath, afterPath := replacementNativePaths(journal)
	if err = write(replacementPending(root), digest(replacementPendingBytes(next.raw)), 0600); err != nil {
		return nil, err
	}
	if err = write(beforePath, digest(before), 0600); err != nil {
		return nil, err
	}
	if _, err = take(initialRecord{Phase: "directory-intent", Path: sealed, Mode: 0755}, false); err != nil {
		return nil, err
	}
	born, err := take(initialRecord{Phase: "directory-born", Path: sealed, Mode: 0700}, true)
	if err != nil {
		return nil, err
	}
	born.Phase, born.Mode = "directory-ready", 0755
	if _, err = take(born, false); err != nil {
		return nil, err
	}
	names := descriptorGenerationNames()
	slices.Sort(names)
	for _, name := range names {
		mode := uint32(0444)
		if name == programName {
			mode = 0555
		}
		if err = write(filepath.Join(sealed, name), hashes[filepath.Join(sealed, name)], mode); err != nil {
			return nil, err
		}
	}
	born.Phase, born.Mode, born.SHA256 = "seal-intent", 0555, b.Generation
	if _, err = take(born, false); err != nil {
		return nil, err
	}
	born.Phase = "sealed"
	if r, e := take(born, false); e != nil {
		return nil, e
	} else {
		owned[sealed] = r
	}
	for _, v := range []struct{ path, sha, intent, done string }{
		{filepath.Join(root, "binding.json"), digest(next.raw), "selection-intent", "selection-written"},
		{"/etc/systemd/system/ardents-descriptor.service", b.UnitSHA256, "unit-intent", "unit-written"},
	} {
		r, e := take(initialRecord{Phase: v.intent, Path: v.path, SHA256: v.sha, Mode: 0644}, true)
		if e != nil {
			return nil, e
		}
		old := owned[v.path]
		if old.Device != r.Device || old.Inode != r.Inode {
			return nil, ErrBinding
		}
		r.Phase = v.done
		if _, e = take(r, false); e != nil {
			return nil, e
		}
		owned[v.path] = r
	}
	if _, err = take(initialRecord{Phase: "reload-intent", Path: "/etc/systemd/system/ardents-descriptor.service", SHA256: b.UnitSHA256, Mode: 0644}, false); err != nil {
		return nil, err
	}
	if err = write(afterPath, digest(after), 0600); err != nil {
		return nil, err
	}
	if _, err = take(initialRecord{Phase: "replacement-verified", Path: filepath.Join(root, "binding.json"), SHA256: digest(next.raw), Mode: 0644}, false); err != nil {
		return nil, err
	}
	if pos != len(records) {
		return nil, ErrBinding
	}
	return owned, nil
}
