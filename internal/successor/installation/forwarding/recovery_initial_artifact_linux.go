package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
)

// The earlier generation suffix retains one concrete custody per unfinished
// artifact. Ready predecessors are observed, never acquired as new writers.
type initialArtifactContinuation struct {
	name          string
	birthSequence int
	retained      bool
	initialFileContinuation
}

type initialGenerationContinuation struct {
	records   uint32
	artifacts []*initialArtifactContinuation
}

type initialArtifactProof struct {
	birth    initialRecord
	file     *initialFile
	retained bool
}

func initialGenerationCut(records uint32) bool { return records >= 72 && records <= 104 }

func initialGenerationNames() []string {
	names := forwardingGenerationNames()
	slices.Sort(names)
	return names
}

func validInitialArtifactBirth(birth *initialRecord, path string, sequence int) bool {
	return birth != nil && birth.Sequence == sequence && birth.Phase == "file-born" && birth.Path == path && birth.Device != 0 && birth.Inode != 0 && birth.SHA256 == "" && birth.Mode == 0600 && birth.UID == 0 && birth.GID == 0
}

// Pure completion rows preserve every original record. Actual file custody and
// fresh effect admission remain with the writer and independent native reader.
func initialGenerationCompletion(original *recoveryOriginal, candidate *recoveryCandidate, proofs []initialArtifactProof) ([]initialRecord, error) {
	count := original.cut.Boundary.Records
	if !initialGenerationCut(count) {
		return nil, ErrBinding
	}
	owned, err := decodeInitialPreReloadRecords(original.prefix.journal.body, candidate.selection.binding, candidate.hashes)
	if err != nil {
		return nil, err
	}
	names := initialGenerationNames()
	complete := int(count-72) / 3
	if len(proofs) != len(names)-complete {
		return nil, ErrBinding
	}
	b := candidate.selection.binding
	rows := []initialRecord{}
	for i := complete; i < len(names); i++ {
		name, proof := names[i], proofs[i-complete]
		path := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation, name)
		sequence := 73 + 3*i
		retained := int(count) == sequence+1
		if proof.retained != retained || !validInitialArtifactBirth(&proof.birth, path, sequence) {
			return nil, ErrBinding
		}
		mode := uint32(0444)
		if name == programName {
			mode = 0555
		}
		if retained {
			if owned[path] != proof.birth {
				return nil, ErrBinding
			}
		} else {
			if _, exists := owned[path]; exists {
				return nil, ErrBinding
			}
			if int(count) <= sequence-1 {
				rows = append(rows, initialRecord{Phase: "file-intent", Path: path, SHA256: candidate.hashes[path], Mode: mode})
			}
			rows = append(rows, proof.birth)
		}
		rows = append(rows, initialRecord{Phase: "file-ready", Path: path, Device: proof.birth.Device, Inode: proof.birth.Inode, SHA256: candidate.hashes[path], Mode: mode})
	}
	return rows, nil
}

// Reconstruct the finite control solely from concrete current file proofs and
// their original-or-continuation birth. No caller-supplied row count or names.
func initialGenerationControl(completion recoveryCompletion, selected generationSelection, proofs []initialArtifactProof) ([]initialRecord, error) {
	if completion.Operation != "initial" || !initialGenerationCut(completion.PrefixRecords) {
		return nil, ErrBinding
	}
	names := initialGenerationNames()
	complete := int(completion.PrefixRecords-72) / 3
	if len(proofs) != len(names)-complete {
		return nil, ErrBinding
	}
	rows := []initialRecord{}
	for i := complete; i < len(names); i++ {
		name, proof := names[i], proofs[i-complete]
		path := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation, name)
		sequence := 73 + 3*i
		retained := int(completion.PrefixRecords) == sequence+1
		file, birth := proof.file, proof.birth
		mode := os.FileMode(0444)
		if name == programName {
			mode = 0555
		}
		if file == nil || file.path != path || proof.retained != retained || !validInitialArtifactBirth(&birth, path, sequence) || !nodeAccess(file.info, mode, 0, 0, false) || len(file.body) == 0 || len(file.body) > 64<<20 || file.info.Size() != int64(len(file.body)) {
			return nil, ErrBinding
		}
		native, ok := infoSys(file.info)
		if !ok || birth.Device != uint64(native.Dev) || birth.Inode != native.Ino {
			return nil, ErrBinding
		}
		if !retained {
			index := sequence - int(completion.PrefixRecords)
			var committed initialRecord
			if index < 0 || len(completion.Records) <= index || json.Unmarshal(completion.Records[index], &committed) != nil || committed != birth {
				return nil, ErrBinding
			}
			rows = append(rows, initialRecord{Phase: "continued-artifact-intent", Path: path, SHA256: digest(file.body), Mode: uint32(mode)})
		}
		phase := "continued-artifact-born"
		if retained {
			phase = "continued-artifact-retained"
		}
		index := sequence + 1 - int(completion.PrefixRecords)
		var ready initialRecord
		if index < 0 || len(completion.Records) <= index || json.Unmarshal(completion.Records[index], &ready) != nil || ready != (initialRecord{Sequence: sequence + 1, Phase: "file-ready", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(file.body), Mode: uint32(mode)}) {
			return nil, ErrBinding
		}
		rows = append(rows, initialRecord{Phase: phase, Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, initialRecord{Phase: "continued-artifact-written", Path: path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(file.body), Mode: uint32(mode)})
	}
	return rows, nil
}

func (g *initialGenerationContinuation) check(candidate *recoveryCandidate, files *completedFiles) error {
	if g == nil || !initialGenerationCut(g.records) || candidate == nil || files == nil || len(candidate.files) != 12 {
		return ErrBinding
	}
	b := candidate.selection.binding
	directory := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	names := initialGenerationNames()
	complete := int(g.records-72) / 3
	if len(g.artifacts) != len(names)-complete {
		return ErrBinding
	}
	present := map[string]bool{}
	for i, name := range names {
		path := filepath.Join(directory, name)
		file := files.files[path]
		mode, wanted := os.FileMode(0444), candidate.files[name]
		if name == programName {
			mode = 0555
		}
		if i >= complete {
			u := g.artifacts[i-complete]
			if u == nil || u.name != name || u.birthSequence != 73+3*i || u.retained != (int(g.records) == u.birthSequence+1) || u.original != file {
				return ErrBinding
			}
			if file == nil {
				if u.retained || u.mutation != nil || u.ready || u.birth != (initialRecord{}) {
					return ErrBinding
				}
				if err := files.absent(path); err != nil {
					return err
				}
				continue
			}
			if !validInitialArtifactBirth(&u.birth, path, u.birthSequence) || !u.retained && u.mutation == nil || u.mutation != nil && !os.SameFile(u.mutation.Identity(), file.info) {
				return ErrBinding
			}
			if !u.ready {
				mode, wanted = 0600, nil
			} else if u.mutation == nil {
				return ErrBinding
			}
		}
		if file == nil || !nodeAccess(file.info, mode, 0, 0, false) || !bytes.Equal(file.body, wanted) {
			return ErrBinding
		}
		if err := observeInitialFile(file); err != nil {
			return err
		}
		present[name] = true
	}
	dir := files.dirs[directory]
	if dir == nil {
		return ErrBinding
	}
	if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := dir.file.ReadDir(len(present) + 1)
	if err != nil && err != io.EOF || len(entries) != len(present) {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if !present[entry.Name()] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
	}
	return files.links()
}

func (s *recoveryInitialReloadAttempt) openGenerationContinuations() error {
	g := s.generation
	if g == nil || s.files.generation != g || g.records != s.original.cut.Boundary.Records || len(s.proof.artifacts) != len(g.artifacts) {
		return ErrBinding
	}
	for _, u := range g.artifacts {
		if !u.retained {
			continue
		}
		if u.original == nil || u.mutation != nil || u.ready {
			return ErrBinding
		}
		parent := s.files.files.dirs[filepath.Dir(u.original.path)]
		if parent == nil {
			return ErrBinding
		}
		if err := s.observe(); err != nil {
			return err
		}
		mode := os.FileMode(0444)
		if u.name == programName {
			mode = 0555
		}
		var err error
		u.mutation, err = fixedfile.ContinueBirth(s.ctx, u.original.path, parent.info, u.original.info, s.candidate.files[u.name], mode, 0)
		if err != nil {
			return err
		}
	}
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) continueGeneration() error {
	g := s.generation
	if g == nil || s.publication.births == nil || s.files.generation != g || len(s.proof.artifacts) != len(g.artifacts) {
		return ErrBinding
	}
	b := s.candidate.selection.binding
	directory := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	parent := s.files.files.dirs[directory]
	if parent == nil {
		return ErrBinding
	}
	for i, u := range g.artifacts {
		path := filepath.Join(directory, u.name)
		wanted := s.candidate.files[u.name]
		mode := os.FileMode(0444)
		if u.name == programName {
			mode = 0555
		}
		if err := s.observe(); err != nil {
			return err
		}
		if u.retained {
			if u.original == nil || u.mutation == nil || u.ready {
				return ErrBinding
			}
			if err := s.publication.births.record("continued-artifact-retained", path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
				return err
			}
		} else {
			if u.original != nil || u.mutation != nil || u.ready || u.birth != (initialRecord{}) {
				return ErrBinding
			}
			if err := s.publication.births.record("continued-artifact-intent", path, nil, digest(wanted), mode, 0, 0); err != nil {
				return err
			}
			if err := s.observe(); err != nil {
				return err
			}
			var err error
			u.mutation, err = fixedfile.Create(s.ctx, path, parent.info, wanted, mode, 0)
			if err != nil {
				return err
			}
			native, ok := infoSys(u.mutation.Identity())
			if !ok {
				return ErrBinding
			}
			u.birth = initialRecord{Sequence: u.birthSequence, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
			if err := s.publication.births.record("continued-artifact-born", path, u.mutation.Identity(), digest(nil), 0600, 0, 0); err != nil {
				return err
			}
			if err := readInitialEmptyBirth(s.files.files, path, u.birth); err != nil {
				return err
			}
			u.original = s.files.files.files[path]
			s.proof.artifacts[i] = initialArtifactProof{birth: u.birth, file: u.original}
		}
		if err := s.observe(); err != nil {
			return err
		}
		if err := u.commit(wanted, mode); err != nil {
			return err
		}
		if err := s.observe(); err != nil {
			return err
		}
		if err := s.publication.births.record("continued-artifact-written", path, u.original.info, digest(wanted), mode, 0, 0); err != nil {
			return err
		}
	}
	return s.observe()
}

func (g *initialGenerationContinuation) close() error {
	if g == nil {
		return nil
	}
	var result error
	for _, u := range g.artifacts {
		result = errors.Join(result, u.close())
	}
	return result
}

// Timestamp is the final generation artifact. This concrete continuation owns
// only its original empty birth or this attempt's new exclusive birth.
func validInitialTimestampBirth(birth *initialRecord, path string) bool {
	return birth != nil && birth.Sequence == 106 && birth.Phase == "file-born" && birth.Path == path && birth.Device != 0 && birth.Inode != 0 && birth.SHA256 == "" && birth.Mode == 0600 && birth.UID == 0 && birth.GID == 0
}

func (s *recoveryInitialFiles) checkGeneration(candidate *recoveryCandidate) error {
	if s.generation != nil {
		return s.generation.check(candidate, s.files)
	}
	b := candidate.selection.binding
	directory := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	u := s.artifact
	if u == nil {
		return verifyInitialInventory(directory, candidate.files)
	}
	path := filepath.Join(directory, "timestamp.json")
	if len(candidate.files) != 12 || u.original != s.files.files[path] {
		return ErrBinding
	}
	if u.original == nil {
		if u.mutation != nil || u.ready {
			return ErrBinding
		}
		if err := s.files.absent(path); err != nil {
			return err
		}
	} else {
		if !validInitialTimestampBirth(&u.birth, path) || u.mutation != nil && !os.SameFile(u.mutation.Identity(), u.original.info) {
			return ErrBinding
		}
		mode, wanted := os.FileMode(0600), []byte(nil)
		if u.ready {
			if u.mutation == nil {
				return ErrBinding
			}
			mode, wanted = 0444, candidate.files["timestamp.json"]
		}
		if !nodeAccess(u.original.info, mode, 0, 0, false) || !bytes.Equal(u.original.body, wanted) {
			return ErrBinding
		}
		if err := observeInitialFile(u.original); err != nil {
			return err
		}
	}
	names := map[string]bool{}
	for name, wanted := range candidate.files {
		if name == "timestamp.json" {
			if u.original != nil {
				names[name] = true
			}
			continue
		}
		file := s.files.files[filepath.Join(directory, name)]
		mode := os.FileMode(0444)
		if name == programName {
			mode = 0555
		}
		if file == nil || !nodeAccess(file.info, mode, 0, 0, false) || !bytes.Equal(file.body, wanted) {
			return ErrBinding
		}
		if err := observeInitialFile(file); err != nil {
			return err
		}
		names[name] = true
	}
	dir := s.files.dirs[directory]
	if dir == nil {
		return ErrBinding
	}
	if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := dir.file.ReadDir(len(names) + 1)
	if err != nil && err != io.EOF || len(entries) != len(names) {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if !names[entry.Name()] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
	}
	return s.files.links()
}

func (s *recoveryInitialReloadAttempt) openTimestampContinuation() error {
	u := s.artifact
	path := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "generations", s.candidate.selection.binding.Generation, "timestamp.json")
	if u == nil || u.original == nil || u.mutation != nil || s.original.cut.Boundary.Records != 107 || s.proof.artifactBirth != &u.birth || s.proof.artifactFile != u.original || !validInitialTimestampBirth(&u.birth, path) {
		return ErrBinding
	}
	parent := s.files.files.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	var err error
	u.mutation, err = fixedfile.ContinueBirth(s.ctx, path, parent.info, u.original.info, s.candidate.files["timestamp.json"], 0444, 0)
	if err != nil {
		return err
	}
	if !sameInputFile(u.mutation.Identity(), u.original.info) {
		return ErrBinding
	}
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) createTimestamp() error {
	u := s.artifact
	if u == nil || u.original != nil || u.mutation != nil || s.original.cut.Boundary.Records < 105 || s.original.cut.Boundary.Records > 106 || s.publication.births == nil || s.proof.artifactBirth != nil || s.proof.artifactCreation != nil || s.proof.artifactFile != nil {
		return ErrBinding
	}
	path := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "generations", s.candidate.selection.binding.Generation, "timestamp.json")
	parent := s.files.files.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	wanted := s.candidate.files["timestamp.json"]
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-artifact-intent", path, nil, digest(wanted), 0444, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	var err error
	u.mutation, err = fixedfile.Create(s.ctx, path, parent.info, wanted, 0444, 0)
	if err != nil {
		return err
	}
	native, ok := infoSys(u.mutation.Identity())
	if !ok {
		return ErrBinding
	}
	u.birth = initialRecord{Sequence: 106, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
	s.proof.artifactCreation = &u.birth
	if err := s.publication.births.record("continued-artifact-born", path, u.mutation.Identity(), digest(nil), 0600, 0, 0); err != nil {
		return err
	}
	if err := readInitialEmptyBirth(s.files.files, path, u.birth); err != nil {
		return err
	}
	u.original = s.files.files.files[path]
	s.proof.artifactFile = u.original
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) continueTimestamp() error {
	u := s.artifact
	if s.original.cut.Boundary.Records <= 106 {
		if err := s.createTimestamp(); err != nil {
			return err
		}
	}
	if u == nil || u.original == nil || u.mutation == nil || u.ready || s.proof.artifactFile != u.original || s.publication.births == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if s.original.cut.Boundary.Records == 107 {
		if s.proof.artifactBirth != &u.birth {
			return ErrBinding
		}
		if err := s.publication.births.record("continued-artifact-retained", u.original.path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
			return err
		}
	} else if s.proof.artifactCreation != &u.birth {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := u.commit(s.candidate.files["timestamp.json"], 0444); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-artifact-written", u.original.path, u.original.info, digest(u.original.body), 0444, 0, 0); err != nil {
		return err
	}
	return s.observe()
}
