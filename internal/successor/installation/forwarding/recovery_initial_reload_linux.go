package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type recoveryReloadProof struct {
	before, after                   *initialFile
	unitBirth                       *initialRecord
	unitCreation                    *initialRecord
	selectionBirth                  *initialRecord
	selectionFile                   *initialFile
	selectionCreation               *initialRecord
	generationDirectory             *initialRecord
	artifactBirth, artifactCreation *initialRecord
	artifactFile                    *initialFile
}

func initialPreReloadCut(records uint32) bool {
	return records >= 105 && records <= 118
}

func initialUnitCreationCut(records uint32) bool {
	return records >= 105 && records <= 115
}

func initialSelectionCreationCut(records uint32) bool {
	return records >= 105 && records <= 111
}

func initialSealCut(records uint32) bool {
	return records >= 105 && records <= 109
}

func initialArtifactCut(records uint32) bool {
	return records >= 105 && records <= 107
}

func recoveryReloadPaths(journal string) (string, string) {
	return journal + ".recovery-native-before", journal + ".recovery-native-after"
}

// Replacement exclusively births its canonical after leaf; the separate
// control proves the birth belongs to recovery, not to the original cut.
func recoveryReloadProofPaths(journal, operation string) (string, string, error) {
	before, after := recoveryReloadPaths(journal)
	switch operation {
	case "initial":
		return before, after, nil
	case "replacement":
		_, after = replacementNativePaths(journal)
		return before, after, nil
	}
	return "", "", ErrBinding
}

type recoveryInitialReloadAttempt struct {
	ctx         context.Context
	v           *release.Verifier
	a           infrastructure.ForwardingAuthorization
	input       Inputs
	original    *recoveryOriginal
	candidate   *recoveryCandidate
	publication *recoveryPublication
	files       *recoveryInitialFiles
	native      *recoveryContext
	manager     *systemd.ForwardingStopped
	proof       recoveryReloadProof
	after       []byte
	terminal    error
	unit        *initialFileContinuation
	selection   *initialFileContinuation
	seal        *initialSealContinuation
	artifact    *initialFileContinuation
}

// The original coordinator was frozen before its next effect, then denied and
// joined. This byte check never substitutes for that actual cut or fresh native
// observations; it returns only independently matchable original inventory.
func initialPreReloadInventory(prefix []byte, cut transitionCutResult, candidate *recoveryCandidate) (map[string]initialRecord, error) {
	if candidate == nil {
		return nil, ErrInput
	}
	selected := candidate.selection
	rows, err := decodeTransitionPrefix(prefix, "initial")
	if err != nil || !initialPreReloadCut(uint32(len(rows))) || cut.Boundary.Records != uint32(len(rows)) || cut.Boundary.PrefixSHA256 != digest(prefix) || cut.Boundary.Journal != selected.binding.Request.InstallationRoot+".forwarding-initial-journal" || cut.Boundary.Operation != "initial" || !bytes.Equal(cut.Boundary.Binding, selected.raw) || cut.Origin.ProgramSHA256 != selected.binding.ProgramSHA256 || !bytes.Equal(cut.Boundary.OriginalContext, cut.Boundary.NativeContext) || rows[0].Phase != "forwarding-initial-v2" || rows[1].Phase != "native-context" || rows[1].SHA256 != digest(cut.Boundary.OriginalContext) {
		return nil, errors.Join(ErrRepairRequired, err)
	}
	native, err := decodeNativeContext(cut.Boundary.NativeContext)
	if err != nil || native.Manager != cut.Boundary.Manager {
		return nil, errors.Join(ErrBinding, err)
	}
	return decodeInitialPreReloadRecords(prefix, selected.binding, candidate.hashes)
}

func initialReloadCompletion(original *recoveryOriginal, candidate *recoveryCandidate, after []byte, creation, selectionCreation, artifactCreation *initialRecord) ([]byte, error) {
	if original == nil || original.prefix == nil {
		return nil, ErrInput
	}
	if _, err := initialPreReloadInventory(original.prefix.journal.body, original.cut, candidate); err != nil {
		return nil, err
	}
	if err := compareReloadContexts(original.cut.Boundary.NativeContext, after); err != nil {
		return nil, err
	}
	b := candidate.selection.binding
	delta := []json.RawMessage{}
	full := bytes.Clone(original.prefix.journal.body)
	rows := []initialRecord{}
	if initialArtifactCut(original.cut.Boundary.Records) {
		path := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation, "timestamp.json")
		var birth initialRecord
		if original.cut.Boundary.Records == 107 {
			inventory, err := decodeInitialPreReloadRecords(original.prefix.journal.body, b, candidate.hashes)
			if err != nil || artifactCreation != nil {
				return nil, errors.Join(ErrBinding, err)
			}
			birth = inventory[path]
		} else {
			if !validInitialTimestampBirth(artifactCreation, path) {
				return nil, ErrBinding
			}
			birth = *artifactCreation
			if original.cut.Boundary.Records == 105 {
				rows = append(rows, initialRecord{Phase: "file-intent", Path: path, SHA256: candidate.hashes[path], Mode: 0444})
			}
			rows = append(rows, birth)
		}
		if !validInitialTimestampBirth(&birth, path) {
			return nil, ErrBinding
		}
		rows = append(rows, initialRecord{Phase: "file-ready", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: candidate.hashes[path], Mode: 0444})
	} else if artifactCreation != nil {
		return nil, ErrBinding
	}
	if initialSealCut(original.cut.Boundary.Records) {
		inventory, err := decodeInitialPreReloadRecords(original.prefix.journal.body, b, candidate.hashes)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
		dir := inventory[path]
		if !validInitialSealDirectory(&dir, path) {
			return nil, ErrBinding
		}
		dir.Mode, dir.SHA256 = 0555, b.Generation
		if original.cut.Boundary.Records <= 108 {
			dir.Phase = "seal-intent"
			rows = append(rows, dir)
		}
		dir.Phase = "sealed"
		rows = append(rows, dir)
	}
	if initialSelectionCreationCut(original.cut.Boundary.Records) {
		path := filepath.Join(b.Request.InstallationRoot, "binding.json")
		if !validInitialSelectionBirth(selectionCreation, path) {
			return nil, ErrBinding
		}
		if original.cut.Boundary.Records <= 110 {
			rows = append(rows, initialRecord{Phase: "file-intent", Path: path, SHA256: digest(candidate.selection.raw), Mode: 0644})
		}
		rows = append(rows, *selectionCreation, initialRecord{Phase: "file-ready", Path: path, Device: selectionCreation.Device, Inode: selectionCreation.Inode, SHA256: digest(candidate.selection.raw), Mode: 0644})
	} else if selectionCreation != nil {
		return nil, ErrBinding
	}
	if original.cut.Boundary.Records <= 115 {
		if !validInitialUnitCreation(creation) {
			return nil, ErrBinding
		}
		if original.cut.Boundary.Records == 112 {
			prefix, err := decodeTransitionPrefix(original.prefix.journal.body, "initial")
			if err != nil {
				return nil, err
			}
			born := prefix[111]
			rows = append(rows, initialRecord{Phase: "file-ready", Path: born.Path, Device: born.Device, Inode: born.Inode, SHA256: digest(candidate.selection.raw), Mode: 0644})
		}
		if original.cut.Boundary.Records <= 113 {
			rows = append(rows, initialRecord{Phase: "selected", Path: filepath.Join(b.Request.InstallationRoot, "binding.json"), SHA256: digest(candidate.selection.raw), Mode: 0644})
		}
		if original.cut.Boundary.Records <= 114 {
			rows = append(rows, initialRecord{Phase: "file-intent", Path: creation.Path, SHA256: b.UnitSHA256, Mode: 0644})
		}
		rows = append(rows, *creation, initialRecord{Phase: "file-ready", Path: creation.Path, Device: creation.Device, Inode: creation.Inode, SHA256: b.UnitSHA256, Mode: 0644})
	} else if creation != nil {
		return nil, ErrBinding
	} else if original.cut.Boundary.Records == 116 {
		prefix, err := decodeTransitionPrefix(original.prefix.journal.body, "initial")
		if err != nil {
			return nil, err
		}
		born := prefix[115]
		rows = append(rows, initialRecord{Phase: "file-ready", Path: born.Path, Device: born.Device, Inode: born.Inode, SHA256: b.UnitSHA256, Mode: 0644})
	}
	if original.cut.Boundary.Records <= 117 {
		rows = append(rows, initialRecord{Phase: "reload-intent", Path: "/etc/systemd/system/ardents-forwarding.service", SHA256: b.UnitSHA256, Mode: 0644})
	}
	rows = append(rows, initialRecord{Phase: "stopped-verified", Path: filepath.Join(b.Request.InstallationRoot, "binding.json"), SHA256: digest(candidate.selection.raw), Mode: 0644}, initialRecord{Phase: "completed", Path: original.cut.Boundary.Journal, SHA256: digest(candidate.selection.raw), Mode: 0600})
	for n, row := range rows {
		row.Sequence = int(original.cut.Boundary.Records) + n
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		delta = append(delta, raw)
		full = append(full, raw...)
		full = append(full, '\n')
	}
	if _, err := decodeInitialRecords(full, b, candidate.hashes); err != nil {
		return nil, err
	}
	cut := original.cut.Boundary
	completion := recoveryCompletion{Schema: "ardents-forwarding-recovery-completion-v2", Operation: "initial", Journal: cut.Journal, Device: cut.Device, Inode: cut.Inode, PrefixSHA256: digest(original.prefix.journal.body), PrefixRecords: cut.Records, Generation: cut.Generation, BindingSHA256: cut.BindingSHA256, OriginalCutSHA256: digest(original.result.body), Outcome: "selected-stopped", Records: delta, TranscriptSHA256: digest(full), NativeContextSHA256: digest(after)}
	raw, err := json.Marshal(completion)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, _, err := decodeRecoveryCompletion(raw, original.prefix.journal.body, original.result.body, original.cut); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *recoveryInitialReloadAttempt) run(history string) error {
	owned, err := initialPreReloadInventory(s.original.prefix.journal.body, s.original.cut, s.candidate)
	if err != nil {
		return err
	}
	s.files, err = readRecoveryInitialInventory(s.original, s.candidate, owned)
	if err != nil {
		return err
	}
	s.artifact = s.files.artifact
	if initialArtifactCut(s.original.cut.Boundary.Records) {
		if s.artifact == nil {
			return ErrBinding
		}
		if s.original.cut.Boundary.Records == 107 {
			s.proof.artifactBirth, s.proof.artifactFile = &s.artifact.birth, s.artifact.original
		}
	}
	if initialSealCut(s.original.cut.Boundary.Records) {
		b := s.candidate.selection.binding
		path := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
		dir, record := s.files.files.dirs[path], owned[path]
		if dir == nil || !validInitialSealDirectory(&record, path) {
			return ErrBinding
		}
		s.seal = &initialSealContinuation{directory: dir, record: record}
		s.proof.generationDirectory = &s.seal.record
	}
	s.native, err = openRecoveryContext(s.ctx, s.original.cut.Boundary.NativeContext)
	if err != nil {
		return err
	}
	if s.original.cut.Boundary.Records <= 112 {
		path := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "binding.json")
		file := s.files.files.files[path]
		birth := owned[path]
		if s.original.cut.Boundary.Records == 112 && (file == nil || !validInitialSelectionBirth(&birth, path)) {
			return ErrBinding
		}
		s.selection = &initialFileContinuation{original: file, birth: birth}
		s.files.selection = s.selection
		if file != nil {
			s.proof.selectionBirth, s.proof.selectionFile = &s.selection.birth, file
		}
	}
	if s.original.cut.Boundary.Records <= 116 {
		unit := s.files.files.files["/etc/systemd/system/ardents-forwarding.service"]
		if s.original.cut.Boundary.Records == 116 && unit == nil {
			return ErrBinding
		}
		birth := owned["/etc/systemd/system/ardents-forwarding.service"]
		s.unit = &initialFileContinuation{original: unit, birth: birth}
		if unit != nil {
			s.proof.unitBirth = &s.unit.birth
		}
		s.manager, err = systemd.OpenForwardingRecoveryUnit(s.ctx, s.native.expected.Manager)
	} else {
		s.manager, err = systemd.OpenForwardingRecoveryInitial(s.ctx, s.native.expected.Manager)
	}
	if err != nil {
		return err
	}
	if err := errors.Join(s.observe(), s.v.CheckHistoryPath(s.ctx, history), recoveryInitialSiblings(s.original)); err != nil {
		return err
	}
	if s.unit != nil && s.original.cut.Boundary.Records == 116 {
		if err := s.openUnitContinuation(); err != nil {
			return err
		}
	}
	if s.artifact != nil && s.original.cut.Boundary.Records == 107 {
		if err := s.openTimestampContinuation(); err != nil {
			return err
		}
	}
	if s.selection != nil && s.original.cut.Boundary.Records == 112 {
		if err := s.openSelectionContinuation(); err != nil {
			return err
		}
	}
	b := s.candidate.selection.binding
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	unit := s.files.files.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil && s.original.cut.Boundary.Records > 115 || s.files.files.dirs[sealed] == nil || s.files.files.dirs[filepath.Dir(b.Request.InstallationRoot)] == nil {
		return ErrBinding
	}
	s.publication.originalPending = s.files.files.files[b.Request.InstallationRoot+".forwarding-initial-pending"]
	s.publication.originalParent = s.files.files.dirs[filepath.Dir(b.Request.InstallationRoot)].info
	if s.publication.originalPending == nil {
		return ErrBinding
	}
	cut := s.original.cut.Boundary
	s.publication.completion = recoveryCompletion{Schema: "ardents-forwarding-recovery-completion-v2", Operation: "initial", Journal: cut.Journal, BindingSHA256: cut.BindingSHA256, OriginalCutSHA256: digest(s.original.result.body)}
	var unitInfo os.FileInfo
	if unit != nil {
		unitInfo = unit.info
	}
	selected, err := s.publication.begin(s.ctx, s.original, s.files.files.dirs[sealed].info, unitInfo, s.observe, nil)
	if err != nil {
		return err
	}
	if s.artifact != nil {
		if err := s.continueTimestamp(); err != nil {
			return err
		}
	}
	if s.seal != nil {
		if err := s.sealGeneration(); err != nil {
			return err
		}
	}
	if s.selection != nil {
		if err := s.continueSelection(); err != nil {
			return err
		}
	}
	if s.unit != nil {
		if err := s.continueUnit(); err != nil {
			return err
		}
		unit = s.unit.original
	}
	beforePath, afterPath := recoveryReloadPaths(cut.Journal)
	if err := s.publication.births.write(beforePath, cut.NativeContext, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	s.proof.before = s.publication.births.files[len(s.publication.births.files)-1]
	if err := errors.Join(s.observe(), s.publication.births.record("recovery-reload-intent", "/etc/systemd/system/ardents-forwarding.service", nil, b.UnitSHA256, 0644, 0, 0)); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	// Close uncertainty blocks before the effect. Old generator observations
	// cannot be refreshed by weakening their checks after a successful Reload.
	if err := s.native.close(); err != nil {
		return err
	}
	s.native = nil
	if err := s.manager.Reload(); err != nil {
		return err
	}
	return s.finishReload(selected, unit, afterPath)
}

func (s *recoveryInitialReloadAttempt) finishReload(selected generationSelection, unit *initialFile, afterPath string) (returned error) {
	before, err := decodeNativeContext(s.original.cut.Boundary.NativeContext)
	if err != nil {
		return err
	}
	namespace, err := openStartNamespace(s.ctx, before.UnitPaths)
	if namespace != nil {
		defer func() { returned = errors.Join(returned, namespace.close()) }()
	}
	if err != nil {
		return err
	}
	after, err := nativeContextAfterReload(s.original.cut.Boundary.NativeContext, namespace)
	if err != nil {
		return errors.Join(err, namespace.close())
	}
	if err := namespace.close(); err != nil {
		return err
	}
	s.native, err = openRecoveryContext(s.ctx, after)
	if err != nil {
		return err
	}
	s.after = after
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.write(afterPath, after, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	s.proof.after = s.publication.births.files[len(s.publication.births.files)-1]
	raw, err := initialReloadCompletion(s.original, s.candidate, after, s.proof.unitCreation, s.proof.selectionCreation, s.proof.artifactCreation)
	if err != nil {
		return err
	}
	completion, _, err := decodeRecoveryCompletion(raw, s.original.prefix.journal.body, s.original.result.body, s.original.cut)
	if err != nil {
		return err
	}
	s.publication.raw, s.publication.completion = raw, completion
	b := selected.binding
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	if err := s.publication.finish(s.original, selected, s.files.files.dirs[sealed].info, unit.info, nil, s.observe); err != nil {
		return err
	}
	if err := errors.Join(s.observe(), s.publication.close(), s.manager.Close(), s.native.close(), s.unit.close(), s.selection.close(), s.artifact.close(), s.files.files.close(), s.original.close()); err != nil {
		return err
	}
	if err := readbackRecoveryStopped(s.ctx, s.v, s.a, s.input, s.publication, filepath.Join(b.Request.InstallationRoot, "binding.json")); err != nil {
		return err
	}
	return errors.Join(s.input.Check(), s.a.Check(s.ctx, s.v))
}

func (s *recoveryInitialReloadAttempt) observe() error {
	check := func() error {
		return errors.Join(s.ctx.Err(), s.input.Check(), s.a.Check(s.ctx, s.v), s.original.check(), s.files.check(s.candidate), s.checkUnitContinuation(), s.native.check(), observeNodePrincipal(s.ctx, s.candidate.selection.binding.UID, s.candidate.selection.binding.GID))
	}
	if err := check(); err != nil {
		return err
	}
	id, err := s.manager.ManagerIdentity()
	if err != nil || id != s.native.expected.Manager {
		return errors.Join(ErrBinding, err)
	}
	for round := 0; round < 2; round++ {
		if err := cgroup.CheckForwardingQuiescent(s.ctx); err != nil {
			return err
		}
		if s.after == nil {
			var paths []string
			var err error
			if s.unit != nil && !s.unit.ready {
				paths, err = s.manager.CheckUnitUnloaded()
			} else {
				paths, err = s.manager.CheckPreReload()
			}
			if err != nil {
				return err
			}
			if !slices.Equal(paths, s.native.expected.UnitPaths) {
				return ErrBinding
			}
		} else {
			unit, service, err := s.manager.ObserveLoaded()
			if err != nil {
				return err
			}
			b := s.candidate.selection.binding
			if err := verifyForwardingStopped(unit, service, b.Request, b.Generation); err != nil {
				return err
			}
		}
		if err := errors.Join(check(), cgroup.CheckForwardingQuiescent(s.ctx)); err != nil {
			return err
		}
	}
	return check()
}

func (s *recoveryInitialReloadAttempt) close() error {
	if s == nil {
		return nil
	}
	s.terminal = errors.Join(s.terminal, s.publication.close(), s.manager.Close(), s.native.close(), s.unit.close(), s.selection.close(), s.artifact.close())
	if s.files != nil {
		s.terminal = errors.Join(s.terminal, s.files.files.close())
	}
	clear(s.after)
	return errors.Join(s.terminal, s.ctx.Err())
}
