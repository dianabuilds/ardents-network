package installation

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type generationTransition struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
	Phase            string `json:"phase"`
	OriginalError    string `json:"original_error,omitempty"`
}

type generationFileInventory struct {
	Schema           string   `json:"schema"`
	GenerationDigest string   `json:"generation_digest"`
	BindingDigest    string   `json:"binding_digest"`
	Files            []string `json:"files"`
}

type generationFileBirth struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Name             string `json:"name"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	Digest           string `json:"digest"`
	Size             int64  `json:"size"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
}

func generationFileRecordName(name string) string {
	return "generation-file-" + digestHex([]byte(name)) + ".json"
}

type generationBirth struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
}

// Only the genuine initial-preparation owner calls this after assembling the
// exact authorized bytes and observing actual account/root provenance.
func stageInitialGeneration(ctx context.Context, lease *installationLease, request Request, files map[string][]byte, selected generationSelection, gid uint32) (result *installationTransaction, returnedErr error) {
	if ctx == nil || lease == nil {
		return nil, ErrBinding
	}
	frozen, binding, err := freezeGenerationInventory(request, lease.path, files, selected, gid)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	stage := &installationTransaction{lease: lease, selected: selected}
	defer func() {
		if returnedErr != nil {
			stage.retainFailure(ctx, returnedErr)
			returnedErr = stage.close()
		}
	}()
	intentBody, err := canonicalJSON(initialTransitionIntent{Schema: "ardents-endpoint-installation-initial-v1", Candidate: selected, CandidateBinding: binding, Request: *request.declared})
	if err != nil || len(intentBody) > 128<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	stage.intent, err = writeStagedFile(ctx, lease.root, "transition.json", intentBody, 0600, 0)
	if err != nil {
		return nil, err
	}
	if err := syncStagingRoot(lease.root); err != nil {
		return nil, err
	}
	journalParent, err := stage.createContainer(ctx, lease.root, "journals")
	if err != nil {
		return nil, err
	}
	stage.journal, err = stage.createTransition(ctx, journalParent.root)
	if err != nil {
		return nil, err
	}
	if err := stage.record(ctx, "writing-generation", nil); err != nil {
		return nil, err
	}
	parent, err := stage.createContainer(ctx, lease.root, "generations")
	if err != nil {
		return nil, err
	}
	stage.generation, err = stage.createGeneration(ctx, parent.root, gid)
	if err != nil {
		return nil, err
	}
	native := stage.generation.Identity()
	birthBody, err := canonicalJSON(generationBirth{Schema: "ardents-endpoint-generation-directory-v1", GenerationDigest: selected.GenerationDigest, Device: native.Device, Inode: native.Inode})
	if err != nil {
		return nil, err
	}
	if err := stage.journal.Write(ctx, journal.Transitions, "generation-directory.json", birthBody); err != nil {
		return nil, err
	}
	if err := stage.writeGeneration(ctx, frozen); err != nil {
		return nil, err
	}
	return stage, nil
}

// Both initial and successor owners reach this only after their exact intent
// and physical directory birth record have become durable.
func (stage *installationTransaction) writeGeneration(ctx context.Context, frozen map[string][]byte) error {
	var binding generationBinding
	if decodeCanonical(frozen["binding.json"], 32<<10, &binding) != nil || binding.GID == 0 {
		return ErrBinding
	}
	marker, err := canonicalJSON(generationFileInventory{Schema: "ardents-endpoint-generation-files-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Files: generation.Names()})
	if err != nil {
		return err
	}
	if err := stage.journal.Write(ctx, journal.Transitions, "generation-files.json", marker); err != nil {
		return err
	}
	names := make([]string, 0, len(frozen))
	for name := range frozen {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := stage.observe(); err != nil {
			return err
		}
		birth, err := stage.generation.CreateFile(ctx, name)
		if err != nil {
			return err
		}
		native, ok := birth.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 || native.Gid != 0 || birth.Mode() != 0600 || birth.Size() != 0 {
			return ErrBinding
		}
		mode := uint32(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		record, err := canonicalJSON(generationFileBirth{Schema: "ardents-endpoint-generation-file-v1", GenerationDigest: stage.selected.GenerationDigest, Name: name, Device: uint64(native.Dev), Inode: native.Ino, Digest: digestHex(frozen[name]), Size: int64(len(frozen[name])), Mode: mode, GID: binding.GID})
		if err != nil {
			return err
		}
		if err := stage.journal.Write(ctx, journal.Transitions, generationFileRecordName(name), record); err != nil {
			return err
		}
		if err := stage.observe(); err != nil {
			return err
		}
		if err := stage.generation.Write(ctx, name, frozen[name]); err != nil {
			return err
		}
	}
	if err := stage.generation.Seal(ctx); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := stage.record(ctx, "generation-staged", nil); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (stage *installationTransaction) retainFailure(ctx context.Context, original error) {
	if stage.terminal == nil {
		stage.terminal = original
	}
	if stage.journal != nil {
		bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		stage.terminal = errors.Join(stage.terminal, stage.record(bounded, "generation-write-failed", original))
		cancel()
	}
}

func (stage *installationTransaction) record(ctx context.Context, phase string, original error) error {
	if (phase != "writing-generation" && phase != "generation-staged" && phase != "generation-write-failed") ||
		((phase == "generation-write-failed") != (original != nil)) {
		return ErrBinding
	}
	if original != nil {
		if first := stage.journal.Bytes(journal.Transitions, "original-transition-failure.json"); len(first) != 0 {
			// Preserve the first durable error. Failure-only bookkeeping retains
			// this caller's failure latch without occupying a fresh phase slot.
			return stage.journal.RecordFailure(ctx, "original-transition-failure.json", first, original)
		}
		if failed := stage.journal.Bytes(journal.Transitions, "0002.json"); len(failed) != 0 {
			var first generationTransition
			if decodeCanonical(failed, 64<<10, &first) == nil &&
				first.Schema == "ardents-endpoint-installation-transition-v1" &&
				first.GenerationDigest == stage.selected.GenerationDigest && first.BindingDigest == stage.selected.BindingDigest &&
				first.Phase == "generation-write-failed" && first.OriginalError != "" {
				// A second cleanup must not relabel or replace the first failure.
				return stage.journal.RecordFailure(ctx, "original-transition-failure.json", failed, original)
			}
		}
	}
	name := "0002.json"
	if phase == "writing-generation" {
		name = "0001.json"
	}
	if len(stage.journal.Bytes(journal.Transitions, name)) != 0 {
		if original == nil {
			return ErrBinding
		}
		name, phase = "original-transition-failure.json", "successor-transition-failed"
	}
	record := generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: phase}
	if original != nil {
		record.OriginalError = original.Error()
	}
	body, err := canonicalJSON(record)
	if err != nil || len(body) > 64<<10 {
		return errors.Join(ErrBinding, err)
	}
	if original != nil {
		return stage.journal.RecordFailure(ctx, name, body, original)
	}
	return stage.journal.Write(ctx, journal.Transitions, name, body)
}

func (stage *installationTransaction) promoteAccess(ctx context.Context, gid uint32) error {
	if ctx == nil || gid == 0 || gid == 1<<32-1 {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	native := stage.generation.Identity()
	if native.GID != gid {
		return ErrBinding
	}
	if len(stage.journal.Bytes(journal.Transitions, "0004.json")) == 0 {
		return ErrBinding
	}
	var generations *installationContainer
	for _, directory := range stage.directories {
		if directory.parent == stage.lease.root && directory.name == "generations" {
			generations = directory
		}
	}
	if generations == nil {
		return ErrBinding
	}
	info, err := promoteReadDirectory(ctx, filepath.Join(stage.lease.path, "generations"), generations.root, generations.identity, gid)
	if info != nil {
		generations.identity = info
	}
	if err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	info, err = promoteReadDirectory(ctx, stage.lease.path, stage.lease.root, stage.lease.identity, gid)
	if info != nil {
		stage.lease.identity = info
	}
	if err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	return ctx.Err()
}

func promoteReadDirectory(ctx context.Context, path string, root *os.Root, original os.FileInfo, gid uint32) (result os.FileInfo, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pathInfo, pathErr := os.Lstat(path)
	rootInfo, rootErr := root.Stat(".")
	if pathErr != nil || rootErr != nil || !sameObservedDirectory(original, pathInfo) || !sameObservedDirectory(original, rootInfo) {
		return nil, errors.Join(ErrBinding, pathErr, rootErr)
	}
	native, ok := original.Sys().(*syscall.Stat_t)
	if !ok || gid == 0 || gid == 1<<32-1 {
		return nil, ErrBinding
	}
	if original.Mode() == os.ModeDir|0750 && native.Gid == gid {
		return original, ctx.Err()
	}
	if original.Mode() != os.ModeDir|0700 || native.Gid != 0 {
		return nil, ErrBinding
	}
	file, err := root.OpenFile(".", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !sameObservedDirectory(original, before) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return nil, err
	}
	if err := file.Chmod(0750); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(original, info) || info.Mode() != os.ModeDir|0750 {
		return nil, errors.Join(ErrBinding, err)
	}
	actual, ok := info.Sys().(*syscall.Stat_t)
	if !ok || actual.Uid != 0 || actual.Gid != gid {
		return nil, ErrBinding
	}
	pathInfo, pathErr = os.Lstat(path)
	rootInfo, rootErr = root.Stat(".")
	if pathErr != nil || rootErr != nil || !sameObservedDirectory(info, pathInfo) || !sameObservedDirectory(info, rootInfo) {
		return nil, errors.Join(ErrBinding, pathErr, rootErr)
	}
	// Preserve the changed same inode even if the original caller goes after
	// durability. Its owner returns refusal and retains explicit failure intent.
	return info, ctx.Err()
}

// Creation and recovery retain different physical owners. Recovery reads a
// sealed Snapshot; it never fabricates a generation's exclusive birth Owner.
func (stage *installationTransaction) generationBytes(name string) []byte {
	if stage == nil {
		return nil
	}
	if stage.generation != nil {
		return stage.generation.Bytes(name)
	}
	return stage.sealed.Bytes(name)
}

func (stage *installationTransaction) generationIdentity() generation.Identity {
	if stage == nil {
		return generation.Identity{}
	}
	if stage.generation != nil {
		return stage.generation.Identity()
	}
	if stage.prefix != nil {
		return stage.prefix.Identity()
	}
	if stage.sealed == nil || stage.sealedIdentity == nil {
		return generation.Identity{}
	}
	info, ok := stage.sealedIdentity.Sys().(*syscall.Stat_t)
	if !ok {
		return generation.Identity{}
	}
	return generation.Identity{Device: uint64(info.Dev), Inode: info.Ino, GID: info.Gid, Mode: stage.sealedIdentity.Mode()}
}

// A complete sealed prefix may have the old directory-only provenance, or all
// fifteen new exact file births. A partial new set cannot look like an old one.
// These facts constrain original custody; fresh Release proofs remain separate.
func readGenerationFileBirths(ctx context.Context, reader *installedRoot, directory string, selected generationSelection, binding generationBinding, expected map[string][]byte, inventory map[string]bool) (map[string]journal.RetainedRecord, error) {
	records := make(map[string]journal.RetainedRecord)
	markerPath := filepath.Join(directory, "generation-files.json")
	marker, markerErr := reader.read(ctx, markerPath, 4<<10, 0600, 0)
	present := !errors.Is(markerErr, os.ErrNotExist)
	if present {
		expectedMarker, err := canonicalJSON(generationFileInventory{Schema: "ardents-endpoint-generation-files-v1", GenerationDigest: selected.GenerationDigest, BindingDigest: selected.BindingDigest, Files: generation.Names()})
		if markerErr != nil || err != nil || !bytes.Equal(marker, expectedMarker) {
			return nil, errors.Join(ErrBinding, markerErr, err)
		}
		inventory["generation-files.json"] = true
		records["generation-files.json"] = journal.RetainedRecord{Identity: reader.files[markerPath].identity, Bytes: marker}
	}
	for _, name := range generation.Names() {
		path := filepath.Join(directory, generationFileRecordName(name))
		body, err := reader.read(ctx, path, 4<<10, 0600, 0)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if !present {
			return nil, ErrBinding
		}
		var birth generationFileBirth
		mode := uint32(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		digest := binding.Files[name]
		if name == "binding.json" {
			digest = selected.BindingDigest
		}
		if err != nil || decodeCanonical(body, 4<<10, &birth) != nil || birth.Schema != "ardents-endpoint-generation-file-v1" || birth.GenerationDigest != selected.GenerationDigest || birth.Name != name || birth.Digest != digest || !canonicalDigest(digest) || birth.Size < 1 || birth.Size > 64<<20 || birth.Mode != mode || birth.GID != reader.gid || birth.Device == 0 || birth.Inode == 0 {
			return nil, errors.Join(ErrBinding, err)
		}
		if name == "binding.json" {
			encoded, err := canonicalJSON(binding)
			if err != nil || birth.Size != int64(len(encoded)) {
				return nil, errors.Join(ErrBinding, err)
			}
		} else if expected != nil && (len(expected[name]) == 0 || birth.Size != int64(len(expected[name]))) {
			return nil, ErrBinding
		}
		filename := filepath.Join(reader.lease.path, "generations", selected.GenerationDigest, name)
		actual, err := reader.readObserved(ctx, filename, birth.Size, os.FileMode(mode), reader.gid, true)
		observed := reader.files[filename]
		if err != nil || observed.identity == nil || int64(len(actual)) > birth.Size {
			return nil, errors.Join(ErrBinding, err)
		}
		native, ok := observed.identity.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != birth.Device || native.Ino != birth.Inode || int64(len(actual)) == birth.Size && digestHex(actual) != birth.Digest {
			return nil, ErrBinding
		}
		recordName := generationFileRecordName(name)
		inventory[recordName] = true
		records[recordName] = journal.RetainedRecord{Identity: reader.files[path].identity, Bytes: body}
	}
	if present && len(records) != len(generation.Names())+1 {
		return nil, ErrBinding
	}
	return records, ctx.Err()
}
