package forwarding

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
)

func replacementJournal(root, generation string) string {
	return root + ".forwarding-replacement-" + generation + "-journal"
}

func replacementPending(root string) string {
	return filepath.Join(root, "forwarding-replacement-pending")
}

func replacementPendingBytes(selected []byte) []byte {
	return []byte("ardents-forwarding-replacement-pending-v1\n" + digest(selected) + "\n")
}

// The entire closed transcript is required. Returned inode observations are
// still untrusted until independently matched; they grant no genuine retirement,
// current native context, Release, recovery continuation or Start permission.
func decodeReplacementContextRecords(raw []byte, selected, previous generationSelection, hashes map[string]string, before, after []byte) (map[string]initialRecord, error) {
	return decodeReplacementTranscript(raw, selected, previous, hashes, before, after, false)
}

// Prefix inventory is not completed replacement or stopped-native authority.
func decodeReplacementPreReloadRecords(raw []byte, selected, previous generationSelection, hashes map[string]string, before []byte) (map[string]initialRecord, error) {
	return decodeReplacementTranscript(raw, selected, previous, hashes, before, nil, true)
}

func decodeReplacementTranscript(raw []byte, selected, previous generationSelection, hashes map[string]string, before, after []byte, preReload bool) (map[string]initialRecord, error) {
	r, b := selected.replacement, selected.binding
	if r == nil || selected.matchPredecessor(previous) != nil || len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' || len(hashes) != 12 {
		return nil, ErrBinding
	}
	for name, wanted := range map[string]string{
		programName: b.ProgramSHA256, "ardents-forwarding.service": b.TemplateSHA256,
		"infrastructure-forwarding.json": b.Generation, "infrastructure-enrollment.json": b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "forwarding-request.json": b.RequestSHA256,
		"forwarding-plan.json": b.PlanSHA256, "rendered-forwarding.service": b.UnitSHA256, "binding.json": digest(selected.raw),
	} {
		if hashes[name] != wanted {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if preReload && len(lines) != 57 && len(lines) != 58 || !preReload && len(lines) != 56 && len(lines) != 62 {
		return nil, ErrBinding
	}
	records := make([]initialRecord, len(lines))
	for n, line := range lines {
		if len(line)+1 > 32<<10 || json.Unmarshal(line, &records[n]) != nil {
			return nil, ErrBinding
		}
		canonical, err := json.Marshal(records[n])
		if err != nil || !bytes.Equal(canonical, line) || records[n].Sequence != n {
			return nil, ErrBinding
		}
	}
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
	retain := func(wanted initialRecord) error {
		actual, err := next(wanted, true)
		if err == nil {
			owned[actual.Path] = actual
		}
		return err
	}
	root := b.Request.InstallationRoot
	journal, pending := replacementJournal(root, b.Generation), replacementPending(root)
	selectedPath, unit := filepath.Join(root, "binding.json"), "/etc/systemd/system/ardents-forwarding.service"
	phase := "forwarding-replacement-v1"
	if preReload || len(records) == 62 {
		phase = "forwarding-replacement-v2"
		if !preReload && compareReloadContexts(before, after) != nil {
			return nil, ErrBinding
		}
		if preReload {
			if _, err := decodeNativeContext(before); err != nil || after != nil {
				return nil, ErrBinding
			}
		}
	} else if before != nil || after != nil {
		return nil, ErrBinding
	}
	if err := retain(initialRecord{Phase: phase, Path: journal, Mode: 0600}); err != nil {
		return nil, err
	}
	if err := retain(initialRecord{Phase: "replacement-admitted", Path: b.History, SHA256: digest(selected.raw), Mode: 0700}); err != nil {
		return nil, err
	}
	context := records[position]
	if !initialSHA(context.SHA256) {
		return nil, ErrBinding
	}
	if phase == "forwarding-replacement-v2" && context.SHA256 != digest(before) {
		return nil, ErrBinding
	}
	if _, err := next(initialRecord{Phase: "native-context", Path: root, SHA256: context.SHA256}, false); err != nil {
		return nil, err
	}
	for _, record := range []initialRecord{
		{Phase: "predecessor-binding", Path: selectedPath, SHA256: digest(previous.raw), Mode: 0644},
		{Phase: "predecessor-unit", Path: unit, SHA256: previous.binding.UnitSHA256, Mode: 0644},
		{Phase: "predecessor-retired", Path: filepath.Join(attemptNumber(r.PredecessorAttempt).retirement(root), "retirement-result.json"), SHA256: r.RetirementSHA256, Mode: 0600},
	} {
		if err := retain(record); err != nil {
			return nil, err
		}
	}
	write := func(path, sha string, mode uint32) error {
		if !initialSHA(sha) {
			return ErrBinding
		}
		if _, err := next(initialRecord{Phase: "file-intent", Path: path, SHA256: sha, Mode: mode}, false); err != nil {
			return err
		}
		born, err := next(initialRecord{Phase: "file-born", Path: path, Mode: 0600}, true)
		if err != nil {
			return err
		}
		born.Phase, born.SHA256, born.Mode = "file-ready", sha, mode
		ready, err := next(born, false)
		if err == nil {
			owned[path] = ready
		}
		return err
	}
	if err := write(pending, digest(replacementPendingBytes(selected.raw)), 0600); err != nil {
		return nil, err
	}
	beforePath, afterPath := replacementNativePaths(journal)
	if phase == "forwarding-replacement-v2" {
		if err := write(beforePath, digest(before), 0600); err != nil {
			return nil, err
		}
	}
	sealed := filepath.Join(root, "generations", b.Generation)
	if _, err := next(initialRecord{Phase: "directory-intent", Path: sealed, Mode: 0755}, false); err != nil {
		return nil, err
	}
	directory, err := next(initialRecord{Phase: "directory-born", Path: sealed, Mode: 0700}, true)
	if err != nil {
		return nil, err
	}
	directory.Phase, directory.Mode = "directory-ready", 0755
	if _, err := next(directory, false); err != nil {
		return nil, err
	}
	names := forwardingGenerationNames()
	slices.Sort(names)
	for _, name := range names {
		mode := uint32(0444)
		if name == programName {
			mode = 0555
		}
		if err := write(filepath.Join(sealed, name), hashes[name], mode); err != nil {
			return nil, err
		}
	}
	directory.Mode, directory.SHA256 = 0555, b.Generation
	for _, phase := range []string{"seal-intent", "sealed"} {
		directory.Phase = phase
		if _, err := next(directory, false); err != nil {
			return nil, err
		}
	}
	owned[sealed] = directory
	for _, change := range []struct{ path, intent, written, sha string }{
		{selectedPath, "selection-intent", "selection-written", digest(selected.raw)},
		{unit, "unit-intent", "unit-written", b.UnitSHA256},
	} {
		old := owned[change.path]
		old.Phase, old.SHA256 = change.intent, change.sha
		if _, err := next(old, false); err != nil {
			return nil, err
		}
		old.Phase = change.written
		if actual, err := next(old, false); err != nil {
			return nil, err
		} else {
			owned[change.path] = actual
		}
	}
	if preReload && position == len(records) {
		return replacementOwnedInventory(owned, 20, position, len(records))
	}
	if _, err := next(initialRecord{Phase: "reload-intent", Path: unit, SHA256: b.UnitSHA256, Mode: 0644}, false); err != nil {
		return nil, err
	}
	if preReload {
		return replacementOwnedInventory(owned, 20, position, len(records))
	}
	if phase == "forwarding-replacement-v2" {
		if err := write(afterPath, digest(after), 0600); err != nil {
			return nil, err
		}
	}
	if _, err := next(initialRecord{Phase: "replacement-verified", Path: selectedPath, SHA256: digest(selected.raw), Mode: 0644}, false); err != nil {
		return nil, err
	}
	wantedOwned := 19
	if phase == "forwarding-replacement-v2" {
		wantedOwned = 21
	}
	return replacementOwnedInventory(owned, wantedOwned, position, len(records))
}

func replacementOwnedInventory(owned map[string]initialRecord, wanted, position, count int) (map[string]initialRecord, error) {
	if position != count || len(owned) != wanted {
		return nil, ErrBinding
	}
	identities := map[[2]uint64]bool{}
	for _, record := range owned {
		identity := [2]uint64{record.Device, record.Inode}
		if identities[identity] {
			return nil, ErrBinding
		}
		identities[identity] = true
	}
	return owned, nil
}
