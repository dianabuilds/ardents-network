//go:build installation_native

package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

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
	return stage, filename, fixedFileObservation{parent: parent, file: stagedFile{identity: info, body: previousBytes, mode: 0644, gid: 0}}
}

func TestInstallationNativeReplacementRecordRetainsOriginalAndExactDigests(t *testing.T) {
	stage, filename, previous := replacementRecordFixture(t)
	candidate := []byte("complete candidate resource\n")
	if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, candidate); err != nil {
		t.Fatal(err)
	}
	name := digestHex([]byte(filename)) + ".json"
	first := stagedFile{body: stage.journal.Bytes(journal.Replacements, name), mode: 0600}
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
	retried := stagedFile{body: stage.journal.Bytes(journal.Replacements, name), mode: 0600}
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
			reader := &installedFiles{files: make(map[string]stagedFile), directories: make(map[string]os.FileInfo)}
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
			if image == "candidate" && !sameReadIdentity(current.file.identity, written.identity) {
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
