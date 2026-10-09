package installation

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// This persisted identity is compatibility provenance, not Release authority.
// Recording a replacement does not authorize truncation or runtime admission.
type fixedReplacementRecord struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Path             string `json:"path"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
	PreviousDigest   string `json:"previous_digest"`
	CandidateDigest  string `json:"candidate_digest"`
}

// The original fresh-pair owner reaches this only after staging and real
// quiescence. Keep every preimage unchanged while establishing repair provenance.
func (owned *successorPreparation) recordFixedReplacements() (returnedErr error) {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.joined {
		return ErrBinding
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			owned.stage.retainFailure(owned.ctx, returnedErr)
		}
	}()
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
		return err
	}
	stage := owned.stage
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		if len(stage.journal.Bytes(journal.Transitions, "0002.json")) == 0 {
			return ErrBinding
		}
		body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: "replacing-fixed-resources"})
		if err != nil {
			return err
		}
		if err := stage.journal.Write(owned.ctx, journal.Transitions, "0003.json", body); err != nil {
			return err
		}
	}
	resources, err := successorFixedResources(stage)
	if err != nil {
		return err
	}
	preimages, err := recoveryFixedBytes(owned.previous)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(resources))
	for filename := range resources {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	for _, filename := range paths {
		if err := owned.observe(); err != nil {
			return err
		}
		if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
			return err
		}
		previous := fixedFileObservation{parent: owned.inspection.directories[filepath.Dir(filename)], file: owned.inspection.files[filename]}
		if err := stage.recordFixedReplacement(owned.ctx, filename, previous, preimages[filename], resources[filename]); err != nil {
			return err
		}
	}
	return errors.Join(owned.observe(), owned.predecessor.observeQuiescent(owned.ctx))
}

func successorFixedResources(stage *installationTransaction) (map[string][]byte, error) {
	if stage == nil || stage.generation == nil && stage.sealed == nil {
		return nil, ErrInput
	}
	files := make(map[string][]byte)
	for _, name := range fixedResourceNames() {
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		files[name] = stage.generationBytes(name)
	}
	return fixedResourceImages(files)
}

// Selection has a different access and phase boundary from root-only resource
// copies. Neither a supplied mode/GID nor a replacement record chooses it.
func (stage *installationTransaction) validateReplacementAccess(filename string, previous fixedFileObservation, candidate []byte) error {
	if stage == nil || stage.lease == nil || stage.journal == nil {
		return ErrInput
	}
	name, phase := "0003.json", "replacing-fixed-resources"
	if filename == filepath.Join(stage.lease.path, "selection.json") {
		native, ok := stage.lease.identity.Sys().(*syscall.Stat_t)
		if !ok || native.Gid == 0 || native.Gid != previous.file.gid || previous.file.mode != 0640 ||
			stage.lease.identity.Mode() != os.ModeDir|0750 || !sameStagingDirectory(stage.lease.identity, previous.parent) {
			return ErrBinding
		}
		var intent successorTransitionIntent
		if err := decodeCanonical(stage.intent.body, 128<<10, &intent); err != nil || intent.Schema != "ardents-endpoint-installation-successor-v1" || intent.Candidate != stage.selected {
			return errors.Join(ErrBinding, err)
		}
		oldBytes, oldErr := canonicalJSON(intent.Previous)
		newBytes, newErr := canonicalJSON(stage.selected)
		if oldErr != nil || newErr != nil || !fixedfile.ReplacementPrefixAllowed(previous.file.body, oldBytes, newBytes) || !bytes.Equal(candidate, newBytes) {
			return errors.Join(ErrBinding, oldErr, newErr)
		}
		if !bytes.Equal(previous.file.body, oldBytes) && len(stage.journal.Bytes(journal.Replacements, digestHex([]byte(filename))+".json")) == 0 {
			return ErrBinding
		}
		if err := stage.verifyTransitionPhase("0004.json", "fixed-resources-replaced"); err != nil {
			return err
		}
		name, phase = "0005.json", "publishing-selection"
	} else if (previous.file.mode != 0644 && previous.file.mode != 0555) || previous.file.gid != 0 {
		return ErrBinding
	}
	return stage.verifyTransitionPhase(name, phase)
}

// Native record mechanism; its product caller supplies the closed inventory
// from the same leased inspection. It never changes a fixed resource.
func (stage *installationTransaction) recordFixedReplacement(ctx context.Context, filename string, previous fixedFileObservation, preimage, candidate []byte) (returnedErr error) {
	if ctx == nil || stage == nil || stage.lease == nil || stage.journal == nil || previous.file.identity == nil || previous.parent == nil ||
		!canonicalPath(filename) || filename == "/" || len(preimage) == 0 || len(preimage) > 64<<20 || len(candidate) == 0 || len(candidate) > 64<<20 {
		return ErrInput
	}
	var intent successorTransitionIntent
	if err := decodeCanonical(stage.intent.body, 128<<10, &intent); err != nil || intent.Schema != "ardents-endpoint-installation-successor-v1" || intent.Candidate != stage.selected {
		return errors.Join(ErrBinding, err)
	}
	if err := stage.validateReplacementAccess(filename, previous, candidate); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := observeFixedFile(filename, previous); err != nil {
		return err
	}
	if !fixedfile.ReplacementPrefixAllowed(previous.file.body, preimage, candidate) {
		return ErrBinding
	}
	native, ok := previous.file.identity.Sys().(*syscall.Stat_t)
	if !ok || native.Dev == 0 || native.Ino == 0 || native.Uid != 0 || native.Gid != previous.file.gid || native.Nlink != 1 {
		return ErrBinding
	}
	record := fixedReplacementRecord{Schema: "ardents-endpoint-installation-replacement-v1", GenerationDigest: stage.selected.GenerationDigest, Path: filename,
		Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(previous.file.mode), GID: previous.file.gid,
		PreviousDigest: digestHex(preimage), CandidateDigest: digestHex(candidate)}
	body, err := canonicalJSON(record)
	if err != nil {
		return err
	}
	if err := stage.journal.Ensure(ctx, journal.Replacements); err != nil {
		return err
	}
	name := digestHex([]byte(filename)) + ".json"
	if existing := stage.journal.Bytes(journal.Replacements, name); existing != nil {
		if !bytes.Equal(existing, body) {
			return ErrBinding
		}
	} else if err := stage.journal.Write(ctx, journal.Replacements, name, body); err != nil {
		return err
	}
	return errors.Join(stage.syncReplacementRecord(ctx, name), observeFixedFile(filename, previous), ctx.Err())
}

func (stage *installationTransaction) syncReplacementRecord(ctx context.Context, name string) error {
	if ctx == nil || stage == nil || stage.journal == nil || !stage.journal.HasCollection(journal.Replacements) {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := stage.journal.Resync(ctx, journal.Replacements, name); err != nil {
		return err
	}
	return errors.Join(stage.observe(), ctx.Err())
}

// This operation retains the same fresh-pair owner, original lease and joined
// predecessor. Filesystem provenance never substitutes for those admissions.
func (owned *successorPreparation) replaceFixedResources() (returnedErr error) {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.joined {
		return ErrBinding
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			if owned.stage.terminal == nil {
				owned.stage.retainFailure(owned.ctx, returnedErr)
			}
		}
	}()
	if err := owned.recordFixedReplacements(); err != nil {
		return err
	}
	resources, err := successorFixedResources(owned.stage)
	if err != nil {
		return err
	}
	preimages, err := recoveryFixedBytes(owned.previous)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(resources))
	for filename := range resources {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	for _, filename := range paths {
		if err := owned.observe(); err != nil {
			return err
		}
		if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
			return err
		}
		previous := fixedFileObservation{parent: owned.inspection.directories[filepath.Dir(filename)], file: owned.inspection.files[filename]}
		written, err := owned.stage.replaceRecordedFixedFile(owned.ctx, filename, previous, preimages[filename], resources[filename])
		if written.identity != nil {
			owned.inspection.files[filename] = written
			owned.stage.fixed[filename] = fixedFileObservation{parent: previous.parent, file: written}
		}
		if err != nil {
			return err
		}
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
		return err
	}
	if err := owned.stage.recordFixedReplacementCompletion(owned.ctx); err != nil {
		return err
	}
	return owned.publishSuccessorSelection()
}

// The transaction calls this only after all exact candidate files are synced
// and actual quiescence is reobserved. Reopened completion is original physical
// provenance: resync it rather than replacing it or treating visibility as ACK.
func (stage *installationTransaction) recordFixedReplacementCompletion(ctx context.Context) error {
	if ctx == nil || stage == nil || stage.journal == nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.verifyTransitionPhase("0003.json", "replacing-fixed-resources"); err != nil {
		return err
	}
	if len(stage.journal.Bytes(journal.Transitions, "0004.json")) != 0 {
		if err := stage.verifyTransitionPhase("0004.json", "fixed-resources-replaced"); err != nil {
			return err
		}
		return stage.journal.Resync(ctx, journal.Transitions, "0004.json")
	}
	body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: "fixed-resources-replaced"})
	if err != nil {
		return err
	}
	return stage.journal.Write(ctx, journal.Transitions, "0004.json", body)
}

// Repair only authorized torn prefixes on the original inode. This lower
// mechanism supplies no fresh proof, manager admission or recovery consumer.
func (stage *installationTransaction) replaceRecordedFixedFile(ctx context.Context, filename string, previous fixedFileObservation, preimage, candidate []byte) (result stagedFile, returnedErr error) {
	if ctx == nil || stage == nil || stage.journal == nil || !stage.journal.HasCollection(journal.Replacements) || previous.parent == nil || previous.file.identity == nil || len(preimage) == 0 || len(candidate) == 0 || len(preimage) > 64<<20 || len(candidate) > 64<<20 || !canonicalPath(filename) || filename == "/" {
		return stagedFile{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return stagedFile{}, err
	}
	if err := stage.observe(); err != nil {
		return stagedFile{}, err
	}
	if err := stage.validateReplacementAccess(filename, previous, candidate); err != nil {
		return stagedFile{}, err
	}
	native, ok := previous.file.identity.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != previous.file.gid || native.Nlink != 1 {
		return stagedFile{}, ErrBinding
	}
	name := digestHex([]byte(filename)) + ".json"
	var record fixedReplacementRecord
	if err := decodeCanonical(stage.journal.Bytes(journal.Replacements, name), 4<<10, &record); err != nil || record != (fixedReplacementRecord{Schema: "ardents-endpoint-installation-replacement-v1", GenerationDigest: stage.selected.GenerationDigest, Path: filename, Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(previous.file.mode), GID: previous.file.gid, PreviousDigest: digestHex(preimage), CandidateDigest: digestHex(candidate)}) {
		return stagedFile{}, errors.Join(ErrBinding, err)
	}
	frozen := bytes.Clone(candidate)
	mutation, err := fixedfile.Replace(ctx, filename, previous.parent, previous.file.identity, preimage, frozen, previous.file.mode, previous.file.gid)
	if mutation != nil {
		defer func() { returnedErr = errors.Join(returnedErr, fixedResourceError(mutation.Close())) }()
	}
	if err != nil {
		return stagedFile{}, fixedResourceError(err)
	}
	if err := stage.syncReplacementRecord(ctx, name); err != nil {
		return stagedFile{}, err
	}
	info, err := mutation.Commit()
	if info != nil {
		result = stagedFile{identity: info, body: frozen, mode: previous.file.mode, gid: previous.file.gid}
	}
	return result, fixedResourceError(err)
}
