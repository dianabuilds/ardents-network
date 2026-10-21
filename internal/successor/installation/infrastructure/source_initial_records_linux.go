package infrastructure

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
)

// This is the complete initial writer's closed transcript, not a phase search,
// creation proof or start permit. Account database records describe historical
// helper effects; the host owns those databases beyond the initial operation.
// A later start must independently admit the current bound principal too.
func decodeSourceInitialRecords(raw []byte, b sourceBinding, hashes map[string]string, admitted string) (map[string]sourceRecord, error) {
	if len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' || !sourceInitialSHA(admitted) {
		return nil, ErrBinding
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 112 {
		return nil, ErrBinding
	}
	records := make([]sourceRecord, len(lines))
	for i, line := range lines {
		if len(line)+1 > 32<<10 || json.Unmarshal(line, &records[i]) != nil {
			return nil, ErrBinding
		}
		canonical, err := json.Marshal(records[i])
		if err != nil || !bytes.Equal(line, canonical) || records[i].Sequence != i {
			return nil, ErrBinding
		}
	}
	position := 0
	// Physical identities are supplied only by a canonical born record, must
	// survive each matching ready/seal record, and are checked against retained
	// native resources by the independent reader. Detached numbers grant none.
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
	journal := b.Request.InstallationRoot + ".source-initial-journal"
	first, err := next(sourceRecord{Phase: "initial", Path: journal, Mode: 0600}, true)
	if err != nil {
		return nil, err
	}
	owned := map[string]sourceRecord{journal: first}
	if _, err = next(sourceRecord{Phase: "admitted", Path: b.Request.InstallationRoot, SHA256: admitted, UID: b.UID, GID: b.GID}, false); err != nil {
		return nil, err
	}
	protected := func(phase, path string, optional bool) (sourceRecord, error) {
		if position >= len(records) {
			return sourceRecord{}, ErrBinding
		}
		r := records[position]
		if optional && r.Device == 0 && r.Inode == 0 {
			return next(sourceRecord{Phase: phase, Path: path}, false)
		}
		if r.UID != 0 || r.Mode > 0777 || r.Mode&0022 != 0 || !sourceInitialSHA(r.SHA256) {
			return sourceRecord{}, ErrBinding
		}
		return next(sourceRecord{Phase: phase, Path: path, SHA256: r.SHA256, Mode: r.Mode, GID: r.GID}, true)
	}
	databases := []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}
	preimages := map[string]sourceRecord{}
	for _, path := range databases {
		if preimages[path], err = protected("account-preimage", path, false); err != nil {
			return nil, err
		}
	}
	auxiliary := map[string]sourceRecord{}
	for _, path := range sourceAccountAuxiliaryPaths() {
		before, e := protected("account-auxiliary-preimage", path, true)
		if e != nil || before.Inode != 0 && path != "/etc/.pwd.lock" && !strings.HasSuffix(path, "-") {
			return nil, ErrBinding
		}
		auxiliary[path] = before
	}
	if _, err = next(sourceRecord{Phase: "account-intent", Path: "ardents-source", UID: b.UID, GID: b.GID}, false); err != nil {
		return nil, err
	}
	for _, path := range databases {
		if _, err = protected("account-born", path, false); err != nil {
			return nil, err
		}
	}
	for _, path := range sourceAccountAuxiliaryPaths() {
		if path == "/etc/.pwd.lock" || strings.HasSuffix(path, "-") {
			after, e := protected("account-auxiliary-result", path, false)
			if e != nil {
				return nil, e
			}
			if path == "/etc/.pwd.lock" {
				if after.Mode != 0600 || after.GID != 0 || after.SHA256 != digest(nil) {
					return nil, ErrBinding
				}
				if before := auxiliary[path]; before.Inode != 0 && (before.Device != after.Device || before.Inode != after.Inode) {
					return nil, ErrBinding
				}
			} else if after.SHA256 != preimages[strings.TrimSuffix(path, "-")].SHA256 {
				return nil, ErrBinding
			}
		} else if _, err = next(sourceRecord{Phase: "account-auxiliary-cleared", Path: path}, false); err != nil {
			return nil, err
		}
	}
	if _, err = next(sourceRecord{Phase: "account-ready", Path: "ardents-source", UID: b.UID, GID: b.GID}, false); err != nil {
		return nil, err
	}
	base := filepath.Join(b.Request.InstallationRoot, "generations")
	sealed := filepath.Join(base, b.Generation)
	for _, d := range []sourceRecord{
		{Path: b.Request.InstallationRoot, Mode: 0755}, {Path: base, Mode: 0755}, {Path: sealed, Mode: 0755},
		{Path: b.Request.StateRoot, Mode: 0700, UID: b.UID, GID: b.GID},
		{Path: b.Request.ParticipationRoot, Mode: 0700, UID: b.UID, GID: b.GID},
		{Path: b.Request.CredentialDirectory, Mode: 0755},
	} {
		d.Phase = "directory-intent"
		if _, err = next(d, false); err != nil {
			return nil, err
		}
		d.Phase = "directory-born"
		if d, err = next(d, true); err != nil {
			return nil, err
		}
		d.Phase = "directory-ready"
		if d, err = next(d, false); err != nil {
			return nil, err
		}
		owned[d.Path] = d
	}
	write := func(path string, mode, uid, gid uint32) error {
		sha := hashes[path]
		if !sourceInitialSHA(sha) {
			return ErrBinding
		}
		r := sourceRecord{Phase: "file-intent", Path: path, SHA256: sha, Mode: mode, UID: uid, GID: gid}
		if _, e := next(r, false); e != nil {
			return e
		}
		r.Phase = "file-born"
		var e error
		if r, e = next(r, true); e != nil {
			return e
		}
		r.Phase = "file-ready"
		if r, e = next(r, false); e != nil {
			return e
		}
		owned[path] = r
		return nil
	}
	for _, name := range []string{"certificate.pem", "private-key.pem", "client-ca.pem"} {
		mode, uid, gid := uint32(0444), uint32(0), uint32(0)
		if name == "private-key.pem" {
			mode, uid, gid = 0600, b.UID, b.GID
		}
		if err = write(filepath.Join(b.Request.CredentialDirectory, name), mode, uid, gid); err != nil {
			return nil, err
		}
	}
	names := sourceGenerationNames()
	slices.Sort(names)
	for _, name := range names {
		mode := uint32(0444)
		if name == ProgramName {
			mode = 0555
		}
		if err = write(filepath.Join(sealed, name), mode, 0, 0); err != nil {
			return nil, err
		}
	}
	directory := owned[sealed]
	directory.Mode, directory.SHA256 = 0555, b.Generation
	for _, phase := range []string{"seal-intent", "sealed"} {
		directory.Phase = phase
		if directory, err = next(directory, false); err != nil {
			return nil, err
		}
	}
	owned[sealed] = directory
	binding := filepath.Join(b.Request.InstallationRoot, "binding.json")
	if err = write(binding, 0644, 0, 0); err != nil {
		return nil, err
	}
	if _, err = next(sourceRecord{Phase: "selected", Path: binding, SHA256: hashes[binding], Mode: 0644}, false); err != nil {
		return nil, err
	}
	unit := "/etc/systemd/system/ardents-source.service"
	if err = write(unit, 0644, 0, 0); err != nil {
		return nil, err
	}
	if _, err = next(sourceRecord{Phase: "reload-intent", Path: unit, SHA256: hashes[unit], Mode: 0644}, false); err != nil {
		return nil, err
	}
	if _, err = next(sourceRecord{Phase: "stopped-verified", Path: binding, SHA256: hashes[binding], Mode: 0644}, false); err != nil || position != len(records) {
		return nil, ErrBinding
	}
	return owned, nil
}

func sourceInitialSHA(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}
