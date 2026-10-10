//go:build installation_native

package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Actual leased filesystem custody with no successful Release/manager/runtime
// fixture: public copied handles must retain the original cancelled caller,
// refuse completion once, and physically close the original writer descriptor.
func TestInstallationNativeSuccessorCopiedHandleRetainsOriginalCancellation(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	reader, request, _, _, _ := successorStagingFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	native := &successorPreparation{ctx: ctx, request: request, inspection: reader}
	owner := &Successor{state: &successorOperation{ctx: ctx, request: request, native: native}}
	copied := *owner
	writer := reader.lease.writer
	cancel()
	result, err := owner.Complete(nil, enrollment.Candidate{}, release.Inputs{})
	if result != (ProvisionResult{}) || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled original admitted replacement", result, err)
	}
	if _, err := writer.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("original writer descriptor remains open", err)
	}
	if owner.state.native != nil || !owner.state.used {
		t.Fatal("physical release or one-use completion not retained")
	}
	if result, err := copied.Complete(nil, enrollment.Candidate{}, release.Inputs{}); result != (ProvisionResult{}) || !errors.Is(err, ErrInput) || !errors.Is(err, context.Canceled) {
		t.Fatal("copied handle renewed or lost original outcome", result, err)
	}
	if err := copied.Close(); !errors.Is(err, context.Canceled) {
		t.Fatal("copied close erased original cancellation", err)
	}
}

// Advance filesystem fixtures only, not Enrollment/Release/account/manager
// authority. Native staging is separately constrained by the genuine caller.
func successorStagingFixture(t *testing.T) (*installedRoot, Request, generationSelection, map[string][]byte, generationSelection) {
	t.Helper()
	previous := archiveStage(t)
	if err := previous.archiveIntent(t.Context()); err != nil {
		t.Fatal(err)
	}
	reader := &installedRoot{lease: previous.lease, installedFiles: &installedFiles{directory: previous.lease.path, gid: 65534, files: map[string]fileObservation{}, directories: map[string]os.FileInfo{}, mutableDirectories: map[string]os.FileInfo{}}}
	reader.directories[previous.lease.path] = previous.lease.identity
	reader.files[filepath.Join(previous.lease.path, "selection.json")] = previous.fixed[filepath.Join(previous.lease.path, "selection.json")].file
	for _, name := range generation.Names() {
		filename := filepath.Join(previous.lease.path, "generations", previous.selected.GenerationDigest, name)
		info, err := os.Lstat(filename)
		if err != nil {
			t.Fatal(err)
		}
		reader.files[filename] = fileObservation{identity: info, body: previous.generation.Bytes(name), gid: 65534, mode: info.Mode()}
	}
	for _, directory := range previous.directories {
		if directory.parent == previous.lease.root && directory.name == "generations" {
			reader.directories[filepath.Join(previous.lease.path, "generations")] = directory.identity
		}
	}
	generationPath := filepath.Join(previous.lease.path, "generations", previous.selected.GenerationDigest)
	generationInfo, err := os.Lstat(generationPath)
	if err != nil {
		t.Fatal(err)
	}
	reader.directories[generationPath] = generationInfo
	var initial initialTransitionIntent
	if err := json.Unmarshal(previous.intent.body, &initial); err != nil {
		t.Fatal(err)
	}
	initial.Request.ManifestSHA256 = ""
	raw, err := canonicalJSON(initial.Request)
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodeRequest(t.Context(), raw, false)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, name := range generation.Names() {
		files[name] = previous.generation.Bytes(name)
	}
	files["protected-endpoint.json"] = []byte("successor mechanism descriptor\n")
	files["request.json"] = raw
	var binding generationBinding
	if err := json.Unmarshal(files["binding.json"], &binding); err != nil {
		t.Fatal(err)
	}
	binding.GenerationDigest = digestHex(files["protected-endpoint.json"])
	for name, body := range files {
		if name != "binding.json" {
			binding.Files[name] = digestHex(body)
		}
	}
	files["binding.json"], err = canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	selected := generationSelection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: binding.GenerationDigest, BindingDigest: digestHex(files["binding.json"])}
	if err := previous.close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := freezeGenerationInventory(request, reader.lease.path, files, selected, 65534); err != nil {
		t.Fatal("candidate mechanism inventory differs", err)
	}
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal("predecessor mechanism inventory differs", err)
	}
	return reader, request, previous.selected, files, selected
}

func TestInstallationNativeSuccessorStagesWithoutChangingPredecessor(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	reader, request, previous, files, selected := successorStagingFixture(t)
	stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stage.close(); err != nil {
			t.Error(err)
		}
	}()
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal("predecessor bytes/inodes/access changed", err)
	}
	var intent successorTransitionIntent
	if err := decodeCanonical(stage.intent.body, 128<<10, &intent); err != nil || intent.Schema != "ardents-endpoint-installation-successor-v1" || intent.Previous != previous || intent.Candidate != selected {
		t.Fatal("owned successor intent differs", err)
	}
	info, err := os.Lstat(filepath.Join(reader.lease.path, "generations", selected.GenerationDigest))
	if err != nil {
		t.Fatal(err)
	}
	var birth generationBirth
	if err := json.Unmarshal(stage.journal.Bytes(journal.Transitions, "generation-directory.json"), &birth); err != nil {
		t.Fatal(err)
	}
	native := info.Sys().(*syscall.Stat_t)
	if birth.Device != uint64(native.Dev) || birth.Inode != native.Ino || birth.GenerationDigest != selected.GenerationDigest {
		t.Fatal("birth record adopted another physical generation")
	}
	for name, wanted := range files {
		actual, err := os.ReadFile(filepath.Join(reader.lease.path, "generations", selected.GenerationDigest, name))
		if err != nil || !bytes.Equal(actual, wanted) {
			t.Fatal("candidate bytes differ", name, err)
		}
		files[name][0] ^= 1
	}
	if err := stage.observe(); err != nil {
		t.Fatal("caller mutation changed staged generation", err)
	}
}

func TestInstallationNativeSuccessorRefusesResidueBeforeIntent(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	for _, parent := range []string{"journals", "generations", "foreign-generations-parent"} {
		t.Run(parent, func(t *testing.T) {
			reader, request, previous, files, selected := successorStagingFixture(t)
			if parent == "foreign-generations-parent" {
				path := filepath.Join(reader.lease.path, "generations")
				if err := os.Rename(path, path+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(path, 0, 65534); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(reader.lease.path, parent, selected.GenerationDigest), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534); stage != nil || !errors.Is(err, ErrBinding) {
				t.Fatal("candidate residue adopted", err)
			}
			if _, err := reader.lease.root.Lstat("transition.json"); !os.IsNotExist(err) {
				t.Fatal("residue refusal wrote intent", err)
			}
		})
	}
}

type successorIntentCancellation struct {
	context.Context
	root string
}

func (ctx successorIntentCancellation) Err() error {
	if _, err := os.Lstat(filepath.Join(ctx.root, "transition.json")); err == nil {
		return context.Canceled
	}
	return ctx.Context.Err()
}

func TestInstallationNativeSuccessorCancellationRetainsIntentAndPredecessor(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	reader, request, previous, files, selected := successorStagingFixture(t)
	ctx := successorIntentCancellation{Context: t.Context(), root: reader.lease.path}
	stage, err := stageSuccessorGeneration(ctx, reader, request, previous, files, selected, 65534)
	if stage == nil || !errors.Is(err, context.Canceled) || !stage.intentAttempted {
		t.Fatal("original cancellation became staged success", err)
	}
	defer func() {
		if err := stage.close(); !errors.Is(err, context.Canceled) {
			t.Error("physical close erased original cancellation", err)
		}
	}()
	for _, parent := range stage.parents {
		if parent.root == nil || parent.file == nil {
			t.Fatal("failed intent released borrowed parent custody")
		}
		if _, err := parent.file.Stat(); err != nil {
			t.Fatal("failed intent closed original parent", err)
		}
	}
	if _, err := reader.lease.root.Lstat("transition.json"); err != nil {
		t.Fatal("failed staging erased original intent", err)
	}
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal("failed staging changed predecessor", err)
	}
}

func TestInstallationNativeFailedIntentCloseRetainsUnjoinedCustody(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	reader, request, previous, files, selected := successorStagingFixture(t)
	ctx := successorIntentCancellation{Context: t.Context(), root: reader.lease.path}
	stage, err := stageSuccessorGeneration(ctx, reader, request, previous, files, selected, 65534)
	if stage == nil || !errors.Is(err, context.Canceled) {
		t.Fatal("missing failed original staging custody", err)
	}
	defer func() { _ = stage.close() }()
	first := errors.New("original staging failure")
	owned := &successorPreparation{ctx: ctx, inspection: reader, stage: stage, terminal: first}
	// Deliberately incomplete predecessor observation: this failure control
	// supplies no valid manager, process, cgroup or completed physical join.
	original := &installedPredecessor{preparation: owned}
	owned.predecessor = original
	for range 2 {
		if err := owned.close(); !errors.Is(err, first) || !errors.Is(err, ErrBinding) || !errors.Is(err, context.Canceled) {
			t.Fatal("unfinished intent became completed cleanup", err)
		}
		if owned.predecessor != original || owned.stage != stage || owned.inspection != reader || reader.lease.root == nil || reader.lease.writer == nil {
			t.Fatal("unfinished original custody was released")
		}
		if _, err := reader.lease.writer.Stat(); err != nil {
			t.Fatal("unfinished original writer lease was closed", err)
		}
	}
}

// Filesystem phases only, without any successful Release/manager/predecessor
// substitute. These tests cannot establish a reloaded or started Installation.
func successorSelectionFixture(t *testing.T, publishing bool) (*installationTransaction, string, fixedFileObservation, []byte) {
	t.Helper()
	reader, request, previous, files, selected := successorStagingFixture(t)
	stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stage.close() })
	for _, phase := range []struct{ name, value string }{{"0004.json", "fixed-resources-replaced"}, {"0005.json", "publishing-selection"}} {
		if phase.name == "0005.json" && !publishing {
			break
		}
		body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: selected.GenerationDigest, BindingDigest: selected.BindingDigest, Phase: phase.value})
		if err != nil {
			t.Fatal(err)
		}
		if err := stage.journal.Write(t.Context(), journal.Transitions, phase.name, body); err != nil {
			t.Fatal(err)
		}
	}
	filename := filepath.Join(stage.lease.path, "selection.json")
	body, err := canonicalJSON(selected)
	if err != nil {
		t.Fatal(err)
	}
	return stage, filename, stage.fixed[filename], body
}

func TestInstallationNativeSuccessorSelectionPreservesOriginalInodeAndGroup(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage, filename, previous, body := successorSelectionFixture(t, true)
	if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, body); err != nil {
		t.Fatal(err)
	}
	name := digestHex([]byte(filename)) + ".json"
	recordPath := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "replacements", name)
	identity, err := os.Lstat(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	recordFile := fileObservation{identity: identity, body: stage.journal.Bytes(journal.Replacements, name), mode: 0600}
	var record fixedReplacementRecord
	if err := json.Unmarshal(recordFile.body, &record); err != nil || record.Mode != 0640 || record.GID != 65534 || record.PreviousDigest != digestHex(previous.file.body) || record.CandidateDigest != digestHex(body) {
		t.Fatal("selection access or byte binding differs", err)
	}
	written, err := stage.replaceRecordedFixedFile(t.Context(), filename, previous, previous.file.body, body)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(previous.file.identity, written.identity) || written.mode != 0640 || written.gid != 65534 || !bytes.Equal(written.body, body) {
		t.Fatal("selection replacement changed original inode/access")
	}
	stage.fixed[filename] = fixedFileObservation{parent: previous.parent, file: written}
	if err := stage.observe(); err != nil {
		t.Fatal("refreshed original observation differs", err)
	}
	if _, err := stage.replaceRecordedFixedFile(t.Context(), filename, previous, previous.file.body, body); err != nil {
		t.Fatal("same-record selection retry refused", err)
	}
	actual, readErr := os.ReadFile(recordPath)
	after, statErr := os.Lstat(recordPath)
	if readErr != nil || statErr != nil || !observedFileMatches(recordFile, after) || !bytes.Equal(actual, recordFile.body) {
		t.Fatal("retry changed original record", readErr, statErr)
	}
	for _, name := range []string{"0006.json", "0007.json"} {
		if len(stage.journal.Bytes(journal.Transitions, name)) != 0 {
			t.Fatal("selection fabricated reload/start", name)
		}
	}
}

func TestInstallationNativeSuccessorSelectionRefusesUnboundAccessAndPhases(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	for _, change := range []string{"phase", "mode", "group", "parent", "previous", "candidate", "foreign-path"} {
		t.Run(change, func(t *testing.T) {
			stage, filename, previous, body := successorSelectionFixture(t, change != "phase")
			switch change {
			case "mode":
				previous.file.mode = 0644
			case "group":
				previous.file.gid = 0
			case "parent":
				var err error
				previous.parent, err = os.Lstat(filepath.Dir(stage.lease.path))
				if err != nil {
					t.Fatal(err)
				}
			case "previous":
				previous.file.body = []byte("foreign predecessor selection")
			case "candidate":
				body = []byte("foreign candidate selection")
			case "foreign-path":
				filename = filepath.Join(stage.lease.path, "foreign-selection.json")
			}
			if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, body); !errors.Is(err, ErrBinding) {
				t.Fatal("unbound selection accepted", err)
			}
			if stage.journal.HasCollection(journal.Replacements) {
				t.Fatal("selection refusal created provenance")
			}
			if err := observeFixedFile(filepath.Join(stage.lease.path, "selection.json"), stage.fixed[filepath.Join(stage.lease.path, "selection.json")]); err != nil {
				t.Fatal("refusal changed original selection", err)
			}
		})
	}
}
