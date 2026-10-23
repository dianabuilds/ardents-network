package infrastructure

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
)

// This closed transcript authenticates recorded configuration/effect ordering,
// never Release, a successful original join or continuation. Its root reader
// independently checks native identities, complete predecessor start/retirement
// histories and the unchanged original semantic roots before accepting it.
func decodeSourceReplacementRecords(raw []byte, replacement sourceReplacementBinding, hashes map[string]string, admitted, previousUnit string) (map[string]sourceRecord, error) {
	if len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' || !sourceInitialSHA(admitted) || !sourceInitialSHA(previousUnit) {
		return nil, ErrBinding
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 55 {
		return nil, ErrBinding
	}
	records := make([]sourceRecord, len(lines))
	for i, line := range lines {
		if len(line) > 32<<10 || json.Unmarshal(line, &records[i]) != nil {
			return nil, ErrBinding
		}
		canonical, err := json.Marshal(records[i])
		if err != nil || !bytes.Equal(canonical, line) || records[i].Sequence != i {
			return nil, ErrBinding
		}
	}
	position := 0
	next := func(wanted sourceRecord, native bool) (sourceRecord, error) {
		if position >= len(records) {
			return sourceRecord{}, ErrBinding
		}
		actual := records[position]
		wanted.Sequence = position
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return sourceRecord{}, ErrBinding
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
		}
		if actual != wanted {
			return sourceRecord{}, ErrBinding
		}
		position++
		return actual, nil
	}
	b := replacement.Configuration
	root, sealed := b.Request.InstallationRoot, filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	selected, unit := filepath.Join(root, "binding.json"), "/etc/systemd/system/ardents-source.service"
	journal := sourceReplacementJournal(root, b.Generation)
	first, err := next(sourceRecord{Phase: "initial", Path: journal, Mode: 0600}, true)
	if err != nil {
		return nil, err
	}
	owned := map[string]sourceRecord{journal: first}
	if _, err = next(sourceRecord{Phase: "replacement-admitted", Path: root, SHA256: admitted, UID: b.UID, GID: b.GID}, false); err != nil {
		return nil, err
	}
	preimages := map[string]sourceRecord{}
	for _, wanted := range []sourceRecord{
		{Phase: "predecessor-binding", Path: selected, SHA256: replacement.PredecessorBindingSHA256, Mode: 0644},
		{Phase: "predecessor-unit", Path: unit, SHA256: previousUnit, Mode: 0644},
		{Phase: "predecessor-retired", Path: filepath.Join(sourceAttemptNumber(replacement.PredecessorAttempt).directory(root), "source-retirement.json"), SHA256: replacement.RetirementSHA256, Mode: 0600},
	} {
		record, e := next(wanted, true)
		if e != nil {
			return nil, e
		}
		preimages[wanted.Path] = record
	}
	write := func(path, sha string, mode uint32, retain bool) error {
		if !sourceInitialSHA(sha) {
			return ErrBinding
		}
		r := sourceRecord{Phase: "file-intent", Path: path, SHA256: sha, Mode: mode}
		if _, err := next(r, false); err != nil {
			return err
		}
		r.Phase = "file-born"
		var err error
		if r, err = next(r, true); err != nil {
			return err
		}
		r.Phase = "file-ready"
		if r, err = next(r, false); err != nil {
			return err
		}
		if retain {
			owned[path] = r
		}
		return nil
	}
	if err := write(filepath.Join(root, "source-replacement-pending"), digest(sourceReplacementPending(hashes[selected])), 0600, false); err != nil {
		return nil, err
	}
	dir := sourceRecord{Phase: "directory-intent", Path: sealed, Mode: 0755}
	if _, err = next(dir, false); err != nil {
		return nil, err
	}
	dir.Phase = "directory-born"
	if dir, err = next(dir, true); err != nil {
		return nil, err
	}
	dir.Phase = "directory-ready"
	if dir, err = next(dir, false); err != nil {
		return nil, err
	}
	names := sourceGenerationNames()
	slices.Sort(names)
	for _, name := range names {
		mode := uint32(0444)
		if name == ProgramName {
			mode = 0555
		}
		path := filepath.Join(sealed, name)
		if err := write(path, hashes[path], mode, true); err != nil {
			return nil, err
		}
	}
	dir.Mode, dir.SHA256 = 0555, b.Generation
	for _, phase := range []string{"seal-intent", "sealed"} {
		dir.Phase = phase
		if dir, err = next(dir, false); err != nil {
			return nil, err
		}
	}
	owned[sealed] = dir
	for _, item := range []struct{ path, first, last string }{
		{selected, "selection-intent", "selection-written"}, {unit, "unit-intent", "unit-written"},
	} {
		r := preimages[item.path]
		r.Phase, r.SHA256 = item.first, hashes[item.path]
		if !sourceInitialSHA(r.SHA256) {
			return nil, ErrBinding
		}
		if r, err = next(r, false); err != nil {
			return nil, err
		}
		r.Phase = item.last
		if r, err = next(r, false); err != nil {
			return nil, err
		}
		owned[item.path] = r
	}
	if _, err = next(sourceRecord{Phase: "reload-intent", Path: unit, SHA256: hashes[unit], Mode: 0644}, false); err != nil {
		return nil, err
	}
	if _, err = next(sourceRecord{Phase: "replacement-verified", Path: selected, SHA256: hashes[selected], Mode: 0644}, false); err != nil || position != len(records) {
		return nil, ErrBinding
	}
	// The original retirement file remains pinned and must match this record;
	// its complete transcript/domain frame is independently consumed as well.
	for path, record := range preimages {
		if path != selected && path != unit {
			owned[path] = record
		}
	}
	return owned, nil
}

func sourceReplacementJournal(root, generation string) string {
	return root + ".source-replacement-" + generation + "-journal"
}

func sourceReplacementPending(binding string) []byte {
	return []byte("ardents-source-replacement-pending-v1\n" + binding + "\n")
}
