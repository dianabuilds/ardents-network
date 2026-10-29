package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// A recovered reader retains independently opened cut and completion files in
// the same custody as its immutable original journal. Reconstructed records are
// a separate logical transcript; the original file body is never substituted.
type recoveredInitial struct {
	completion *initialFile
	logical    []byte
	nativeRaw  []byte
	paths      []string
	reload     *recoveryReloadProof
	// Only the actual publisher's closed writer can identify present guards.
	originalPending, recoveryPending *initialFile
}

func (c *completedInitial) initialHistorySHA() string {
	if r := c.files.recovered; r != nil {
		return digest(r.completion.body)
	}
	return digest(c.files.journal.body)
}

func (c *completedInitial) readRecoveredInitial(known *recoveryPublication) (raw []byte, returned error) {
	f, selected := c.files, c.initial
	journal := f.journal.path
	control, completion, pending := recoveryPaths(journal)
	cutControl, cutResult, cutPending := transitionCutPaths(journal)
	nativeBefore, nativeAfter := recoveryReloadPaths(journal)
	any := false
	for _, path := range []string{control, completion, pending, cutControl, cutResult, cutPending, journal + ".recovery-native-context", nativeBefore, nativeAfter} {
		if _, err := os.Lstat(path); err == nil {
			any = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if !any {
		if known != nil {
			return nil, ErrBinding
		}
		return f.journal.body, nil
	}
	r := &recoveredInitial{}
	f.recovered = r // owns partial native resources even when opening fails
	read := func(path string, maximum int64, exclusive bool) (*initialFile, error) {
		file, err := f.retain(path, maximum, 0600, 0, 0)
		if err != nil {
			return nil, err
		}
		if exclusive {
			if err := syscall.Flock(int(file.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				return nil, err
			}
		}
		_, err = f.fill(file, maximum)
		return file, err
	}
	cc, err := read(cutControl, 64<<10, true)
	if err != nil {
		return nil, err
	}
	cr, err := read(cutResult, 256<<10, false)
	if err != nil {
		return nil, err
	}
	cut, err := decodeTransitionCut(cc.body, cr.body, f.journal.body, cc.info, cr.info, f.journal.info)
	if err != nil || cut.Boundary.Operation != "initial" || !bytes.Equal(cut.Boundary.Binding, selected.raw) {
		return nil, errors.Join(ErrBinding, err)
	}
	preReload := initialPreReloadCut(cut.Boundary.Records)
	if preReload {
		candidate := &recoveryCandidate{selection: selected, hashes: c.initialHashes(filepath.Join(selected.binding.Request.InstallationRoot, "binding.json"))}
		if _, err := initialPreReloadInventory(f.journal.body, cut, candidate); err != nil {
			return nil, err
		}
	} else {
		if _, err := initialStoppedPrefix(f.journal.body, cut, selected); err != nil {
			return nil, err
		}
	}
	rc, err := read(control, 64<<10, true)
	if err != nil {
		return nil, err
	}
	r.completion, err = read(completion, 4<<20, false)
	if err != nil {
		return nil, err
	}
	envelope, logical, err := decodeRecoveryCompletion(r.completion.body, f.journal.body, cr.body, cut)
	if err != nil {
		return nil, err
	}
	if preReload {
		if envelope.Schema != "ardents-forwarding-recovery-completion-v2" {
			return nil, ErrBinding
		}
		r.reload = &recoveryReloadProof{}
		if cut.Boundary.Records <= 112 {
			path := filepath.Join(selected.binding.Request.InstallationRoot, "binding.json")
			owned, err := decodeInitialPreReloadRecords(f.journal.body, selected.binding, c.initialHashes(path))
			if err != nil {
				return nil, err
			}
			if initialSealCut(cut.Boundary.Records) {
				dirPath := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation)
				dir := owned[dirPath]
				if !validInitialSealDirectory(&dir, dirPath) {
					return nil, ErrBinding
				}
				r.reload.generationDirectory = &dir
			}
			birth := owned[path]
			if initialSelectionCreationCut(cut.Boundary.Records) {
				index := int(111 - cut.Boundary.Records)
				if len(envelope.Records) <= index || json.Unmarshal(envelope.Records[index], &birth) != nil {
					return nil, ErrBinding
				}
			}
			if !validInitialSelectionBirth(&birth, path) || f.files[path] == nil {
				return nil, ErrBinding
			}
			r.reload.selectionFile = f.files[path]
			if initialSelectionCreationCut(cut.Boundary.Records) {
				r.reload.selectionCreation = &birth
			} else {
				r.reload.selectionBirth = &birth
			}
		}
		if initialGenerationCut(cut.Boundary.Records) {
			owned, err := decodeInitialPreReloadRecords(f.journal.body, selected.binding, c.initialHashes(filepath.Join(selected.binding.Request.InstallationRoot, "binding.json")))
			if err != nil {
				return nil, err
			}
			complete := int(cut.Boundary.Records-72) / 3
			for i, name := range initialGenerationNames() {
				if i < complete {
					continue
				}
				path := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation, name)
				sequence := 73 + 3*i
				retained := int(cut.Boundary.Records) == sequence+1
				birth := owned[path]
				if !retained {
					index := sequence - int(cut.Boundary.Records)
					if index < 0 || len(envelope.Records) <= index || json.Unmarshal(envelope.Records[index], &birth) != nil {
						return nil, ErrBinding
					}
				}
				if !validInitialArtifactBirth(&birth, path, sequence) || f.files[path] == nil {
					return nil, ErrBinding
				}
				r.reload.artifacts = append(r.reload.artifacts, initialArtifactProof{birth: birth, file: f.files[path], retained: retained})
			}
		}
		if initialArtifactCut(cut.Boundary.Records) {
			path := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation, "timestamp.json")
			var birth initialRecord
			if cut.Boundary.Records == 107 {
				owned, err := decodeInitialPreReloadRecords(f.journal.body, selected.binding, c.initialHashes(filepath.Join(selected.binding.Request.InstallationRoot, "binding.json")))
				if err != nil {
					return nil, err
				}
				birth = owned[path]
			} else {
				index := int(106 - cut.Boundary.Records)
				if len(envelope.Records) <= index || json.Unmarshal(envelope.Records[index], &birth) != nil {
					return nil, ErrBinding
				}
			}
			if !validInitialTimestampBirth(&birth, path) || f.files[path] == nil {
				return nil, ErrBinding
			}
			r.reload.artifactFile = f.files[path]
			if cut.Boundary.Records == 107 {
				r.reload.artifactBirth = &birth
			} else {
				r.reload.artifactCreation = &birth
			}
		}
		if cut.Boundary.Records <= 115 {
			index := int(115 - cut.Boundary.Records)
			var birth initialRecord
			if len(envelope.Records) <= index || json.Unmarshal(envelope.Records[index], &birth) != nil || !validInitialUnitCreation(&birth) {
				return nil, ErrBinding
			}
			r.reload.unitCreation = &birth
		}
		if cut.Boundary.Records == 116 {
			owned, err := decodeInitialPreReloadRecords(f.journal.body, selected.binding, c.initialHashes(filepath.Join(selected.binding.Request.InstallationRoot, "binding.json")))
			if err != nil {
				return nil, err
			}
			birth := owned["/etc/systemd/system/ardents-forwarding.service"]
			r.reload.unitBirth = &birth
		}
		r.reload.before, err = read(nativeBefore, 64<<10, false)
		if err != nil {
			return nil, err
		}
		r.reload.after, err = read(nativeAfter, 64<<10, false)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(r.reload.before.body, cut.Boundary.NativeContext) || digest(r.reload.after.body) != envelope.NativeContextSHA256 {
			return nil, ErrBinding
		}
	} else if envelope.Schema != "ardents-forwarding-recovery-completion-v1" {
		return nil, ErrBinding
	}
	sealed := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation)
	unit := f.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil || f.dirs[sealed] == nil {
		return nil, ErrBinding
	}
	for _, path := range []string{cutPending, journal + ".recovery-native-context"} {
		if err := f.absent(path); err != nil {
			return nil, err
		}
		f.absentPaths = append(f.absentPaths, path)
	}
	if known == nil {
		if err := f.absent(pending); err != nil {
			return nil, err
		}
		f.absentPaths = append(f.absentPaths, pending)
	} else {
		if known.births == nil || !known.births.closed || known.births.terminal != nil || known.pending == nil || known.completion.Journal != journal || !bytes.Equal(known.raw, r.completion.body) {
			return nil, ErrBinding
		}
		guard, err := read(pending, 128, false)
		if err != nil || !sameInputFile(guard.info, known.pending.info) || !bytes.Equal(guard.body, recoveryPendingBytes(envelope.OriginalCutSHA256)) {
			return nil, errors.Join(ErrBinding, err)
		}
		originalGuard, err := read(f.pending, 128, false)
		if err != nil || known.originalPending == nil || !sameInputFile(originalGuard.info, known.originalPending.info) || !bytes.Equal(originalGuard.body, []byte("ardents-forwarding-initial-pending-v1\n"+digest(selected.raw)+"\n")) {
			return nil, errors.Join(ErrBinding, err)
		}
		r.originalPending, r.recoveryPending = originalGuard, guard
	}
	var pendingInfo os.FileInfo
	if r.recoveryPending != nil {
		pendingInfo = r.recoveryPending.info
	}
	if err := decodeRecoveryControlProof(rc.body, r.completion.body, envelope, selected, rc.info, r.completion.info, f.journal.info, f.dirs[sealed].info, unit.info, pendingInfo, nil, r.reload); err != nil {
		return nil, err
	}
	identities := map[[2]uint64]bool{}
	for _, file := range f.fileOrder {
		native, ok := infoSys(file.info)
		if !ok || identities[[2]uint64{uint64(native.Dev), native.Ino}] {
			return nil, ErrBinding
		}
		identities[[2]uint64{uint64(native.Dev), native.Ino}] = true
	}
	r.nativeRaw = bytes.Clone(cut.Boundary.NativeContext)
	r.logical = logical
	r.paths = []string{cutControl, cutResult, control, completion}
	if r.reload != nil {
		r.nativeRaw = bytes.Clone(r.reload.after.body)
		r.paths = append(r.paths, nativeBefore, nativeAfter)
	}
	if r.recoveryPending != nil {
		r.paths = append(r.paths, pending)
	}
	// Fresh current context is opened only after the complete ordered later
	// replacement lineage has independently validated every own reload proof.
	return logical, nil
}

func (r *recoveredInitial) check() error {
	if r == nil || r.completion == nil || r.logical == nil {
		return ErrBinding
	}
	return nil
}

func checkRecoveryManager(native *recoveryContext) error {
	if native == nil {
		return nil
	}
	if err := native.check(); err != nil {
		return err
	}
	manager, paths, err := systemd.ObserveForwardingManagerDeclaration(native.ctx)
	if err != nil || manager != native.expected.Manager || !slices.Equal(paths, native.expected.UnitPaths) {
		return errors.Join(ErrBinding, err)
	}
	return native.check()
}

func (r *recoveredInitial) close() error {
	if r == nil {
		return nil
	}
	clear(r.logical)
	clear(r.nativeRaw)
	return nil
}
