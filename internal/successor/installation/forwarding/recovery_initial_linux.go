package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Recovery never reopens a writer for the immutable original journal. This
// observation retains fresh native files while the independently opened original
// exclusive lease remains held by this same Installation operation.
type recoveryInitialFiles struct {
	files      *completedFiles
	owned      map[string]initialRecord
	selection  *initialFileContinuation
	artifact   *initialFileContinuation
	generation *initialGenerationContinuation
}

func stoppedInitialCompletion(original *recoveryOriginal, candidate *recoveryCandidate) ([]byte, []byte, error) {
	if original == nil || candidate == nil || candidate.selection.replacement != nil || original.cut.Boundary.Operation != "initial" || !bytes.Equal(original.cut.Boundary.Binding, candidate.selection.raw) {
		return nil, nil, ErrRepairRequired
	}
	prefix := original.prefix.journal.body
	rows, err := initialStoppedPrefix(prefix, original.cut, candidate.selection)
	if err != nil {
		return nil, nil, errors.Join(ErrRepairRequired, err)
	}
	b := candidate.selection.binding
	delta := []json.RawMessage{}
	full := bytes.Clone(prefix)
	if len(rows) == 119 {
		if rows[118].Phase != "stopped-verified" {
			return nil, nil, ErrRepairRequired
		}
		row := initialRecord{Sequence: 119, Phase: "completed", Path: original.prefix.journal.path, SHA256: digest(candidate.selection.raw), Mode: 0600}
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, nil, err
		}
		delta = append(delta, raw)
		full = append(full, raw...)
		full = append(full, '\n')
	}
	if _, err := decodeInitialRecords(full, b, candidate.hashes); err != nil {
		return nil, nil, err
	}
	boundary := original.cut.Boundary
	completion := recoveryCompletion{
		Schema: "ardents-forwarding-recovery-completion-v1", Operation: "initial", Journal: boundary.Journal,
		Device: boundary.Device, Inode: boundary.Inode, PrefixSHA256: digest(prefix), PrefixRecords: boundary.Records,
		Generation: b.Generation, BindingSHA256: digest(candidate.selection.raw), OriginalCutSHA256: digest(original.result.body),
		Outcome: "selected-stopped", Records: delta, TranscriptSHA256: digest(full),
	}
	raw, err := json.Marshal(completion)
	if err != nil {
		return nil, nil, err
	}
	raw = append(raw, '\n')
	if _, _, err := decodeRecoveryCompletion(raw, prefix, original.result.body, original.cut); err != nil {
		return nil, nil, err
	}
	return raw, full, nil
}

func initialStoppedPrefix(prefix []byte, cut transitionCutResult, selected generationSelection) ([]initialRecord, error) {
	rows, err := decodeTransitionPrefix(prefix, "initial")
	if err != nil || len(rows) != 119 && len(rows) != 120 || rows[0].Phase != "forwarding-initial-v2" || rows[1].Phase != "native-context" || rows[1].SHA256 != digest(cut.Boundary.OriginalContext) || rows[118].Phase != "stopped-verified" || cut.Boundary.Operation != "initial" || !bytes.Equal(cut.Boundary.Binding, selected.raw) || cut.Origin.ProgramSHA256 != selected.binding.ProgramSHA256 {
		return nil, errors.Join(ErrRepairRequired, err)
	}
	for _, raw := range [][]byte{cut.Boundary.OriginalContext, cut.Boundary.NativeContext} {
		native, err := decodeNativeContext(raw)
		if err != nil || native.Manager != cut.Boundary.Manager {
			return nil, errors.Join(ErrBinding, err)
		}
	}
	return rows, nil
}

func readRecoveryInitialFiles(original *recoveryOriginal, candidate *recoveryCandidate, logical []byte) (result *recoveryInitialFiles, returned error) {
	owned, err := decodeInitialRecords(logical, candidate.selection.binding, candidate.hashes)
	if err != nil {
		return nil, err
	}
	return readRecoveryInitialInventory(original, candidate, owned)
}

// Native input custody is shared, not completed-transaction permission. The
// early caller supplies only the independently parsed original prefix inventory.
func readRecoveryInitialInventory(original *recoveryOriginal, candidate *recoveryCandidate, owned map[string]initialRecord) (result *recoveryInitialFiles, returned error) {
	f := &completedFiles{ctx: original.ctx, files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}}
	s := &recoveryInitialFiles{files: f, owned: owned}
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, f.close())
			result = nil
		}
	}()
	for path, record := range owned {
		if record.Phase == "file-born" {
			unit := path == "/etc/systemd/system/ardents-forwarding.service" && original.cut.Boundary.Records == 116
			selection := path == filepath.Join(candidate.selection.binding.Request.InstallationRoot, "binding.json") && original.cut.Boundary.Records == 112
			artifact := path == filepath.Join(candidate.selection.binding.Request.InstallationRoot, "generations", candidate.selection.binding.Generation, "timestamp.json") && original.cut.Boundary.Records == 107
			if initialGenerationCut(original.cut.Boundary.Records) {
				for i, name := range initialGenerationNames() {
					if path == filepath.Join(candidate.selection.binding.Request.InstallationRoot, "generations", candidate.selection.binding.Generation, name) && int(original.cut.Boundary.Records) == 74+3*i && validInitialArtifactBirth(&record, path, 73+3*i) {
						artifact = true
					}
				}
			}
			if !unit && !selection && !artifact {
				return nil, ErrBinding
			}
			if err := readInitialEmptyBirth(f, path, record); err != nil {
				return nil, err
			}
			continue
		}
		if path == original.prefix.journal.path || candidate.hashes[path] != "" {
			maximum := int64(64 << 20)
			if path == original.prefix.journal.path {
				maximum = 4 << 20
			}
			raw, err := f.read(path, maximum, os.FileMode(record.Mode), record.UID, record.GID)
			if err != nil {
				return nil, err
			}
			if path == original.prefix.journal.path {
				if !bytes.Equal(raw, original.prefix.journal.body) || !sameInputFile(f.files[path].info, original.prefix.journal.info) {
					return nil, ErrBinding
				}
			} else if digest(raw) != candidate.hashes[path] {
				return nil, ErrBinding
			}
		} else if err := f.directory(path, os.FileMode(record.Mode), record.UID, record.GID); err != nil {
			return nil, err
		}
	}
	if original.cut.Boundary.Operation == "initial" && initialGenerationCut(original.cut.Boundary.Records) {
		s.generation = &initialGenerationContinuation{records: original.cut.Boundary.Records}
		complete := int(original.cut.Boundary.Records-72) / 3
		for i, name := range initialGenerationNames() {
			if i < complete {
				continue
			}
			path := filepath.Join(candidate.selection.binding.Request.InstallationRoot, "generations", candidate.selection.binding.Generation, name)
			birth, exists := owned[path]
			file := f.files[path]
			retained := int(original.cut.Boundary.Records) == 74+3*i
			if retained {
				if !exists || file == nil || !validInitialArtifactBirth(&birth, path, 73+3*i) {
					return nil, ErrBinding
				}
			} else if exists || file != nil {
				return nil, ErrBinding
			}
			s.generation.artifacts = append(s.generation.artifacts, &initialArtifactContinuation{name: name, birthSequence: 73 + 3*i, retained: retained, initialFileContinuation: initialFileContinuation{original: file, birth: birth}})
		}
	}
	if original.cut.Boundary.Operation == "initial" && initialArtifactCut(original.cut.Boundary.Records) {
		path := filepath.Join(candidate.selection.binding.Request.InstallationRoot, "generations", candidate.selection.binding.Generation, "timestamp.json")
		birth, exists := owned[path]
		file := f.files[path]
		if original.cut.Boundary.Records == 107 {
			if !exists || file == nil || !validInitialTimestampBirth(&birth, path) {
				return nil, ErrBinding
			}
		} else if exists || file != nil {
			return nil, ErrBinding
		}
		s.artifact = &initialFileContinuation{original: file, birth: birth}
	}
	if original.cut.Boundary.Operation == "initial" && initialUnitCreationCut(original.cut.Boundary.Records) {
		path := "/etc/systemd/system/ardents-forwarding.service"
		if _, err := f.parents(path); err != nil {
			return nil, err
		}
		if err := f.absent(path); err != nil {
			return nil, err
		}
	}
	return s, s.check(candidate)
}

func (s *recoveryInitialFiles) check(candidate *recoveryCandidate) error {
	if s == nil || candidate == nil {
		return ErrInput
	}
	f := s.files
	if err := f.links(); err != nil {
		return err
	}
	for path, record := range s.owned {
		var info os.FileInfo
		if file := f.files[path]; file != nil {
			if err := observeInitialFile(file); err != nil {
				return err
			}
			info = file.info
		} else if dir := f.dirs[path]; dir != nil {
			info = dir.info
		}
		native, ok := infoSys(info)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return ErrBinding
		}
	}
	b := candidate.selection.binding
	rootEntries := []string{"binding.json", "generations"}
	selectionPath := filepath.Join(b.Request.InstallationRoot, "binding.json")
	if _, original := s.owned[selectionPath]; !original {
		file := f.files[selectionPath]
		if file == nil {
			if err := f.absent(selectionPath); err != nil {
				return err
			}
			rootEntries = []string{"generations"}
		} else {
			u := s.selection
			if u == nil || u.original != file || u.mutation == nil || !os.SameFile(u.mutation.Identity(), file.info) || !validInitialSelectionBirth(&u.birth, selectionPath) {
				return ErrBinding
			}
			if err := observeInitialFile(file); err != nil {
				return err
			}
		}
	}
	if err := checkInitialEmptyRoots(f, b.Request); err != nil {
		return err
	}
	if err := s.checkGeneration(candidate); err != nil {
		return err
	}
	for path, wanted := range map[string][]string{
		b.Request.InstallationRoot:                               rootEntries,
		filepath.Join(b.Request.InstallationRoot, "generations"): {b.Generation},
		b.Request.CredentialDirectory:                            {"certificate.pem", "private-key.pem"},
	} {
		dir := f.dirs[path]
		if dir == nil {
			return ErrBinding
		}
		if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		entries, err := dir.file.ReadDir(len(wanted) + 1)
		if err != nil && err != io.EOF || len(entries) != len(wanted) {
			return errors.Join(ErrBinding, err)
		}
		for _, entry := range entries {
			if !slices.Contains(wanted, entry.Name()) || entry.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
		}
	}
	return f.links()
}

func recoveryInitialSiblings(original *recoveryOriginal) error {
	root := strings.TrimSuffix(original.prefix.journal.path, ".forwarding-initial-journal")
	control, result, _ := transitionCutPaths(original.prefix.journal.path)
	wanted := map[string]bool{}
	for _, path := range []string{original.prefix.journal.path, root + ".forwarding-initial-pending", control, result} {
		wanted[filepath.Base(path)] = true
	}
	parent := original.prefix.parents.byPath[filepath.Dir(root)]
	if parent == nil {
		return ErrBinding
	}
	if _, err := parent.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := parent.file.ReadDir(4097)
	if err != nil && err != io.EOF || len(entries) > 4096 {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), filepath.Base(root)+".forwarding-") {
			continue
		}
		if !wanted[entry.Name()] || !entry.Type().IsRegular() {
			return ErrRepairRequired
		}
		delete(wanted, entry.Name())
	}
	if len(wanted) != 0 {
		return ErrRepairRequired
	}
	return original.check()
}

// Recover consumes fresh candidate authority and one named genuine original cut.
// It never replays Start, ACK, account creation or domain completion. Prefixes
// outside the independently proved safe classifier retain every blocker.
func Recover(v *release.Verifier, a infrastructure.ForwardingAuthorization, input Inputs, journal, distribution, history string) (returned error) {
	p := input.original
	if p == nil || v == nil || !validPath(journal) || os.Geteuid() != 0 {
		return ErrInput
	}
	if _, finite := p.ctx.Deadline(); !finite {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	p.mu.Lock()
	if p.installing {
		p.mu.Unlock()
		p.uses.Done()
		return ErrInput
	}
	p.installing = true
	p.mu.Unlock()
	using := true
	published := false
	var original *recoveryOriginal
	var candidate *recoveryCandidate
	var native *recoveryContext
	var files *recoveryInitialFiles
	var manager *systemd.ForwardingReplacement
	var publication *recoveryPublication
	var replacement *recoveryReplacementAttempt
	var retirement *recoveryRetirementAttempt
	var start *recoveryStartAttempt
	var initialReload *recoveryInitialReloadAttempt
	defer func() {
		if published {
			candidate.clear()
			return
		}
		returned = errors.Join(returned, publication.close(), manager.Close(), native.close())
		if files != nil {
			returned = errors.Join(returned, files.files.close())
		}
		returned = errors.Join(returned, replacement.close(), retirement.close(), start.close(), initialReload.close(), original.close())
		if using {
			p.uses.Done()
		}
		returned = errors.Join(returned, input.Close(), v.Close(), p.ctx.Err())
		candidate.clear()
		if returned != nil {
			returned = errors.Join(ErrRepairRequired, returned)
		}
	}()
	if err := errors.Join(installedPlatform(p.ctx), input.IndependentPaths(distribution, history), a.Check(p.ctx, v), v.CheckHistoryPath(p.ctx, history)); err != nil {
		return err
	}
	var err error
	original, err = readRecoveryOriginal(p.ctx, journal)
	if err != nil {
		return err
	}
	selected, err := decodeSelection(original.cut.Boundary.Binding)
	operation := original.cut.Boundary.Operation
	matched := operation == "initial" && journal == p.request.InstallationRoot+".forwarding-initial-journal" || operation == "replacement" && journal == replacementJournal(p.request.InstallationRoot, selected.binding.Generation)
	if operation == "retirement" || operation == "start" {
		for n := attemptNumber(0); n < attemptLimit; n++ {
			path := n.retirementJournal(p.request.InstallationRoot)
			if operation == "start" {
				path = n.journal(p.request.InstallationRoot)
			}
			if journal == path {
				matched = true
				break
			}
		}
	}
	if err != nil || selected.binding.History != history || !matched {
		return errors.Join(ErrRepairRequired, err)
	}
	candidate, err = prepareRecoveryCandidate(a, input, selected, original)
	if err != nil {
		return err
	}
	if err := verifyControllerProgram(p.ctx, a.Program()); err != nil {
		return err
	}
	publish := func() error {
		if err := errors.Join(input.Check(), a.Check(p.ctx, v)); err != nil {
			return err
		}
		p.uses.Done()
		using = false
		if err := errors.Join(input.Close(), v.Close(), p.ctx.Err()); err != nil {
			return err
		}
		var originalPublication error
		if publication.completion.Operation == "start" && publication.originalPending == nil {
			originalPublication = publishNeverStartedAbsentPending(publication)
		} else {
			originalPublication = publishRecoveryOriginalPending(publication)
		}
		if err := originalPublication; err != nil {
			return err
		}
		err := publishInitialPending(publication.births, publication.pending, publication.parent)
		published = err == nil
		return err
	}
	if operation == "start" {
		publication = &recoveryPublication{}
		start = &recoveryStartAttempt{ctx: p.ctx, v: v, a: a, input: input, original: original, candidate: candidate, publication: publication}
		if err := start.run(filepath.Join(selected.binding.Request.InstallationRoot, "binding.json"), history); err != nil {
			return err
		}
		if err := errors.Join(start.close(), original.close()); err != nil {
			return err
		}
		return publish()
	}
	if operation == "replacement" {
		publication = &recoveryPublication{}
		replacement = &recoveryReplacementAttempt{ctx: p.ctx, v: v, a: a, input: input, original: original, candidate: candidate, publication: publication}
		if err := replacement.run(filepath.Join(selected.binding.Request.InstallationRoot, "binding.json"), history); err != nil {
			return err
		}
		if err := errors.Join(replacement.close(), original.close()); err != nil {
			return err
		}
		return publish()
	}
	if operation == "retirement" {
		publication = &recoveryPublication{}
		retirement = &recoveryRetirementAttempt{ctx: p.ctx, v: v, a: a, input: input, original: original, candidate: candidate, publication: publication}
		if err := retirement.run(filepath.Join(selected.binding.Request.InstallationRoot, "binding.json"), history); err != nil {
			return err
		}
		if err := errors.Join(retirement.close(), original.close()); err != nil {
			return err
		}
		return publish()
	}
	if operation == "initial" && initialPreReloadCut(original.cut.Boundary.Records) {
		publication = &recoveryPublication{}
		initialReload = &recoveryInitialReloadAttempt{ctx: p.ctx, v: v, a: a, input: input, original: original, candidate: candidate, publication: publication}
		publication.reload = initialReload
		if err := initialReload.run(history); err != nil {
			return err
		}
		if err := errors.Join(initialReload.close(), original.close()); err != nil {
			return err
		}
		return publish()
	}
	completion, logical, err := stoppedInitialCompletion(original, candidate)
	if err != nil {
		return err
	}
	files, err = readRecoveryInitialFiles(original, candidate, logical)
	clear(logical)
	if err != nil {
		return err
	}
	native, err = openRecoveryContext(p.ctx, original.cut.Boundary.NativeContext)
	if err != nil {
		return err
	}
	manager, err = systemd.OpenForwardingRecoveryReplacement(p.ctx, native.expected.Manager)
	if err != nil {
		return err
	}
	admit := func() error {
		if err := errors.Join(input.Check(), a.Check(p.ctx, v), v.CheckHistoryPath(p.ctx, history), original.check(), files.check(candidate), native.check(), observeNodePrincipal(p.ctx, selected.binding.UID, selected.binding.GID)); err != nil {
			return err
		}
		id, err := manager.ManagerIdentity()
		if err != nil || id != native.expected.Manager {
			return errors.Join(ErrBinding, err)
		}
		for round := 0; round < 2; round++ {
			if err := cgroup.CheckForwardingQuiescent(p.ctx); err != nil {
				return err
			}
			facts, err := manager.Observe()
			if err != nil {
				return err
			}
			if !slices.Equal(facts.UnitPaths, native.expected.UnitPaths) {
				return ErrBinding
			}
			if err := verifyForwardingStopped(facts.Unit, facts.Service, selected.binding.Request, selected.binding.Generation); err != nil {
				return err
			}
		}
		return errors.Join(input.Check(), a.Check(p.ctx, v), original.check(), files.check(candidate), native.check(), p.ctx.Err())
	}
	if err := admit(); err != nil {
		return err
	}
	if err := recoveryInitialSiblings(original); err != nil {
		return err
	}
	b := selected.binding
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	unit := files.files.files["/etc/systemd/system/ardents-forwarding.service"]
	publication = &recoveryPublication{originalPending: files.files.files[b.Request.InstallationRoot+".forwarding-initial-pending"], originalParent: files.files.dirs[filepath.Dir(b.Request.InstallationRoot)].info}
	if err := publication.create(p.ctx, original, completion, files.files.dirs[sealed].info, unit.info, admit); err != nil {
		return err
	}
	if err := errors.Join(admit(), publication.close(), manager.Close(), native.close(), files.files.close(), original.close()); err != nil {
		return err
	}
	// Independent completed readback has no writer descriptor or prior lease.
	bound := filepath.Join(b.Request.InstallationRoot, "binding.json")
	if err := readbackRecoveryStopped(p.ctx, v, a, input, publication, bound); err != nil {
		return err
	}
	// All original users, readback, physical writers and authority owners have
	// joined. The recovery guard blocks while the original guard is removed.
	return publish()
}

func publishRecoveryOriginalPending(s *recoveryPublication) error {
	if s == nil || s.births == nil || !s.births.closed || s.births.terminal != nil || s.originalPending == nil || s.originalParent == nil {
		return ErrRepairRequired
	}
	if err := s.births.ctx.Err(); err != nil {
		return err
	}
	guard, err := os.Lstat(s.pending.path)
	if err != nil || !sameInputFile(guard, s.pending.info) {
		return errors.Join(ErrBinding, err)
	}
	parent, err := os.Lstat(filepath.Dir(s.originalPending.path))
	expected, ok := infoSys(s.originalParent)
	if err != nil || !ok || !os.SameFile(parent, s.originalParent) || !nodeAccess(parent, s.originalParent.Mode().Perm(), expected.Uid, expected.Gid, true) {
		return errors.Join(ErrBinding, err)
	}
	named, err := os.Lstat(s.originalPending.path)
	if err != nil || !sameInputFile(named, s.originalPending.info) {
		return errors.Join(ErrBinding, err)
	}
	if err := s.births.ctx.Err(); err != nil {
		return err
	}
	return os.Remove(s.originalPending.path)
}
