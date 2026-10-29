package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Recovery's own physical publication never writes the original journal or
// removes its blocker. The admitting operation owns fresh authority, native
// classification and every required close before final publication.
type recoveryPublication struct {
	births            *initialBirths
	pending           *initialFile
	result            *initialFile
	parent            os.FileInfo
	completion        recoveryCompletion
	raw               []byte
	originalPending   *initialFile
	originalParent    os.FileInfo
	startReadback     *startTerminalSnapshot
	reload            *recoveryInitialReloadAttempt
	replacementReload *recoveryReplacementAttempt
}

func recoveryPendingBytes(cutSHA string) []byte {
	return []byte("ardents-forwarding-recovery-pending-v1\n" + cutSHA + "\n")
}

func recoveryAbsent(journal string) error {
	control, completion, pending := recoveryPaths(journal)
	before, after := recoveryReloadPaths(journal)
	for _, path := range []string{control, completion, pending, journal + ".recovery-native-context", before, after} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrRepairRequired, err)
		}
	}
	return nil
}

func (s *recoveryPublication) create(ctx context.Context, original *recoveryOriginal, raw []byte, candidate, unit os.FileInfo, admit func() error) (returned error) {
	if s == nil || original == nil || original.prefix == nil {
		return ErrInput
	}
	completion, _, err := decodeRecoveryCompletion(raw, original.prefix.journal.body, original.result.body, original.cut)
	if err != nil {
		return err
	}
	s.completion, s.raw = completion, bytes.Clone(raw)
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, s.close())
		}
	}()
	selected, err := s.begin(ctx, original, candidate, unit, admit, nil)
	if err != nil {
		return err
	}
	return s.finish(original, selected, candidate, unit, nil, admit)
}

// The admitting operation supplies only its actual independently retained
// terminal child when a new attempt directory is required. No path/boolean
// waiver can grant an extra birth in the shared publication mechanism.
func (s *recoveryPublication) begin(ctx context.Context, original *recoveryOriginal, candidate, unit os.FileInfo, admit func() error, terminal *startTerminalSnapshot) (generationSelection, error) {
	if s == nil || s.births != nil || ctx == nil || original == nil || original.prefix == nil || admit == nil {
		return generationSelection{}, ErrInput
	}
	selected, err := decodeSelection(original.cut.Boundary.Binding)
	validCandidate := nodeAccess(candidate, 0555, 0, 0, true)
	candidatePhase, candidateMode := "candidate-verified", os.FileMode(0555)
	if reload := s.reload; reload != nil && initialSealCut(original.cut.Boundary.Records) {
		u := reload.seal
		validCandidate = u != nil && u.directory != nil && !u.attempted && !u.sealed && reload.original == original && reload.proof.generationDirectory == &u.record && reload.files != nil && reload.files.files.dirs[u.directory.path] == u.directory && nodeAccess(candidate, 0755, 0, 0, true) && os.SameFile(candidate, u.directory.info)
		if validCandidate {
			_, nativeErr := u.native(0755)
			if nativeErr != nil {
				return selected, nativeErr
			}
		}
		candidatePhase, candidateMode = "candidate-retained", 0755
	}
	validUnit := nodeAccess(unit, 0644, 0, 0, false)
	if reload := s.reload; reload != nil && initialUnitCreationCut(original.cut.Boundary.Records) {
		u := reload.unit
		validUnit = unit == nil && u != nil && u.original == nil && u.mutation == nil && !u.ready && reload.original == original && reload.proof.unitBirth == nil && reload.proof.unitCreation == nil
	}
	if reload := s.reload; reload != nil && original.cut.Boundary.Records == 116 {
		u := reload.unit
		validUnit = u != nil && u.original != nil && u.mutation != nil && !u.ready && reload.original == original && reload.proof.unitBirth == &u.birth && sameInputFile(unit, u.original.info) && sameInputFile(u.mutation.Identity(), unit) && nodeAccess(unit, 0600, 0, 0, false) && unit.Size() == 0
	}
	if err != nil || !validCandidate || !validUnit {
		return selected, errors.Join(ErrBinding, err)
	}
	journal := original.cut.Boundary.Journal
	if s.completion.Journal != journal || s.completion.OriginalCutSHA256 != digest(original.result.body) {
		return selected, ErrBinding
	}
	if err := errors.Join(admit(), original.check(), recoveryAbsent(journal)); err != nil {
		return selected, err
	}
	s.parent = original.prefix.parent()
	control, resultPath, pending := recoveryPaths(journal)
	sealed := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation)
	allowed := map[string]bool{resultPath: true, pending: true, journal: true, sealed: true, "/etc/systemd/system/ardents-forwarding.service": true}
	first := "forwarding-recovery-v1"
	if s.reload != nil && s.replacementReload != nil {
		return selected, ErrBinding
	}
	if reload := s.reload; reload != nil {
		if terminal != nil || reload.original != original || reload.publication != s || reload.candidate == nil || s.completion.Schema != "ardents-forwarding-recovery-completion-v2" || s.completion.Operation != "initial" {
			return selected, ErrBinding
		}
		if _, err := initialPreReloadInventory(original.prefix.journal.body, original.cut, reload.candidate); err != nil {
			return selected, err
		}
		before, after := recoveryReloadPaths(journal)
		allowed[before], allowed[after] = true, true
		first = "forwarding-recovery-reload-v1"
		if reload.unit != nil {
			if !initialUnitCreationCut(original.cut.Boundary.Records) && original.cut.Boundary.Records != 116 {
				return selected, ErrBinding
			}
			first = "forwarding-recovery-unit-v1"
			if original.cut.Boundary.Records <= 115 {
				first = "forwarding-recovery-unit-create-v1"
			}
			if original.cut.Boundary.Records <= 112 {
				u := reload.selection
				if u == nil || u.ready {
					return selected, ErrBinding
				}
				if original.cut.Boundary.Records == 112 {
					if u.original == nil || u.mutation == nil || reload.proof.selectionBirth != &u.birth || reload.proof.selectionFile != u.original || reload.proof.selectionCreation != nil {
						return selected, ErrBinding
					}
				} else if !initialSelectionCreationCut(original.cut.Boundary.Records) || u.original != nil || u.mutation != nil || reload.proof.selectionBirth != nil || reload.proof.selectionFile != nil || reload.proof.selectionCreation != nil {
					return selected, ErrBinding
				}
				allowed[filepath.Join(selected.binding.Request.InstallationRoot, "binding.json")] = true
				first = "forwarding-recovery-selection-v1"
				if initialSelectionCreationCut(original.cut.Boundary.Records) {
					first = "forwarding-recovery-selection-create-v1"
				}
			}
		}
		if reload.seal != nil {
			if !initialSealCut(original.cut.Boundary.Records) {
				return selected, ErrBinding
			}
			first = "forwarding-recovery-seal-v1"
		}
	}
	if reload := s.reload; reload != nil && reload.artifact != nil {
		u := reload.artifact
		if !initialArtifactCut(original.cut.Boundary.Records) || reload.seal == nil || reload.files == nil || reload.files.artifact != u || u.ready {
			return selected, ErrBinding
		}
		path := filepath.Join(sealed, "timestamp.json")
		if original.cut.Boundary.Records == 107 {
			if u.original == nil || u.mutation == nil || reload.proof.artifactBirth != &u.birth || reload.proof.artifactFile != u.original || reload.proof.artifactCreation != nil || !validInitialTimestampBirth(&u.birth, path) {
				return selected, ErrBinding
			}
			first = "forwarding-recovery-artifact-v1"
		} else {
			if u.original != nil || u.mutation != nil || u.birth != (initialRecord{}) || reload.proof.artifactBirth != nil || reload.proof.artifactCreation != nil || reload.proof.artifactFile != nil {
				return selected, ErrBinding
			}
			first = "forwarding-recovery-artifact-create-v1"
		}
		allowed[path] = true
	}
	if reload := s.replacementReload; reload != nil {
		if terminal != nil || reload.original != original || reload.publication != s || reload.beforeReload == nil || reload.beforeReload.original != original || s.completion.Schema != "ardents-forwarding-recovery-completion-v2" || s.completion.Operation != "replacement" {
			return selected, ErrBinding
		}
		if err := reload.beforeReload.check(); err != nil {
			return selected, err
		}
		before, after, err := recoveryReloadProofPaths(journal, "replacement")
		if err != nil {
			return selected, err
		}
		allowed[before], allowed[after] = true, true
		first = "forwarding-recovery-reload-v1"
	}
	if terminal != nil {
		if terminal.original != original || terminal.owner == nil || terminal.owner.files.invocations.head() == nil || terminal.owner.files.invocations.head().terminal != terminal || s.completion.Operation != "start" {
			return selected, ErrBinding
		}
		if _, born := terminal.prefix.owned[terminal.path]; !born {
			allowed[terminal.path] = true
		}
	}
	s.births, err = openForwardingBirths(ctx, control, allowed, first)
	if err != nil {
		return selected, err
	}
	if err := s.births.write(pending, recoveryPendingBytes(s.completion.OriginalCutSHA256), 0600, 0, 0, admit); err != nil {
		return selected, err
	}
	s.pending = s.births.files[len(s.births.files)-1]
	for _, row := range []struct {
		phase, path string
		info        os.FileInfo
		sha         string
		mode        os.FileMode
	}{
		{"original-verified", journal, original.prefix.journal.info, s.completion.OriginalCutSHA256, 0600},
		{candidatePhase, sealed, candidate, s.completion.BindingSHA256, candidateMode},
	} {
		if err := errors.Join(admit(), original.check()); err != nil {
			return selected, err
		}
		if err := s.births.record(row.phase, row.path, row.info, row.sha, row.mode, 0, 0); err != nil {
			return selected, err
		}
	}
	return selected, nil
}

func (s *recoveryPublication) finish(original *recoveryOriginal, selected generationSelection, candidate, unit, terminal os.FileInfo, admit func() error) error {
	if err := errors.Join(admit(), original.check()); err != nil {
		return err
	}
	if err := s.births.record("stopped-verified", "/etc/systemd/system/ardents-forwarding.service", unit, selected.binding.UnitSHA256, 0644, 0, 0); err != nil {
		return err
	}
	_, resultPath, _ := recoveryPaths(s.completion.Journal)
	if err := s.births.write(resultPath, s.raw, 0600, 0, 0, admit); err != nil {
		return err
	}
	s.result = s.births.files[len(s.births.files)-1]
	if err := errors.Join(admit(), original.check()); err != nil {
		return err
	}
	if err := s.births.record("recovery-completed", resultPath, s.result.info, digest(s.raw), 0600, 0, 0); err != nil {
		return err
	}
	var reload *recoveryReloadProof
	if s.reload != nil {
		reload = &s.reload.proof
	}
	if s.replacementReload != nil {
		reload = &s.replacementReload.proof
	}
	if err := decodeRecoveryControlProof(s.births.journal.body, s.raw, s.completion, selected, s.births.journal.info, s.result.info, original.prefix.journal.info, candidate, unit, s.pending.info, terminal, reload); err != nil {
		return err
	}
	return errors.Join(s.births.check(), admit(), original.check())
}

func decodeRecoveryControl(raw, completionRaw []byte, completion recoveryCompletion, selected generationSelection, controlInfo, resultInfo, originalInfo, candidateInfo, unitInfo, pendingInfo os.FileInfo) error {
	return decodeStartRecoveryControl(raw, completionRaw, completion, selected, controlInfo, resultInfo, originalInfo, candidateInfo, unitInfo, pendingInfo, nil)
}

func decodeStartRecoveryControl(raw, completionRaw []byte, completion recoveryCompletion, selected generationSelection, controlInfo, resultInfo, originalInfo, candidateInfo, unitInfo, pendingInfo, terminalInfo os.FileInfo) error {
	return decodeRecoveryControlProof(raw, completionRaw, completion, selected, controlInfo, resultInfo, originalInfo, candidateInfo, unitInfo, pendingInfo, terminalInfo, nil)
}

func decodeRecoveryControlProof(raw, completionRaw []byte, completion recoveryCompletion, selected generationSelection, controlInfo, resultInfo, originalInfo, candidateInfo, unitInfo, pendingInfo, terminalInfo os.FileInfo, reload *recoveryReloadProof) error {
	if len(raw) == 0 || len(raw) > 64<<10 || raw[len(raw)-1] != '\n' || !nodeAccess(controlInfo, 0600, 0, 0, false) || !nodeAccess(resultInfo, 0600, 0, 0, false) || !nodeAccess(originalInfo, 0600, 0, 0, false) || !nodeAccess(candidateInfo, 0555, 0, 0, true) || !nodeAccess(unitInfo, 0644, 0, 0, false) || controlInfo.Size() != int64(len(raw)) || resultInfo.Size() != int64(len(completionRaw)) {
		return ErrBinding
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	count := 11
	if completion.Schema == "ardents-forwarding-recovery-completion-v2" {
		if completion.Operation != "initial" && completion.Operation != "replacement" || terminalInfo != nil || reload == nil || reload.before == nil || reload.after == nil {
			return ErrBinding
		}
		count = 18
		if completion.Operation == "initial" && initialUnitCreationCut(completion.PrefixRecords) {
			if reload.unitBirth != nil || !validInitialUnitCreation(reload.unitCreation) {
				return ErrBinding
			}
			count = 21
		} else if reload.unitCreation != nil {
			return ErrBinding
		} else if completion.Operation == "initial" && completion.PrefixRecords == 116 {
			if reload.unitBirth == nil {
				return ErrBinding
			}
			count = 20
		} else if reload.unitBirth != nil {
			return ErrBinding
		}
		if completion.Operation == "initial" && initialSelectionCreationCut(completion.PrefixRecords) {
			if count != 21 || reload.selectionBirth != nil || reload.selectionCreation == nil || reload.selectionFile == nil {
				return ErrBinding
			}
			count = 24
		} else if completion.Operation == "initial" && completion.PrefixRecords == 112 {
			if count != 21 || reload.selectionBirth == nil || reload.selectionFile == nil {
				return ErrBinding
			}
			count = 23
		} else if reload.selectionBirth != nil || reload.selectionFile != nil {
			return ErrBinding
		}
		if count != 24 && reload.selectionCreation != nil {
			return ErrBinding
		}
		if completion.Operation == "initial" && initialSealCut(completion.PrefixRecords) {
			if count != 24 || reload.generationDirectory == nil {
				return ErrBinding
			}
			count = 26
		} else if reload.generationDirectory != nil {
			return ErrBinding
		}
		if completion.Operation == "initial" && initialArtifactCut(completion.PrefixRecords) {
			if count != 26 || reload.artifactFile == nil {
				return ErrBinding
			}
			if completion.PrefixRecords == 107 {
				if reload.artifactBirth == nil || reload.artifactCreation != nil {
					return ErrBinding
				}
				count = 28
			} else {
				if reload.artifactBirth != nil || reload.artifactCreation == nil {
					return ErrBinding
				}
				count = 29
			}
		} else if reload.artifactBirth != nil || reload.artifactCreation != nil || reload.artifactFile != nil {
			return ErrBinding
		}
	} else if reload != nil {
		return ErrBinding
	} else if completion.Operation == "start" {
		if terminalInfo == nil {
			return ErrBinding
		}
		if len(completion.Records) == 4 {
			count = 14
		} else if len(completion.Records) != 1 {
			return ErrBinding
		}
	} else if terminalInfo != nil {
		return ErrBinding
	}
	if len(lines) != count {
		return ErrBinding
	}
	rows := make([]initialRecord, len(lines))
	for n, line := range lines {
		if json.Unmarshal(line, &rows[n]) != nil {
			return ErrBinding
		}
		canonical, _ := json.Marshal(rows[n])
		if !bytes.Equal(line, canonical) || rows[n].Sequence != n {
			return ErrBinding
		}
	}
	control, result, pending := recoveryPaths(completion.Journal)
	c, _ := infoSys(controlInfo)
	r, _ := infoSys(resultInfo)
	o, _ := infoSys(originalInfo)
	d, _ := infoSys(candidateInfo)
	u, _ := infoSys(unitInfo)
	if digest(selected.raw) != completion.BindingSHA256 || selected.binding.Generation != completion.Generation {
		return ErrBinding
	}
	sealed := filepath.Join(selected.binding.Request.InstallationRoot, "generations", completion.Generation)
	sha, guardSHA := digest(completionRaw), digest(recoveryPendingBytes(completion.OriginalCutSHA256))
	wanted := []initialRecord{
		{Phase: "forwarding-recovery-v1", Path: control, Device: uint64(c.Dev), Inode: c.Ino, Mode: 0600},
		{Phase: "file-intent", Path: pending, SHA256: guardSHA, Mode: 0600},
		{Phase: "file-born", Path: pending, Mode: 0600},
		{Phase: "file-ready", Path: pending, SHA256: guardSHA, Mode: 0600},
		{Phase: "original-verified", Path: completion.Journal, Device: uint64(o.Dev), Inode: o.Ino, SHA256: completion.OriginalCutSHA256, Mode: 0600},
		{Phase: "candidate-verified", Path: sealed, Device: uint64(d.Dev), Inode: d.Ino, SHA256: completion.BindingSHA256, Mode: 0555},
		{Phase: "stopped-verified", Path: "/etc/systemd/system/ardents-forwarding.service", Device: uint64(u.Dev), Inode: u.Ino, SHA256: selected.binding.UnitSHA256, Mode: 0644},
		{Phase: "file-intent", Path: result, SHA256: sha, Mode: 0600},
		{Phase: "file-born", Path: result, Device: uint64(r.Dev), Inode: r.Ino, Mode: 0600},
		{Phase: "file-ready", Path: result, Device: uint64(r.Dev), Inode: r.Ino, SHA256: sha, Mode: 0600},
		{Phase: "recovery-completed", Path: result, Device: uint64(r.Dev), Inode: r.Ino, SHA256: sha, Mode: 0600},
	}
	if count == 14 {
		number, err := startJournalNumber(selected.binding.Request.InstallationRoot, completion.Journal)
		native, ok := infoSys(terminalInfo)
		if err != nil || !ok || !nodeAccess(terminalInfo, 0750, 0, selected.binding.GID, true) {
			return ErrBinding
		}
		path := number.directory(selected.binding.Request.InstallationRoot)
		triple := []initialRecord{{Phase: "directory-intent", Path: path, Mode: 0750, GID: selected.binding.GID}, {Phase: "directory-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0700}, {Phase: "directory-ready", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0750, GID: selected.binding.GID}}
		prefix := append([]initialRecord(nil), wanted[:6]...)
		prefix = append(prefix, triple...)
		wanted = append(prefix, wanted[6:]...)
	}
	if count == 18 || count == 20 || count == 21 || count == 23 || count == 24 || count == 26 || count == 28 || count == 29 {
		before, after, err := recoveryReloadProofPaths(completion.Journal, completion.Operation)
		if err != nil {
			return err
		}
		if reload.before.path != before || reload.after.path != after || digest(reload.after.body) != completion.NativeContextSHA256 || completion.NativeContextSHA256 == "" {
			return ErrBinding
		}
		if err := compareReloadContexts(reload.before.body, reload.after.body); err != nil {
			return err
		}
		triple := func(file *initialFile) ([]initialRecord, error) {
			native, ok := infoSys(file.info)
			if !ok || !nodeAccess(file.info, 0600, 0, 0, false) || file.info.Size() != int64(len(file.body)) || len(file.body) == 0 || len(file.body) > 64<<10 {
				return nil, ErrBinding
			}
			return []initialRecord{{Phase: "file-intent", Path: file.path, SHA256: digest(file.body), Mode: 0600}, {Phase: "file-born", Path: file.path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}, {Phase: "file-ready", Path: file.path, Device: uint64(native.Dev), Inode: native.Ino, SHA256: digest(file.body), Mode: 0600}}, nil
		}
		a, err := triple(reload.before)
		if err != nil {
			return err
		}
		b, err := triple(reload.after)
		if err != nil {
			return err
		}
		wanted[0].Phase = "forwarding-recovery-reload-v1"
		prefix := append([]initialRecord(nil), wanted[:6]...)
		prefix = append(prefix, a...)
		prefix = append(prefix, initialRecord{Phase: "recovery-reload-intent", Path: "/etc/systemd/system/ardents-forwarding.service", SHA256: selected.binding.UnitSHA256, Mode: 0644})
		prefix = append(prefix, b...)
		wanted = append(prefix, wanted[6:]...)
		if count == 20 {
			birth := reload.unitBirth
			native, ok := infoSys(unitInfo)
			if !ok || birth.Sequence != 115 || birth.Phase != "file-born" || birth.Path != "/etc/systemd/system/ardents-forwarding.service" || birth.Mode != 0600 || birth.UID != 0 || birth.GID != 0 || birth.SHA256 != "" || birth.Device != uint64(native.Dev) || birth.Inode != native.Ino {
				return ErrBinding
			}
			wanted[0].Phase = "forwarding-recovery-unit-v1"
			continued := []initialRecord{{Phase: "continued-file-retained", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, {Phase: "continued-file-written", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: selected.binding.UnitSHA256, Mode: 0644}}
			prefix := append([]initialRecord(nil), wanted[:6]...)
			prefix = append(prefix, continued...)
			wanted = append(prefix, wanted[6:]...)
		}
		if count == 21 || count == 23 || count == 24 || count == 26 || count == 28 || count == 29 {
			birth := reload.unitCreation
			if birth.Device != uint64(u.Dev) || birth.Inode != u.Ino {
				return ErrBinding
			}
			// The separate completion must commit this actual new birth,
			// rather than borrowing an invented original prefix record.
			index := int(115 - completion.PrefixRecords)
			var continuedBirth initialRecord
			if len(completion.Records) <= index || json.Unmarshal(completion.Records[index], &continuedBirth) != nil || continuedBirth != *birth {
				return ErrBinding
			}
			wanted[0].Phase = "forwarding-recovery-unit-create-v1"
			continued := []initialRecord{{Phase: "continued-file-intent", Path: birth.Path, SHA256: selected.binding.UnitSHA256, Mode: 0644}, {Phase: "continued-file-born", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, {Phase: "continued-file-written", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: selected.binding.UnitSHA256, Mode: 0644}}
			prefix := append([]initialRecord(nil), wanted[:6]...)
			prefix = append(prefix, continued...)
			wanted = append(prefix, wanted[6:]...)
		}
		if count == 23 || count == 24 || count == 26 || count == 28 || count == 29 {
			file, birth := reload.selectionFile, reload.selectionBirth
			if count == 24 || count == 26 || count == 28 || count == 29 {
				birth = reload.selectionCreation
			}
			path := filepath.Join(selected.binding.Request.InstallationRoot, "binding.json")
			if file.path != path || !validInitialSelectionBirth(birth, path) || !nodeAccess(file.info, 0644, 0, 0, false) || !bytes.Equal(file.body, selected.raw) {
				return ErrBinding
			}
			native, ok := infoSys(file.info)
			if !ok || birth.Device != uint64(native.Dev) || birth.Inode != native.Ino {
				return ErrBinding
			}
			wanted[0].Phase = "forwarding-recovery-selection-v1"
			continued := []initialRecord{{Phase: "continued-file-retained", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, {Phase: "continued-file-written", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(selected.raw), Mode: 0644}}
			if count == 24 || count == 26 || count == 28 || count == 29 {
				index := int(111 - completion.PrefixRecords)
				var committed initialRecord
				if len(completion.Records) <= index || json.Unmarshal(completion.Records[index], &committed) != nil || committed != *birth {
					return ErrBinding
				}
				wanted[0].Phase = "forwarding-recovery-selection-create-v1"
				continued = []initialRecord{{Phase: "continued-file-intent", Path: path, SHA256: digest(selected.raw), Mode: 0644}, {Phase: "continued-file-born", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, {Phase: "continued-file-written", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(selected.raw), Mode: 0644}}
			}
			prefix := append([]initialRecord(nil), wanted[:6]...)
			prefix = append(prefix, continued...)
			wanted = append(prefix, wanted[6:]...)
		}
		if count == 26 || count == 28 || count == 29 {
			dir := reload.generationDirectory
			if !validInitialSealDirectory(dir, sealed) || dir.Device != uint64(d.Dev) || dir.Inode != d.Ino {
				return ErrBinding
			}
			wanted[0].Phase = "forwarding-recovery-seal-v1"
			wanted[5].Phase, wanted[5].Mode = "candidate-retained", 0755
			prefix := append([]initialRecord(nil), wanted[:6]...)
			prefix = append(prefix, initialRecord{Phase: "continued-directory-seal-intent", Path: sealed, Device: dir.Device, Inode: dir.Inode, SHA256: selected.binding.Generation, Mode: 0555}, initialRecord{Phase: "continued-directory-sealed", Path: sealed, Device: dir.Device, Inode: dir.Inode, SHA256: selected.binding.Generation, Mode: 0555})
			wanted = append(prefix, wanted[6:]...)
		}
		if count == 28 || count == 29 {
			file := reload.artifactFile
			birth := reload.artifactBirth
			if count == 29 {
				birth = reload.artifactCreation
			}
			path := filepath.Join(sealed, "timestamp.json")
			native, ok := infoSys(file.info)
			if !ok || file.path != path || !validInitialTimestampBirth(birth, path) || !nodeAccess(file.info, 0444, 0, 0, false) || file.info.Size() != int64(len(file.body)) || len(file.body) == 0 || len(file.body) > 1<<20 || birth.Device != uint64(native.Dev) || birth.Inode != native.Ino {
				return ErrBinding
			}
			index := int(107 - completion.PrefixRecords)
			var ready initialRecord
			if len(completion.Records) <= index || json.Unmarshal(completion.Records[index], &ready) != nil || ready != (initialRecord{Sequence: 107, Phase: "file-ready", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(file.body), Mode: 0444}) {
				return ErrBinding
			}
			wanted[0].Phase = "forwarding-recovery-artifact-v1"
			continued := []initialRecord{{Phase: "continued-artifact-retained", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, {Phase: "continued-artifact-written", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(file.body), Mode: 0444}}
			if count == 29 {
				index := int(106 - completion.PrefixRecords)
				var committed initialRecord
				if len(completion.Records) <= index || json.Unmarshal(completion.Records[index], &committed) != nil || committed != *birth {
					return ErrBinding
				}
				wanted[0].Phase = "forwarding-recovery-artifact-create-v1"
				continued = []initialRecord{{Phase: "continued-artifact-intent", Path: path, SHA256: digest(file.body), Mode: 0444}, {Phase: "continued-artifact-born", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, {Phase: "continued-artifact-written", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(file.body), Mode: 0444}}
			}
			prefix := append([]initialRecord(nil), wanted[:6]...)
			prefix = append(prefix, continued...)
			wanted = append(prefix, wanted[6:]...)
		}
	}
	// Every admitted grammar ends with stopped, result intent/born/ready and
	// recovery-completed, independently of its operation-specific prefix.
	stopped := count - 5
	if rows[2].Device == 0 || rows[2].Inode == 0 || rows[2].Device != rows[3].Device || rows[2].Inode != rows[3].Inode || !validPath(sealed) || !validStartDigest(rows[stopped].SHA256) {
		return ErrBinding
	}
	if pendingInfo != nil {
		native, ok := infoSys(pendingInfo)
		if !ok || !nodeAccess(pendingInfo, 0600, 0, 0, false) || uint64(native.Dev) != rows[2].Device || native.Ino != rows[2].Inode || pendingInfo.Size() != int64(len(recoveryPendingBytes(completion.OriginalCutSHA256))) {
			return ErrBinding
		}
	}
	wanted[2].Device, wanted[2].Inode = rows[2].Device, rows[2].Inode
	wanted[3].Device, wanted[3].Inode = rows[3].Device, rows[3].Inode
	for n := range rows {
		wanted[n].Sequence = n
		if rows[n] != wanted[n] {
			return ErrBinding
		}
	}
	return nil
}

func (s *recoveryPublication) close() error {
	if s == nil {
		return nil
	}
	return s.births.close()
}
