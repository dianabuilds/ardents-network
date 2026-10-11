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

	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
)

func fixedCreationStage(t *testing.T) *installationTransaction {
	t.Helper()
	lease, request, files, selected := nativeStagingFixture(t)
	stage, err := stageInitialGeneration(t.Context(), lease, request, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stage.close() })
	if err := stage.fixedPhase(t.Context(), "0003.json", "installing-fixed-resources"); err != nil {
		t.Fatal(err)
	}
	return stage
}

func TestInstallationNativeFixedCreationRecordsActualBirthBeforePayload(t *testing.T) {
	stage := fixedCreationStage(t)
	filename := filepath.Join(nativeRequestDirectory(t), "fixed")
	body := []byte("fixed resource bytes\n")
	if err := stage.createFixedFile(t.Context(), filename, body, 0644, 0); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("actual resource differs", err)
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "creations", digestHex([]byte(filename))+".json")
	recorded, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
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
	info, statErr := os.Lstat(filename)
	if statErr != nil {
		t.Fatal(statErr)
	}
	native := info.Sys().(*syscall.Stat_t)
	if err := json.Unmarshal(recorded, &record); err != nil || record.Schema != "ardents-endpoint-installation-creation-v1" || record.GenerationDigest != stage.selected.GenerationDigest || record.Path != filename || record.Device != uint64(native.Dev) || record.Inode != native.Ino || record.Mode != 0644 || record.GID != 0 || record.PreviousDigest != "" || record.CandidateDigest != digestHex(body) {
		t.Fatal("creation binding differs", err)
	}
	if err := stage.createFixedFile(t.Context(), filename, body, 0644, 0); err == nil {
		t.Fatal("existing file adopted")
	}
	if err := os.WriteFile(filename, []byte("changed bytes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(stage.observe(), ErrBinding) {
		t.Fatal("changed owned resource accepted")
	}
}

func TestInstallationNativeFixedCreationRequiresOriginalPhase(t *testing.T) {
	lease, request, files, selected := nativeStagingFixture(t)
	stage, err := stageInitialGeneration(t.Context(), lease, request, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stage.close(); err != nil {
			t.Error(err)
		}
	}()
	filename := filepath.Join(nativeRequestDirectory(t), "never-born")
	if err := stage.createFixedFile(t.Context(), filename, []byte("payload"), 0644, 0); !errors.Is(err, ErrBinding) {
		t.Fatal("fixed write without phase accepted", err)
	}
	if _, err := os.Lstat(filename); !os.IsNotExist(err) {
		t.Fatal("pre-intent write created file", err)
	}
}

type cancellationAtCreationRecord struct {
	context.Context
	cancel   context.CancelFunc
	filename string
}

func (c *cancellationAtCreationRecord) Err() error {
	if _, err := os.Lstat(c.filename); err == nil {
		c.cancel()
	}
	return c.Context.Err()
}

func TestInstallationNativeFixedCreationCancellationRetainsEmptyRecordedLeaf(t *testing.T) {
	stage := fixedCreationStage(t)
	filename := filepath.Join(nativeRequestDirectory(t), "fixed")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	recordPath := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "creations", digestHex([]byte(filename))+".json")
	err := stage.createFixedFile(&cancellationAtCreationRecord{Context: ctx, cancel: cancel, filename: recordPath}, filename, []byte("must not be written\n"), 0644, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	info, err := os.Lstat(filename)
	if err != nil || !rootOwnedFile(info) || info.Size() != 0 || info.Mode().Perm() != 0600 {
		t.Fatal("cancelled write changed or removed its empty leaf", err)
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatal("owned birth record lost", err)
	}
	if _, err := os.Stat(filepath.Join(stage.lease.path, "selection.json")); !os.IsNotExist(err) {
		t.Fatal("cancelled fixed creation selected")
	}
}

func TestInstallationNativeFixedDirectoryBirthModeAndNoAdoption(t *testing.T) {
	stage := fixedCreationStage(t)
	directory := filepath.Join(nativeRequestDirectory(t), "root")
	if err := stage.birthFixedDirectory(t.Context(), directory, 0700, 0); err != nil {
		t.Fatal(err)
	}
	if err := stage.birthFixedDirectory(t.Context(), filepath.Join(directory, "nested", "leaf"), 0555, 0); err != nil {
		t.Fatal(err)
	}
	if err := stage.changeFixedDirectoryMode(t.Context(), directory, 0555); err != nil {
		t.Fatal(err)
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	if err := stage.birthFixedDirectory(t.Context(), directory, 0700, 0); err == nil {
		t.Fatal("retained directory adopted")
	}
}

func TestInstallationNativeFixedDirectoryPromotionRetainsChildIdentity(t *testing.T) {
	stage := fixedCreationStage(t)
	directory := filepath.Join(nativeRequestDirectory(t), "worker-root")
	if err := stage.birthFixedDirectory(t.Context(), directory, 0700, 0); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "worker")
	if err := stage.createFixedFile(t.Context(), filename, []byte("signed worker fixture\n"), 0555, 0); err != nil {
		t.Fatal(err)
	}
	original, err := os.Lstat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.changeFixedDirectoryMode(t.Context(), directory, 0555); err != nil {
		t.Fatal(err)
	}
	if err := stage.observe(); err != nil {
		t.Fatal("owned parent promotion invalidated its unchanged child", err)
	}
	current, err := os.Lstat(filename)
	if err != nil || !os.SameFile(original, current) {
		t.Fatal("promotion replaced the child", err)
	}
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("signed worker fixture\n"), 0555); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(stage.observe(), ErrBinding) {
		t.Fatal("promotion adopted a same-byte replacement child")
	}
}

func TestInstallationNativeFixedDirectoryBirthIsRecordedBeforeAccess(t *testing.T) {
	stage := fixedCreationStage(t)
	directory := filepath.Join(nativeRequestDirectory(t), "worker-root")
	recordPath := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "directory-creations", digestHex([]byte(directory))+".json")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err := stage.birthFixedDirectory(&cancellationAtCreationRecord{Context: ctx, cancel: cancel, filename: recordPath}, directory, 0710, 65534)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("access proceeded after recorded birth cancelled its original caller", err)
	}
	info, err := os.Lstat(directory)
	if err != nil || !privateJournalDirectory(info) {
		t.Fatal("cancelled directory birth changed access or removed its residue", err)
	}
	body, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal("durable directory birth missing", err)
	}
	var record struct {
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
	native := info.Sys().(*syscall.Stat_t)
	if err := json.Unmarshal(body, &record); err != nil || record.Schema != "ardents-endpoint-directory-creation-v1" || record.GenerationDigest != stage.selected.GenerationDigest || record.Path != directory || record.Device != uint64(native.Dev) || record.Inode != native.Ino || record.PreviousMode != 0700 || record.PreviousGID != 0 || record.Mode != 0710 || record.GID != 65534 {
		t.Fatal("directory birth does not identify its original inode and intended access", err)
	}
}

func TestInstallationNativeFixedDirectoryPromotionRecordsBeforeChange(t *testing.T) {
	stage := fixedCreationStage(t)
	directory := filepath.Join(nativeRequestDirectory(t), "worker-root")
	if err := stage.birthFixedDirectory(t.Context(), directory, 0700, 0); err != nil {
		t.Fatal(err)
	}
	original, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "directory-creations", digestHex([]byte(directory))+"-access.json")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err = stage.changeFixedDirectoryMode(&cancellationAtCreationRecord{Context: ctx, cancel: cancel, filename: recordPath}, directory, 0555)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("promotion proceeded after recorded access cancelled its original caller", err)
	}
	current, err := os.Lstat(directory)
	if err != nil || !sameObservedDirectory(original, current) {
		t.Fatal("cancelled promotion changed the original directory", err)
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatal("promotion intent lost", err)
	}
	if err := stage.changeFixedDirectoryMode(t.Context(), directory, 0555); err == nil {
		t.Fatal("initial operation silently adopted retained promotion intent")
	}
}

// Filesystem mechanism fixture only: no successful manager, Release pair or
// predecessor join is supplied. It cannot establish upgrade acceptance.
func replacementRecordFixture(t *testing.T) (*installationTransaction, string, fixedFileObservation) {
	t.Helper()
	reader, request, previous, files, selected := successorStagingFixture(t)
	stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stage.close() })
	phase, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: selected.GenerationDigest, BindingDigest: selected.BindingDigest, Phase: "replacing-fixed-resources"})
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.journal.Write(t.Context(), journal.Transitions, "0003.json", phase); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(nativeRequestDirectory(t), "original-fixed")
	previousBytes := []byte("complete predecessor resource\n")
	if err := os.WriteFile(filename, previousBytes, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filename, 0644); err != nil {
		t.Fatal(err)
	}
	parent, err := os.Lstat(filepath.Dir(filename))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filename)
	if err != nil {
		t.Fatal(err)
	}
	return stage, filename, fixedFileObservation{parent: parent, file: fileObservation{identity: info, body: previousBytes, mode: 0644, gid: 0}}
}

func TestInstallationNativeReplacementRecordRetainsOriginalAndExactDigests(t *testing.T) {
	stage, filename, previous := replacementRecordFixture(t)
	candidate := []byte("complete candidate resource\n")
	if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, candidate); err != nil {
		t.Fatal(err)
	}
	name := digestHex([]byte(filename)) + ".json"
	first := fileObservation{body: stage.journal.Bytes(journal.Replacements, name), mode: 0600}
	var statErr error
	first.identity, statErr = os.Lstat(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "replacements", name))
	if statErr != nil {
		t.Fatal(statErr)
	}
	var actual struct {
		Schema, GenerationDigest, Path  string
		Device, Inode                   uint64
		Mode, GID                       uint32
		PreviousDigest, CandidateDigest string
	}
	// Inspect independently named persisted fields rather than the production
	// decoder's struct tags; JSON field names with underscores are explicit here.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(first.body, &fields); err != nil || len(fields) != 9 {
		t.Fatal("record inventory differs", err)
	}
	values := map[string]any{"schema": &actual.Schema, "generation_digest": &actual.GenerationDigest, "path": &actual.Path, "device": &actual.Device, "inode": &actual.Inode, "mode": &actual.Mode, "gid": &actual.GID, "previous_digest": &actual.PreviousDigest, "candidate_digest": &actual.CandidateDigest}
	for key, dest := range values {
		if err := json.Unmarshal(fields[key], dest); err != nil {
			t.Fatal(key, err)
		}
	}
	native := previous.file.identity.Sys().(*syscall.Stat_t)
	if actual.Schema != "ardents-endpoint-installation-replacement-v1" || actual.GenerationDigest != stage.selected.GenerationDigest || actual.Path != filename || actual.Device != uint64(native.Dev) || actual.Inode != native.Ino || actual.Mode != 0644 || actual.GID != 0 || actual.PreviousDigest != digestHex(previous.file.body) || actual.CandidateDigest != digestHex(candidate) {
		t.Fatal("original inode or complete byte binding differs")
	}
	if err := observeFixedFile(filename, previous); err != nil {
		t.Fatal("recording changed the resource", err)
	}
	if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, candidate); err != nil {
		t.Fatal("exact retry refused", err)
	}
	retried := fileObservation{body: stage.journal.Bytes(journal.Replacements, name), mode: 0600}
	retried.identity, statErr = os.Lstat(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "replacements", name))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if !os.SameFile(first.identity, retried.identity) || !bytes.Equal(first.body, retried.body) {
		t.Fatal("retry replaced the original record")
	}
	if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, []byte("foreign candidate")); !errors.Is(err, ErrBinding) {
		t.Fatal("record rebound to different candidate", err)
	}
	if err := observeFixedFile(filename, previous); err != nil {
		t.Fatal("candidate conflict changed the resource", err)
	}
}

func TestInstallationNativeReplacementSeparatesObservedAndCompletePreimage(t *testing.T) {
	for _, image := range []string{"empty", "old-prefix", "candidate-prefix", "candidate"} {
		t.Run(image, func(t *testing.T) {
			t.Parallel()
			stage, filename, original := replacementRecordFixture(t)
			candidate := []byte("complete candidate resource\n")
			if err := stage.recordFixedReplacement(t.Context(), filename, original, original.file.body, candidate); err != nil {
				t.Fatal(err)
			}
			body := candidate
			switch image {
			case "empty":
				body = nil
			case "old-prefix":
				body = original.file.body[:10]
			case "candidate-prefix":
				body = candidate[:10]
			}
			if err := os.WriteFile(filename, body, 0644); err != nil {
				t.Fatal(err)
			}
			reader := &installedFiles{files: make(map[string]fileObservation), directories: make(map[string]os.FileInfo)}
			if _, err := reader.readObserved(t.Context(), filename, 64<<20, 0644, 0, true); err != nil {
				t.Fatal(err)
			}
			current := fixedFileObservation{parent: original.parent, file: reader.files[filename]}
			if !os.SameFile(current.file.identity, original.file.identity) {
				t.Fatal("original inode lost")
			}
			name := digestHex([]byte(filename)) + ".json"
			record := stage.journal.Bytes(journal.Replacements, name)
			if err := stage.recordFixedReplacement(t.Context(), filename, current, original.file.body, candidate); err != nil {
				t.Fatal("current observation laundered complete preimage", err)
			}
			written, err := stage.replaceRecordedFixedFile(t.Context(), filename, current, original.file.body, candidate)
			if err != nil || !os.SameFile(written.identity, original.file.identity) || !bytes.Equal(written.body, candidate) || !bytes.Equal(record, stage.journal.Bytes(journal.Replacements, name)) {
				t.Fatal("same-inode repair changed original provenance", err)
			}
			if image == "candidate" && !sameObservedFile(current.file.identity, written.identity) {
				t.Fatal("complete candidate was rewritten instead of resynchronized")
			}
		})
	}
}

func TestInstallationNativeReplacementRecordRefusesChangedPreimage(t *testing.T) {
	for _, change := range []string{"inode", "bytes", "mode", "link", "phase"} {
		t.Run(change, func(t *testing.T) {
			stage, filename, previous := replacementRecordFixture(t)
			switch change {
			case "inode":
				if err := os.Rename(filename, filename+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, previous.file.body, 0644); err != nil {
					t.Fatal(err)
				}
			case "bytes":
				if err := os.WriteFile(filename, []byte("foreign bytes"), 0644); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if err := os.Chmod(filename, 0600); err != nil {
					t.Fatal(err)
				}
			case "link":
				if err := os.Link(filename, filename+"-link"); err != nil {
					t.Fatal(err)
				}
			case "phase":
				if err := os.WriteFile(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "0003.json"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, []byte("candidate")); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign preimage accepted", err)
			}
			if stage.journal.HasCollection(journal.Replacements) {
				t.Fatal("refusal created a replacement record directory")
			}
		})
	}
}

func TestInstallationNativeReplacementRecordCancellationKeepsBothPreimageAndRecord(t *testing.T) {
	stage, filename, previous := replacementRecordFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	name := digestHex([]byte(filename)) + ".json"
	recordPath := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "replacements", name)
	err := stage.recordFixedReplacement(&cancellationAtCreationRecord{Context: ctx, cancel: cancel, filename: recordPath}, filename, previous, previous.file.body, []byte("candidate"))
	if !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	if err := observeFixedFile(filename, previous); err != nil {
		t.Fatal("cancelled recording changed preimage", err)
	}
	if !stage.journal.HasCollection(journal.Replacements) || stage.journal.Bytes(journal.Replacements, name) == nil {
		t.Fatal("cancelled durable record lost custody")
	}
	if err := stage.journal.Observe(); !errors.Is(err, context.Canceled) {
		t.Fatal("written cancellation was renewed", err)
	}
	actual, readErr := os.ReadFile(recordPath)
	if readErr != nil || !bytes.Equal(actual, stage.journal.Bytes(journal.Replacements, name)) {
		t.Fatal("cancelled original record changed", readErr)
	}
}

// These tests exercise filesystem provenance only. They supply no successful
// manager, fresh pair, physical predecessor or installed-start authority.
func TestInstallationNativeReplacementRepairsOnlyRecordedOriginalPrefixes(t *testing.T) {
	for _, prefix := range []string{"empty", "old-prefix", "new-prefix", "old-complete", "new-complete"} {
		t.Run(prefix, func(t *testing.T) {
			stage, filename, previous := replacementRecordFixture(t)
			candidate := []byte("new resource is shorter\n")
			if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, candidate); err != nil {
				t.Fatal(err)
			}
			var torn []byte
			switch prefix {
			case "old-prefix":
				torn = previous.file.body[:9]
			case "new-prefix":
				torn = candidate[:9]
			case "old-complete":
				torn = previous.file.body
			case "new-complete":
				torn = candidate
			}
			if err := os.WriteFile(filename, torn, previous.file.mode); err != nil {
				t.Fatal(err)
			}
			written, err := stage.replaceRecordedFixedFile(t.Context(), filename, previous, previous.file.body, candidate)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(previous.file.identity, written.identity) || !bytes.Equal(written.body, candidate) || written.mode != previous.file.mode || written.gid != previous.file.gid {
				t.Fatal("replacement changed its original ownership")
			}
			if err := observeFixedFile(filename, fixedFileObservation{parent: previous.parent, file: written}); err != nil {
				t.Fatal("candidate differs", err)
			}
			// Use the immutable original preimage and record again, rather than
			// binding a new record to the now-visible candidate.
			if _, err := stage.replaceRecordedFixedFile(t.Context(), filename, previous, previous.file.body, candidate); err != nil {
				t.Fatal("same-record retry refused", err)
			}
		})
	}
}

func TestInstallationNativeReplacementRefusesForeignImageAndOwnership(t *testing.T) {
	for _, change := range []string{"bytes", "suffix", "mixed", "inode", "mode", "link", "record", "candidate", "no-record"} {
		t.Run(change, func(t *testing.T) {
			stage, filename, previous := replacementRecordFixture(t)
			candidate := []byte("new complete resource\n")
			if change != "no-record" {
				if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, candidate); err != nil {
					t.Fatal(err)
				}
			}
			switch change {
			case "bytes":
				if err := os.WriteFile(filename, []byte("foreign image"), previous.file.mode); err != nil {
					t.Fatal(err)
				}
			case "suffix":
				if err := os.WriteFile(filename, append(bytes.Clone(candidate), '!'), previous.file.mode); err != nil {
					t.Fatal(err)
				}
			case "mixed":
				if err := os.WriteFile(filename, append(bytes.Clone(candidate[:5]), previous.file.body[5:]...), previous.file.mode); err != nil {
					t.Fatal(err)
				}
			case "inode":
				if err := os.Rename(filename, filename+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, previous.file.body, previous.file.mode); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if err := os.Chmod(filename, 0600); err != nil {
					t.Fatal(err)
				}
			case "link":
				if err := os.Link(filename, filename+"-link"); err != nil {
					t.Fatal(err)
				}
			case "record":
				name := digestHex([]byte(filename)) + ".json"
				if err := os.WriteFile(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "replacements", name), []byte("foreign record"), 0600); err != nil {
					t.Fatal(err)
				}
			case "candidate":
				candidate = []byte("foreign candidate")
			}
			before, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			beforeInfo, err := os.Lstat(filename)
			if err != nil {
				t.Fatal(err)
			}
			written, err := stage.replaceRecordedFixedFile(t.Context(), filename, previous, previous.file.body, candidate)
			if err == nil || written.identity != nil {
				t.Fatal("foreign replacement accepted", err)
			}
			after, readErr := os.ReadFile(filename)
			afterInfo, statErr := os.Lstat(filename)
			unchanged := readErr == nil && statErr == nil && os.SameFile(beforeInfo, afterInfo) && beforeInfo.Mode() == afterInfo.Mode() && beforeInfo.Size() == afterInfo.Size() && beforeInfo.ModTime().Equal(afterInfo.ModTime())
			if unchanged {
				originalNative := beforeInfo.Sys().(*syscall.Stat_t)
				finalNative := afterInfo.Sys().(*syscall.Stat_t)
				unchanged = originalNative.Uid == finalNative.Uid && originalNative.Gid == finalNative.Gid && originalNative.Nlink == finalNative.Nlink && originalNative.Ctim == finalNative.Ctim
			}
			if !unchanged || !bytes.Equal(before, after) {
				t.Fatal("refusal modified foreign resource", readErr, statErr)
			}
		})
	}
}

func TestInstallationNativeReplacementCancellationDeniesMutation(t *testing.T) {
	stage, filename, previous := replacementRecordFixture(t)
	candidate := []byte("candidate")
	if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, candidate); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if written, err := stage.replaceRecordedFixedFile(ctx, filename, previous, previous.file.body, candidate); !errors.Is(err, context.Canceled) || written.identity != nil {
		t.Fatal("cancelled mutation accepted", err)
	}
	if err := observeFixedFile(filename, previous); err != nil {
		t.Fatal("cancelled replacement changed preimage", err)
	}
}
