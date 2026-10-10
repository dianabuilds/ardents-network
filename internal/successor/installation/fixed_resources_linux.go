package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
)

// Fixed images are detached bytes. Their construction grants no filesystem,
// Release, selection or startup authority; each native operation retains its
// own admission, original handles, mutation order and physical completion.
// Missing destinations never authorize foreign inode adoption or grant
// platform/Execution admission.
func fixedResourceNames() map[string]string {
	return map[string]string{
		"/usr/lib/ardents/text-worker-root/ardents-text":      "ardents-text-linux-amd64",
		"/etc/systemd/system/ardents-text-reader@.service":    "ardents-text-reader@.service",
		"/etc/systemd/system/ardents-text-publisher@.service": "ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket":      "ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-publisher.socket":   "ardents-text-publisher.socket",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules":   "50-ardents-text.rules",
		"/usr/lib/tmpfiles.d/ardents-text.conf":               "ardents-text.conf",
		"/etc/systemd/system/ardents-endpoint.service":        "ardents-endpoint.service",
	}
}

func fixedResourceImages(files map[string][]byte) (map[string][]byte, error) {
	resources := make(map[string][]byte)
	digests := make(map[string]string)
	for filename, name := range fixedResourceNames() {
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		body := files[name]
		if len(body) == 0 {
			return nil, ErrBinding
		}
		resources[filename] = body
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			digests[filename] = digestHex(body)
		}
	}
	manifest, err := fixedArtifactManifest(digests)
	if err != nil {
		return nil, err
	}
	resources["/etc/ardents/text-worker-artifact.json"] = manifest
	return resources, nil
}

func fixedArtifactManifest(digests map[string]string) ([]byte, error) {
	return canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", digests})
}

// Root-controlled presence alone does not identify a directory created by this
// transaction. Retain the original inode and intended access before mutation;
// explicit recovery must validate these bytes rather than adopt a pathname.
type fixedDirectoryCreation struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Path             string `json:"path"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	PreviousMode     uint32 `json:"previous_mode"`
	PreviousGID      uint32 `json:"previous_gid"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
}

func (stage *installationTransaction) recordFixedDirectory(ctx context.Context, directory string, original os.FileInfo, mode os.FileMode, gid uint32, promotion bool) error {
	if stage == nil || stage.journal == nil || original == nil || !canonicalPath(directory) || directory == "/" ||
		(mode != 0700 && mode != 0755 && mode != 0555 && mode != 0710) ||
		((mode == 0710) != (gid != 0)) {
		return ErrInput
	}
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		return ErrBinding
	}
	native, ok := original.Sys().(*syscall.Stat_t)
	if !ok || !privateJournalDirectory(original) || native.Ino == 0 || native.Dev == 0 {
		return ErrBinding
	}
	if promotion && mode != 0555 {
		return ErrBinding
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := stage.journal.Ensure(ctx, journal.DirectoryCreations); err != nil {
		return err
	}
	record := fixedDirectoryCreation{Schema: "ardents-endpoint-directory-creation-v1", GenerationDigest: stage.selected.GenerationDigest,
		Path: directory, Device: uint64(native.Dev), Inode: native.Ino, PreviousMode: uint32(original.Mode().Perm()), PreviousGID: native.Gid, Mode: uint32(mode), GID: gid}
	body, err := canonicalJSON(record)
	if err != nil {
		return err
	}
	name := digestHex([]byte(directory))
	if promotion {
		name += "-access"
	}
	return stage.journal.Write(ctx, journal.DirectoryCreations, name+".json", body)
}

func (stage *installationTransaction) fixedPhase(ctx context.Context, name, phase string) error {
	if name == "0003.json" && phase == "installing-fixed-resources" {
		if len(stage.journal.Bytes(journal.Transitions, "0002.json")) == 0 {
			return ErrBinding
		}
	} else if name == "0004.json" && phase == "fixed-resources-installed" {
		if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
			return ErrBinding
		}
	} else if name == "0005.json" && phase == "publishing-selection" {
		if len(stage.journal.Bytes(journal.Transitions, "0004.json")) == 0 {
			return ErrBinding
		}
	} else if name == "0006.json" && phase == "reloading-manager" {
		if len(stage.journal.Bytes(journal.Transitions, "0005.json")) == 0 {
			return ErrBinding
		}
		if _, selected := stage.fixed[filepath.Join(stage.lease.path, "selection.json")]; !selected {
			return ErrBinding
		}
	} else if name == "0007.json" && phase == "installed-stopped" {
		if len(stage.journal.Bytes(journal.Transitions, "0006.json")) == 0 {
			return ErrBinding
		}
	} else {
		return ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: phase})
	if err != nil {
		return err
	}
	return stage.journal.Write(ctx, journal.Transitions, name, body)
}

func (stage *installationTransaction) ensureRootParent(ctx context.Context, directory string) error {
	if _, err := os.Lstat(directory); os.IsNotExist(err) {
		return stage.birthFixedDirectory(ctx, directory, 0755, 0)
	}
	_, err := rootDirectoryAncestors(directory)
	return err
}

func (stage *installationTransaction) birthFixedDirectory(ctx context.Context, directory string, mode os.FileMode, gid uint32) (returnedErr error) {
	if stage == nil || stage.journal == nil || !canonicalPath(directory) || directory == "/" ||
		(mode != 0700 && mode != 0755 && mode != 0555 && mode != 0710) || ((mode == 0710) != (gid != 0)) {
		return ErrInput
	}
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		return ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := requireAbsentManagedPath(directory); err != nil {
		return err
	}
	if err := stage.ensureRootParent(ctx, filepath.Dir(directory)); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	pathBirth, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !privateJournalDirectory(birth) || !os.SameFile(birth, pathBirth) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if err := errors.Join(file.Sync(), syncDirectDirectory(filepath.Dir(directory))); err != nil {
		return err
	}
	if err := stage.recordFixedDirectory(ctx, directory, birth, mode, gid, false); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := file.Stat()
	currentPath, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !sameObservedDirectory(birth, current) || !sameObservedDirectory(birth, currentPath) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	info, err := file.Stat()
	pathInfo, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !os.SameFile(birth, info) || !sameObservedDirectory(info, pathInfo) || info.Mode() != os.ModeDir|mode {
		return errors.Join(ErrBinding, err, pathErr)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != gid {
		return ErrBinding
	}
	if stage.fixedDirectories == nil {
		stage.fixedDirectories = make(map[string]os.FileInfo)
	}
	stage.fixedDirectories[directory] = info
	return errors.Join(syncDirectDirectory(filepath.Dir(directory)), ctx.Err())
}

func (stage *installationTransaction) changeFixedDirectoryMode(ctx context.Context, directory string, mode os.FileMode) (returnedErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	original := stage.fixedDirectories[directory]
	file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !sameObservedDirectory(original, before) {
		return errors.Join(ErrBinding, err)
	}
	native, ok := before.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrBinding
	}
	if err := stage.recordFixedDirectory(ctx, directory, before, mode, native.Gid, true); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := file.Stat()
	currentPath, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !sameObservedDirectory(before, current) || !sameObservedDirectory(before, currentPath) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	info, err := file.Stat()
	pathInfo, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !os.SameFile(original, info) || !sameObservedDirectory(info, pathInfo) || info.Mode() != os.ModeDir|mode {
		return errors.Join(ErrBinding, err, pathErr)
	}
	stage.fixedDirectories[directory] = info
	// The owned parent changed access on this exact inode. Its already pinned
	// children keep their original file identities and bytes, but must observe
	// the new parent mode rather than treating our promotion as substitution.
	for filename, observation := range stage.fixed {
		if filepath.Dir(filename) == directory {
			if !sameObservedDirectory(original, observation.parent) {
				return ErrBinding
			}
			observation.parent = info
			stage.fixed[filename] = observation
		}
	}
	return errors.Join(stage.observe(), ctx.Err())
}

// Preserve creation-v1 separately from successor truncation records. A birth
// recorded here owns precisely one inode, never another file at the same path.
type fixedCreationRecord struct {
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

type fixedFileObservation struct {
	parent os.FileInfo
	file   fileObservation
}

// The transaction must observe manager absence and its original account/roots
// before selecting fixed effects. This mechanism itself grants no admission.
// Every failure retains the born leaf and journal for explicit repair.
func (stage *installationTransaction) createFixedFile(ctx context.Context, filename string, body []byte, mode os.FileMode, gid uint32) (returnedErr error) {
	if stage == nil || stage.journal == nil || ctx == nil || !canonicalPath(filename) || filename == "/" || len(body) == 0 || len(body) > 64<<20 ||
		(mode != 0644 && mode != 0555 && mode != 0640) {
		return ErrInput
	}
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		return ErrBinding
	}
	isSelection := filename == filepath.Join(stage.lease.path, "selection.json")
	if isSelection {
		wanted, err := canonicalJSON(stage.selected)
		if len(stage.journal.Bytes(journal.Transitions, "0005.json")) == 0 || err != nil || !bytes.Equal(body, wanted) || mode != 0640 {
			return errors.Join(ErrBinding, err)
		}
		native, ok := stage.lease.identity.Sys().(*syscall.Stat_t)
		if !ok || native.Gid != gid || gid == 0 || stage.lease.identity.Mode() != os.ModeDir|0750 {
			return ErrBinding
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if !isSelection {
		if _, err := rootDirectoryAncestors(filepath.Dir(filename)); err != nil {
			return err
		}
	}
	if err := stage.journal.Ensure(ctx, journal.Creations); err != nil {
		return err
	}
	parentInfo, err := os.Lstat(filepath.Dir(filename))
	trustedParent := rootDirectory(parentInfo)
	if isSelection {
		trustedParent = sameObservedDirectory(stage.lease.identity, parentInfo)
	}
	if err != nil || !trustedParent {
		return errors.Join(ErrBinding, err)
	}
	frozen := bytes.Clone(body)
	mutation, err := fixedfile.Create(ctx, filename, parentInfo, frozen, mode, gid)
	if mutation != nil {
		defer func() { returnedErr = errors.Join(returnedErr, fixedResourceError(mutation.Close())) }()
	}
	if err != nil {
		return fixedResourceError(err)
	}
	birth := mutation.Identity()
	native, ok := birth.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrBinding
	}
	record := fixedCreationRecord{Schema: "ardents-endpoint-installation-creation-v1", GenerationDigest: stage.selected.GenerationDigest,
		Path: filename, Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(mode), GID: gid, CandidateDigest: digestHex(frozen)}
	recordBody, err := canonicalJSON(record)
	if err != nil || len(recordBody) > 64<<10 {
		return errors.Join(ErrBinding, err)
	}
	if err := stage.journal.Write(ctx, journal.Creations, digestHex([]byte(filename))+".json", recordBody); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	// The physical owner rechecks the original empty inode after journal I/O.
	info, err := mutation.Commit()
	if info != nil {
		if stage.fixed == nil {
			stage.fixed = make(map[string]fixedFileObservation)
		}
		stage.fixed[filename] = fixedFileObservation{parent: parentInfo, file: fileObservation{identity: info, body: frozen, mode: mode, gid: gid}}
	}
	return fixedResourceError(err)
}

// Physical failures retain their native causes and the domain's error classes.
func fixedResourceError(err error) error {
	if errors.Is(err, fixedfile.ErrBinding) {
		return errors.Join(ErrBinding, err)
	}
	if errors.Is(err, fixedfile.ErrInput) {
		return errors.Join(ErrInput, err)
	}
	return err
}

func observeFixedFile(filename string, expected fixedFileObservation) (returnedErr error) {
	parentInfo, err := os.Lstat(filepath.Dir(filename))
	if err != nil || !sameObservedDirectory(expected.parent, parentInfo) {
		return errors.Join(ErrBinding, err)
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	handle, err := root.Stat(".")
	if err != nil || !sameObservedDirectory(expected.parent, handle) {
		return errors.Join(ErrBinding, err)
	}
	if err := observeStagedFile(root, filepath.Base(filename), expected.file); err != nil {
		return err
	}
	after, err := os.Lstat(filepath.Dir(filename))
	if err != nil || !sameObservedDirectory(expected.parent, after) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

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
			stage.lease.identity.Mode() != os.ModeDir|0750 || !sameObservedDirectory(stage.lease.identity, previous.parent) {
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
func (stage *installationTransaction) replaceRecordedFixedFile(ctx context.Context, filename string, previous fixedFileObservation, preimage, candidate []byte) (result fileObservation, returnedErr error) {
	if ctx == nil || stage == nil || stage.journal == nil || !stage.journal.HasCollection(journal.Replacements) || previous.parent == nil || previous.file.identity == nil || len(preimage) == 0 || len(candidate) == 0 || len(preimage) > 64<<20 || len(candidate) > 64<<20 || !canonicalPath(filename) || filename == "/" {
		return fileObservation{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return fileObservation{}, err
	}
	if err := stage.observe(); err != nil {
		return fileObservation{}, err
	}
	if err := stage.validateReplacementAccess(filename, previous, candidate); err != nil {
		return fileObservation{}, err
	}
	native, ok := previous.file.identity.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != previous.file.gid || native.Nlink != 1 {
		return fileObservation{}, ErrBinding
	}
	name := digestHex([]byte(filename)) + ".json"
	var record fixedReplacementRecord
	if err := decodeCanonical(stage.journal.Bytes(journal.Replacements, name), 4<<10, &record); err != nil || record != (fixedReplacementRecord{Schema: "ardents-endpoint-installation-replacement-v1", GenerationDigest: stage.selected.GenerationDigest, Path: filename, Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(previous.file.mode), GID: previous.file.gid, PreviousDigest: digestHex(preimage), CandidateDigest: digestHex(candidate)}) {
		return fileObservation{}, errors.Join(ErrBinding, err)
	}
	frozen := bytes.Clone(candidate)
	mutation, err := fixedfile.Replace(ctx, filename, previous.parent, previous.file.identity, preimage, frozen, previous.file.mode, previous.file.gid)
	if mutation != nil {
		defer func() { returnedErr = errors.Join(returnedErr, fixedResourceError(mutation.Close())) }()
	}
	if err != nil {
		return fileObservation{}, fixedResourceError(err)
	}
	if err := stage.syncReplacementRecord(ctx, name); err != nil {
		return fileObservation{}, err
	}
	info, err := mutation.Commit()
	if info != nil {
		result = fileObservation{identity: info, body: frozen, mode: previous.file.mode, gid: previous.file.gid}
	}
	return result, fixedResourceError(err)
}
