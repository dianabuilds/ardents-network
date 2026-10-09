//go:build installation_native

package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Actual filesystem admission only: no successful Release, manager, process,
// start or public recovery substitute is supplied by this fixture.
func writingRecoveryFixture(t *testing.T, count int, private bool) (*successorRecoveryNative, []string) {
	t.Helper()
	r, _ := stagedRecoveryFixture(t)
	names := generation.Names()
	sort.Strings(names)
	directory := filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest)
	if err := os.Remove(filepath.Join(r.journal, "0002.json")); err != nil {
		t.Fatal(err)
	}
	for _, name := range names[count:] {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(r.journal, generationFileRecordName(name))); err != nil {
			t.Fatal(err)
		}
	}
	if private && count > 0 {
		filename := filepath.Join(directory, names[count-1])
		body, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filename, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(filename, 0, 0); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, body[:len(body)/2], 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(directory, 0, 0); err != nil {
		t.Fatal(err)
	}
	// The fixture constructs a fresh reader observation of the interrupted
	// image, rather than refreshing an admitted recovery's retained custody.
	delete(r.reader.directories, directory)
	r.writing = &generationWriteRecovery{births: make(map[string]generationFileBirth)}
	return r, names
}

func TestInstallationNativeWritingRecoveryAdmitsClosedBirthPrefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		count   int
		private bool
	}{
		{"empty", 0, false}, {"last-torn", 1, true}, {"complete-prefix", 7, false}, {"all-complete-before-seal", 15, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // Each prefix retains independent roots, journal and descriptors.
			r, _ := writingRecoveryFixture(t, tc.count, tc.private)
			records, images, err := r.readWritingJournal(t.Context())
			if err != nil || len(images) != tc.count || len(records) != tc.count+3 {
				t.Fatal("closed prefix refused", len(records), len(images), err)
			}
			parent := filepath.Join(r.reader.lease.path, "generations")
			directory := filepath.Join(parent, r.intent.Candidate.GenerationDigest)
			prefix, err := generation.OpenPrefix(t.Context(), parent, r.reader.directories[parent], r.reader.directories[directory], r.intent.Candidate.GenerationDigest, r.reader.gid, images)
			if prefix != nil {
				t.Cleanup(func() { _ = prefix.Close() })
			}
			if err != nil {
				t.Fatal(err)
			}
			retained, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records)
			if retained != nil {
				t.Cleanup(func() { _ = retained.Close() })
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := prefix.Observe(); err != nil {
				t.Fatal(err)
			}
			if err := retained.Observe(); err != nil {
				t.Fatal(err)
			}
			if r.checked.selected.GenerationDigest == "" {
				t.Fatal("fixture lost detached expected facts")
			}
			// Opening this physical prefix neither manufactures a creation Owner
			// nor completes phase2 or changes any artifact bytes.
			for name, image := range images {
				actual, err := os.ReadFile(filepath.Join(directory, name))
				if err != nil || !bytes.Equal(actual, image.Bytes) {
					t.Fatal("admission changed bytes", name, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(r.journal, "0002.json")); !os.IsNotExist(err) {
				t.Fatal("admission completed phase2", err)
			}
		})
	}
}

func TestInstallationNativeWritingRecoveryRefusesInvalidPrefix(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"birth-gap", "incomplete-before-later-birth", "foreign-inode", "unrecorded-leaf", "promoted-partial-directory", "missing-marker", "wrong-phase", "extra-phase"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel() // No manager effects or shared fixture state.
			r, names := writingRecoveryFixture(t, 3, false)
			directory := filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest)
			filename := filepath.Join(directory, names[0])
			switch mutation {
			case "birth-gap":
				if err := os.Remove(filepath.Join(r.journal, generationFileRecordName(names[1]))); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(directory, names[1])); err != nil {
					t.Fatal(err)
				}
			case "incomplete-before-later-birth":
				if err := os.Chmod(filename, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(filename, 0, 0); err != nil {
					t.Fatal(err)
				}
				if err := os.Truncate(filename, 0); err != nil {
					t.Fatal(err)
				}
			case "foreign-inode":
				body, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filename, filepath.Join(filepath.Dir(directory), "original-leaf")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, body, 0555); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(filename, 0, int(r.reader.gid)); err != nil {
					t.Fatal(err)
				}
			case "unrecorded-leaf":
				if err := os.WriteFile(filepath.Join(directory, names[3]), []byte("residue"), 0600); err != nil {
					t.Fatal(err)
				}
			case "promoted-partial-directory":
				if err := os.Chown(directory, 0, int(r.reader.gid)); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(directory, 0750); err != nil {
					t.Fatal(err)
				}
			case "missing-marker":
				if err := os.Remove(filepath.Join(r.journal, "generation-files.json")); err != nil {
					t.Fatal(err)
				}
			case "wrong-phase":
				filename := filepath.Join(r.journal, "0001.json")
				body, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, bytes.Replace(body, []byte("writing-generation"), []byte("generation-staged"), 1), 0600); err != nil {
					t.Fatal(err)
				}
			case "extra-phase":
				if err := os.WriteFile(filepath.Join(r.journal, "0003.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := r.readWritingJournal(t.Context()); err == nil {
				t.Fatal("invalid prefix accepted", mutation)
			}
		})
	}
}

// Physical/schema admission only; fresh authorization and continuation remain
// exercised by the real command, never manufactured by this fixture.
func TestInstallationNativeWritingRecoveryRetainsFirstFailurePrefixes(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"failed", "copied", "retired", "conflict", "wrong-phase", "foreign-generation", "missing-error"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			r, _ := writingRecoveryFixture(t, 3, true)
			generationDigest, phase, originalError := r.intent.Candidate.GenerationDigest, "generation-write-failed", "short write"
			if state == "foreign-generation" {
				generationDigest = strings.Repeat("e", 64)
			}
			if state == "wrong-phase" {
				phase = "successor-transition-failed"
			}
			if state == "missing-error" {
				originalError = ""
			}
			body := []byte(fmt.Sprintf(`{"schema":"ardents-endpoint-installation-transition-v1","generation_digest":%q,"binding_digest":%q,"phase":%q,"original_error":%q}`,
				generationDigest, r.intent.Candidate.BindingDigest, phase, originalError))
			body = append(body, '\n')
			if state != "retired" {
				if err := os.WriteFile(filepath.Join(r.journal, "0002.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if state == "copied" || state == "retired" || state == "conflict" {
				copy := bytes.Clone(body)
				if state == "conflict" {
					copy = bytes.Replace(copy, []byte("short write"), []byte("different error"), 1)
				}
				if err := os.WriteFile(filepath.Join(r.journal, "original-transition-failure.json"), copy, 0600); err != nil {
					t.Fatal(err)
				}
			}
			records, _, err := r.readWritingJournal(t.Context())
			valid := state == "failed" || state == "copied" || state == "retired"
			if !valid {
				if err == nil {
					t.Fatal("unbound failure accepted", state)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if r.writing.failedPhase != (state != "retired") {
				t.Fatal("wrong failed-slot classification")
			}
			name := "0002.json"
			if state == "retired" {
				name = "original-transition-failure.json"
			}
			if !bytes.Equal(records[name].Bytes, body) {
				t.Fatal("first error bytes changed")
			}
			if state == "failed" {
				retained, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = retained.Close() })
				stage := &installationTransaction{journal: retained, selected: r.intent.Candidate}
				if err := stage.record(t.Context(), "generation-write-failed", errors.New("different later cleanup error")); err != nil {
					t.Fatal(err)
				}
				actual, err := os.ReadFile(filepath.Join(r.journal, "original-transition-failure.json"))
				if err != nil || !bytes.Equal(actual, body) {
					t.Fatal("cleanup relabeled or changed first failed phase", err)
				}
			}
			if state == "retired" {
				retained, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = retained.Close() })
				stage := &installationTransaction{journal: retained, selected: r.intent.Candidate}
				_ = stage.record(t.Context(), "generation-write-failed", errors.New("later failed repair"))
				actual, err := os.ReadFile(filepath.Join(r.journal, name))
				if err != nil || !bytes.Equal(actual, body) {
					t.Fatal("later failure erased first error", err)
				}
				if _, err := os.Lstat(filepath.Join(r.journal, "0002.json")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("later failure occupied completed-phase slot", err)
				}
			}
		})
	}
}

// Filesystem provenance only. One actual generation supplies the journal's
// original candidate births. Previous selection is a detached expected image,
// not an inspected predecessor or a successful installed recovery substitute.
// Successor production staging and original predecessor custody have separate
// tests; these reader checks supply no Release, account, process, manager or ACK.
func stagedRecoveryFixture(t *testing.T) (*successorRecoveryNative, Request) {
	t.Helper()
	lease, request, files, selected := nativeStagingFixture(t)
	request.declared.ManifestSHA256 = ""
	requestBody, err := canonicalJSON(*request.declared)
	if err != nil {
		t.Fatal(err)
	}
	request, err = DecodeRequest(t.Context(), requestBody, false)
	if err != nil {
		t.Fatal(err)
	}
	files["request.json"] = requestBody
	var binding generationBinding
	if err := json.Unmarshal(files["binding.json"], &binding); err != nil {
		t.Fatal(err)
	}
	binding.Files["request.json"] = digestHex(requestBody)
	files["binding.json"], err = canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	selected.BindingDigest = digestHex(files["binding.json"])
	stage, err := stageInitialGeneration(t.Context(), lease, request, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	// Establish the actual read access needed by independent reader/selection
	// probes, without unrelated initial fixed resources or archival phases.
	parent := filepath.Join(lease.path, "generations")
	for _, directory := range []string{parent, lease.path} {
		if err := os.Chown(directory, 0, 65534); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(directory, 0750); err != nil {
			t.Fatal(err)
		}
	}
	lease.identity, err = lease.root.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	previous := generationSelection{Schema: selected.Schema, GenerationDigest: digestHex([]byte("previous descriptor image")), BindingDigest: digestHex([]byte("previous binding image"))}
	intent := successorTransitionIntent{Schema: "ardents-endpoint-installation-successor-v1", Previous: previous, Candidate: selected, CandidateBinding: binding, Request: *request.declared}
	intentBody, err := canonicalJSON(intent)
	if err != nil {
		t.Fatal(err)
	}
	// Fixture construction finishes before the independent reader opens. It
	// neither refreshes an admitted observation nor retains creator descriptors.
	if err := lease.root.Remove("transition.json"); err != nil {
		t.Fatal(err)
	}
	intentFile, err := writeStagedFile(t.Context(), lease.root, "transition.json", intentBody, 0600, 0)
	if err != nil {
		t.Fatal(err)
	}
	selectionBody, err := canonicalJSON(previous)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := writeStagedFile(t.Context(), lease.root, "selection.json", selectionBody, 0640, 65534)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncStagingRoot(lease.root); err != nil {
		t.Fatal(err)
	}
	reader := &installedRoot{lease: lease, installedFiles: &installedFiles{directory: lease.path, gid: 65534, files: map[string]fileObservation{
		filepath.Join(lease.path, "transition.json"): intentFile,
		filepath.Join(lease.path, "selection.json"):  selection,
	}, directories: map[string]os.FileInfo{lease.path: lease.identity}, mutableDirectories: map[string]os.FileInfo{}}}
	for _, directory := range []string{parent, filepath.Join(parent, selected.GenerationDigest)} {
		if err := reader.pinGenerationDirectory(directory); err != nil {
			t.Fatal(err)
		}
	}
	r := &successorRecoveryNative{reader: reader, journal: filepath.Join(lease.path, "journals", selected.GenerationDigest), intentBody: intentBody, intent: intent}
	r.checked = inspectedGeneration{files: files, selected: selected, binding: binding}
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeSuccessorRecovery(r) })
	return r, request
}

func TestInstallationNativeStagedRecoveryRetainsSealedReadCustody(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	r, _ := stagedRecoveryFixture(t)
	records, err := r.readStagedJournal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(r.reader.lease.path, "generations")
	directory := filepath.Join(parent, r.intent.Candidate.GenerationDigest)
	snapshot, err := generation.OpenSnapshot(t.Context(), parent, r.reader.directories[parent], r.reader.directories[directory], r.intent.Candidate.GenerationDigest, r.reader.gid)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records)
	if err != nil {
		_ = snapshot.Close()
		t.Fatal(err)
	}
	stage := &installationTransaction{lease: r.reader.lease, selected: r.intent.Candidate, sealed: snapshot, sealedIdentity: r.reader.directories[directory], journal: retained, intent: r.reader.files[filepath.Join(r.reader.lease.path, "transition.json")]}
	t.Cleanup(func() { _ = stage.close() })
	if stage.generation != nil {
		t.Fatal("recovery fabricated exclusive creation owner")
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	resources, err := successorFixedResources(stage)
	if err != nil || len(resources) != 9 {
		t.Fatal("sealed read owner did not supply actual fixed bytes", len(resources), err)
	}
	program := filepath.Join(directory, "ardents-linux-amd64")
	if err := os.Rename(program, filepath.Join(filepath.Dir(directory), "retained-original-program")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, snapshot.Bytes("ardents-linux-amd64"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(program, 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := stage.observe(); err == nil {
		t.Fatal("sealed recovery accepted same-byte foreign program inode")
	}
}

func TestInstallationNativeStagedRecoveryRefusesWrongPhaseAndBirth(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, mutation := range []string{"binding", "phase", "birth", "extra-record", "failure", "third-binding", "third-phase", "replacement-group"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel() // Every refusal owns its original generation and journal.
			r, _ := stagedRecoveryFixture(t)
			filename := filepath.Join(r.journal, "0002.json")
			body, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "binding":
				body = bytes.Replace(body, []byte(r.intent.Candidate.BindingDigest), []byte(r.intent.Previous.BindingDigest), 1)
			case "phase":
				body = bytes.Replace(body, []byte("generation-staged"), []byte("generation-write-failed"), 1)
			case "birth":
				filename = filepath.Join(r.journal, "generation-directory.json")
				body = []byte(`{"schema":"ardents-endpoint-generation-directory-v1","generation_digest":"` + r.intent.Candidate.GenerationDigest + `","device":1,"inode":1}`)
			case "extra-record":
				filename = filepath.Join(r.journal, "0004.json")
			case "failure":
				filename = filepath.Join(r.journal, "original-transition-failure.json")
			case "third-binding", "third-phase":
				filename = filepath.Join(r.journal, "0003.json")
				record := generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, BindingDigest: r.intent.Candidate.BindingDigest, Phase: "replacing-fixed-resources"}
				if mutation == "third-binding" {
					record.BindingDigest = r.intent.Previous.BindingDigest
				} else {
					record.Phase = "fixed-resources-replaced"
				}
				body, err = canonicalJSON(record)
				if err != nil {
					t.Fatal(err)
				}
			case "replacement-group":
				if err := os.Mkdir(filepath.Join(r.journal, "replacements"), 0700); err != nil {
					t.Fatal(err)
				}

			}
			if err := os.WriteFile(filename, body, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := r.readStagedJournal(t.Context()); err == nil {
				t.Fatal("invalid staged provenance accepted", mutation)
			}
		})
	}
}

func TestInstallationNativeStagedRecoveryCopyClosesOnOriginalCancellation(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	r, request := stagedRecoveryFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	r.pending = &successorPreparation{ctx: ctx, request: request, inspection: r.reader, recovery: r}
	owner := &SuccessorRecovery{state: &successorRecoveryOperation{ctx: ctx, cancel: cancel, native: r}}
	copy := *owner
	writer := r.reader.lease.writer
	cancel()
	if result, err := owner.Complete(nil, enrollment.Candidate{}, release.Inputs{}); result != (ProvisionResult{}) || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled recovery admitted effects", result, err)
	}
	if _, err := writer.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("cancelled recovery retained writer", err)
	}
	if _, err := copy.Complete(nil, enrollment.Candidate{}, release.Inputs{}); !errors.Is(err, ErrInput) || !errors.Is(err, context.Canceled) {
		t.Fatal("copy renewed or erased original outcome", err)
	}
}

// A phase record alone grants no mutation or Release authority. This verifies
// independent physical reopen of the exact flat prefix, without a manager.
func TestInstallationNativeStagedRecoveryRetainsReplacementIntention(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	r, _ := stagedRecoveryFixture(t)
	body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, BindingDigest: r.intent.Candidate.BindingDigest, Phase: "replacing-fixed-resources"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.journal, "0003.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	records, err := r.readStagedJournal(t.Context())
	if err != nil || len(records) != 20 {
		t.Fatal("exact flat replacement intention refused", len(records), err)
	}
	retained, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = retained.Close() })
	if err := retained.Resync(t.Context(), journal.Transitions, "0003.json"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retained.Bytes(journal.Transitions, "0003.json"), body) || retained.HasCollection(journal.Replacements) {
		t.Fatal("reopen changed original intention or fabricated a group")
	}
}

// Logical fixed paths have real temporary inode observations in this mechanism
// fixture. No /etc or /usr resource, Release proof, manager or runtime is changed.
// Public opening observes real fixed paths; this fixture maps only filesystem
// provenance and byte-order policy and proves no actual-manager acceptance.
func stagedReplacementPrefixFixture(t *testing.T, count int) (*successorRecoveryNative, []string, map[string]string) {
	t.Helper()
	r, _ := stagedRecoveryFixture(t)
	phase, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, BindingDigest: r.intent.Candidate.BindingDigest, Phase: "replacing-fixed-resources"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.journal, "0003.json"), phase, 0600); err != nil {
		t.Fatal(err)
	}
	group := filepath.Join(r.journal, "replacements")
	if err := os.Mkdir(group, 0700); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"/etc/ardents/text-worker-artifact.json",
		"/etc/systemd/system/ardents-endpoint.service",
		"/etc/systemd/system/ardents-text-publisher.socket",
		"/etc/systemd/system/ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-reader@.service",
		"/usr/lib/ardents/text-worker-root/ardents-text",
		"/usr/lib/tmpfiles.d/ardents-text.conf",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules",
	}
	resources, err := recoveryFixedBytes(r.checked)
	if err != nil || len(resources) != len(paths)+1 {
		t.Fatal("fixed inventory", err)
	}
	r.previous = r.checked
	r.previous.files = make(map[string][]byte)
	for name, body := range r.checked.files {
		r.previous.files[name] = append([]byte("predecessor "), body...)
	}
	preimages, err := recoveryFixedBytes(r.previous)
	if err != nil {
		t.Fatal(err)
	}
	directory := nativeRequestDirectory(t)
	physical := make(map[string]string)
	for index, filename := range paths {
		leaf := filepath.Join(directory, fmt.Sprintf("fixed-%d", index))
		body := preimages[filename]
		if err := os.WriteFile(leaf, body, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(leaf, 0644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(leaf)
		if err != nil {
			t.Fatal(err)
		}
		physical[filename] = leaf
		r.reader.files[filename] = fileObservation{identity: info, body: body, mode: 0644, gid: 0}
		if index < count {
			native := info.Sys().(*syscall.Stat_t)
			// Independent explicit persisted grammar, including its canonical order.
			record := fmt.Sprintf("{\"schema\":\"ardents-endpoint-installation-replacement-v1\",\"generation_digest\":%q,\"path\":%q,\"device\":%d,\"inode\":%d,\"mode\":420,\"gid\":0,\"previous_digest\":%q,\"candidate_digest\":%q}\n", r.intent.Candidate.GenerationDigest, filename, native.Dev, native.Ino, digestHex(body), digestHex(resources[filename]))
			if err := os.WriteFile(filepath.Join(group, digestHex([]byte(filename))+".json"), []byte(record), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return r, paths, physical
}

func TestInstallationNativeStagedReplacementPrefixesRetainOriginalGroup(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, count := range []int{0, 1, 5, 9} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			t.Parallel() // Each prefix has its own files, journal and retained group.
			r, paths, physical := stagedReplacementPrefixFixture(t, count)
			records, err := r.readStagedJournal(t.Context())
			if err != nil || r.stagedReplacements == nil || len(r.stagedReplacements.Records) != count {
				t.Fatal("original prefix refused", err)
			}
			owner, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records, *r.stagedReplacements)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			for _, filename := range paths[:count] {
				if err := owner.Resync(t.Context(), journal.Replacements, digestHex([]byte(filename))+".json"); err != nil {
					t.Fatal(err)
				}
			}
			if err := owner.ResyncReplacements(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := owner.Ensure(t.Context(), journal.Replacements); err != nil {
				t.Fatal("re-created original group", err)
			}
			for _, filename := range paths {
				body, err := os.ReadFile(physical[filename])
				info, statErr := os.Lstat(physical[filename])
				if err != nil || statErr != nil || !bytes.Equal(body, r.reader.files[filename].body) || !os.SameFile(info, r.reader.files[filename].identity) {
					t.Fatal("provenance read changed a fixed preimage", err, statErr)
				}
			}
		})
	}
}

func TestInstallationNativeStagedReplacementPrefixRefusesConflicts(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, mutation := range []string{"generation", "path", "inode", "previous", "candidate", "mode", "gap", "selection-record", "extra-record", "later-phase", "group-access"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel() // Each refusal owns an independent physical prefix.
			r, paths, _ := stagedReplacementPrefixFixture(t, 2)
			group := filepath.Join(r.journal, "replacements")
			filename := filepath.Join(group, digestHex([]byte(paths[0]))+".json")
			body, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "generation":
				body = bytes.Replace(body, []byte(r.intent.Candidate.GenerationDigest), []byte(r.intent.Previous.GenerationDigest), 1)
			case "path":
				body = bytes.Replace(body, []byte(paths[0]), []byte(paths[1]), 1)
			case "inode":
				body = bytes.Replace(body, []byte("\"inode\":"), []byte("\"inode\":1"), 1)
			case "previous":
				body = bytes.Replace(body, []byte(digestHex(r.reader.files[paths[0]].body)), []byte(digestHex([]byte("foreign preimage\n"))), 1)
			case "candidate":
				resources, err := recoveryFixedBytes(r.checked)
				if err != nil {
					t.Fatal(err)
				}
				body = bytes.Replace(body, []byte(digestHex(resources[paths[0]])), []byte(digestHex([]byte("foreign candidate\n"))), 1)
			case "mode":
				body = bytes.Replace(body, []byte("\"mode\":420"), []byte("\"mode\":384"), 1)
			case "gap":
				if err := os.Remove(filename); err != nil {
					t.Fatal(err)
				}
			case "selection-record":
				filename = filepath.Join(group, digestHex([]byte(filepath.Join(r.reader.lease.path, "selection.json")))+".json")
			case "extra-record":
				filename = filepath.Join(group, "foreign.json")
			case "later-phase":
				filename = filepath.Join(r.journal, "0004.json")
			case "group-access":
				if err := os.Chmod(group, 0750); err != nil {
					t.Fatal(err)
				}
			}
			if mutation != "gap" {
				if err := os.WriteFile(filename, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.readStagedJournal(t.Context()); err == nil {
				t.Fatal("invalid replacement prefix accepted", mutation)
			}
		})
	}
}

func observeFixtureFixedBytes(t *testing.T, r *successorRecoveryNative, logical, physical string, body []byte) {
	t.Helper()
	if err := os.WriteFile(physical, body, 0644); err != nil {
		t.Fatal(err)
	}
	reader := &installedFiles{files: make(map[string]fileObservation), directories: make(map[string]os.FileInfo)}
	if _, err := reader.readObserved(t.Context(), physical, 64<<20, 0644, 0, true); err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(r.reader.files[logical].identity, reader.files[physical].identity) {
		t.Fatal("fixture replaced the original inode")
	}
	r.reader.files[logical] = reader.files[physical]
}

func TestInstallationNativeStagedFixedMutationOrder(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for boundary := 0; boundary <= 9; boundary++ {
		for _, image := range []string{"empty", "old-prefix", "candidate-prefix", "candidate"} {
			if boundary == 9 && image != "candidate" || boundary != 9 && image != "empty" && boundary != 4 {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s", boundary, image), func(t *testing.T) {
				t.Parallel()
				r, paths, physical := stagedReplacementPrefixFixture(t, 9)
				after, err := recoveryFixedBytes(r.checked)
				if err != nil {
					t.Fatal(err)
				}
				for index, filename := range paths {
					body := r.reader.files[filename].body
					if index < boundary {
						body = after[filename]
					} else if index == boundary {
						switch image {
						case "empty":
							body = nil
						case "old-prefix":
							body = body[:len(body)/2]
						case "candidate-prefix":
							body = after[filename][:len(after[filename])/2]
						case "candidate":
							body = after[filename]
						}
					}
					observeFixtureFixedBytes(t, r, filename, physical[filename], body)
				}
				if _, err := r.readStagedJournal(t.Context()); err != nil {
					t.Fatal("complete original records refused", err)
				}
				if err := r.checkStagedFixedResources(); err != nil || !r.fixedMutation {
					t.Fatal("ordered same-inode mutation refused", err)
				}
			})
		}
	}
}

func TestInstallationNativeStagedFixedMutationRefusesAmbiguity(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, mutation := range []string{"gap-before-write", "second-torn", "candidate-after-old", "foreign-bytes", "suffix", "missing-record", "observed-digest"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			r, paths, physical := stagedReplacementPrefixFixture(t, 9)
			after, err := recoveryFixedBytes(r.checked)
			if err != nil {
				t.Fatal(err)
			}
			observeFixtureFixedBytes(t, r, paths[0], physical[paths[0]], nil)
			switch mutation {
			case "gap-before-write":
				observeFixtureFixedBytes(t, r, paths[0], physical[paths[0]], after[paths[0]])
				observeFixtureFixedBytes(t, r, paths[2], physical[paths[2]], after[paths[2]])
			case "second-torn":
				observeFixtureFixedBytes(t, r, paths[1], physical[paths[1]], nil)
			case "candidate-after-old":
				observeFixtureFixedBytes(t, r, paths[1], physical[paths[1]], after[paths[1]])
			case "foreign-bytes":
				observeFixtureFixedBytes(t, r, paths[0], physical[paths[0]], []byte("foreign"))
			case "suffix":
				observeFixtureFixedBytes(t, r, paths[0], physical[paths[0]], append(bytes.Clone(after[paths[0]]), '!'))
			case "missing-record":
				if err := os.Remove(filepath.Join(r.journal, "replacements", digestHex([]byte(paths[8]))+".json")); err != nil {
					t.Fatal(err)
				}
			case "observed-digest":
				filename := filepath.Join(r.journal, "replacements", digestHex([]byte(paths[0]))+".json")
				body, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				before, err := recoveryFixedBytes(r.previous)
				if err != nil {
					t.Fatal(err)
				}
				body = bytes.Replace(body, []byte(digestHex(before[paths[0]])), []byte(digestHex(nil)), 1)
				if err := os.WriteFile(filename, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, journalErr := r.readStagedJournal(t.Context())
			if journalErr == nil && r.checkStagedFixedResources() == nil {
				t.Fatal("ambiguous mutation admitted", mutation)
			}
		})
	}
}

// Mechanism/policy only: real temporary original files and records, without a
// substituted successful manager, Release verifier or accepting runtime.
func fixedReplacementCompletionFixture(t *testing.T) (*successorRecoveryNative, []string, map[string]string) {
	t.Helper()
	r, paths, physical := stagedReplacementPrefixFixture(t, 9)
	resources, err := recoveryFixedBytes(r.checked)
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range paths {
		observeFixtureFixedBytes(t, r, filename, physical[filename], resources[filename])
	}
	body := fmt.Sprintf("{\"schema\":\"ardents-endpoint-installation-transition-v1\",\"generation_digest\":%q,\"binding_digest\":%q,\"phase\":\"fixed-resources-replaced\"}\n", r.intent.Candidate.GenerationDigest, r.intent.Candidate.BindingDigest)
	if err := os.WriteFile(filepath.Join(r.journal, "0004.json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return r, paths, physical
}

func makeFixtureFixedPreimagesUnchanged(t *testing.T, r *successorRecoveryNative, paths []string) {
	t.Helper()
	before, err := recoveryFixedBytes(r.previous)
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range paths {
		path := filepath.Join(r.journal, "replacements", digestHex([]byte(filename))+".json")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.Replace(body, []byte(digestHex(before[filename])), []byte(digestHex(r.reader.files[filename].body)), 1)
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r.previous.files = r.checked.files
}

func TestInstallationNativeStagedFixedCompletionRetainsOriginalRecord(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, unchanged := range []bool{false, true} {
		t.Run(fmt.Sprint(unchanged), func(t *testing.T) {
			t.Parallel()
			r, paths, _ := fixedReplacementCompletionFixture(t)
			if unchanged {
				makeFixtureFixedPreimagesUnchanged(t, r, paths)
			}
			records, err := r.readStagedJournal(t.Context())
			if err != nil || len(records) != 21 || r.checkStagedFixedResources() != nil || !r.fixedMutation {
				t.Fatal("completed fixed resources did not require continuing quiescence", err)
			}
			owner, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records, *r.stagedReplacements)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			stage := &installationTransaction{selected: r.intent.Candidate, journal: owner}
			before := records["0004.json"]
			for range 2 {
				if err := stage.recordFixedReplacementCompletion(t.Context()); err != nil {
					t.Fatal("original completion could not be resynchronized", err)
				}
			}
			after, err := os.Lstat(filepath.Join(r.journal, "0004.json"))
			if err != nil || !sameObservedFile(before.Identity, after) || !bytes.Equal(before.Bytes, owner.Bytes(journal.Transitions, "0004.json")) {
				t.Fatal("completion resync recreated or rewrote original record", err)
			}
			if _, err := os.Lstat(filepath.Join(r.journal, "0005.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("physical completion resync published selection", err)
			}
		})
	}
}

func TestInstallationNativeStagedFixedCompletionRefusesIncomplete(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, mutation := range []string{"last-torn", "all-old", "missing-record", "missing-record-unchanged", "missing-intention", "binding", "phase", "first-error", "later-phase"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			r, paths, physical := fixedReplacementCompletionFixture(t)
			filename := filepath.Join(r.journal, "0004.json")
			body, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "last-torn":
				observeFixtureFixedBytes(t, r, paths[8], physical[paths[8]], nil)
			case "all-old":
				before, err := recoveryFixedBytes(r.previous)
				if err != nil {
					t.Fatal(err)
				}
				for _, path := range paths {
					observeFixtureFixedBytes(t, r, path, physical[path], before[path])
				}
			case "missing-record", "missing-record-unchanged":
				if mutation == "missing-record-unchanged" {
					makeFixtureFixedPreimagesUnchanged(t, r, paths)
				}
				if err := os.Remove(filepath.Join(r.journal, "replacements", digestHex([]byte(paths[8]))+".json")); err != nil {
					t.Fatal(err)
				}
			case "missing-intention":
				if err := os.Remove(filepath.Join(r.journal, "0003.json")); err != nil {
					t.Fatal(err)
				}
			case "binding":
				body = bytes.Replace(body, []byte(r.intent.Candidate.BindingDigest), []byte(r.intent.Previous.BindingDigest), 1)
			case "phase":
				body = bytes.Replace(body, []byte("fixed-resources-replaced"), []byte("replacing-fixed-resources"), 1)
			case "first-error":
				body = bytes.Replace(body, []byte("}\n"), []byte(",\"original_error\":\"earlier refusal\"}\n"), 1)
			case "later-phase":
				filename = filepath.Join(r.journal, "0005.json")
			}
			if err := os.WriteFile(filename, body, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = r.readStagedJournal(t.Context())
			if err == nil && r.checkStagedFixedResources() == nil {
				t.Fatal("invalid fixed completion admitted", mutation)
			}
		})
	}
}

// Physical selection provenance; no successful manager/proof/runtime fixture.
func selectionPublishingFixture(t *testing.T, record bool) (*successorRecoveryNative, string) {
	t.Helper()
	r, _, _ := fixedReplacementCompletionFixture(t)
	phase := fmt.Sprintf("{\"schema\":\"ardents-endpoint-installation-transition-v1\",\"generation_digest\":%q,\"binding_digest\":%q,\"phase\":\"publishing-selection\"}\n", r.intent.Candidate.GenerationDigest, r.intent.Candidate.BindingDigest)
	if err := os.WriteFile(filepath.Join(r.journal, "0005.json"), []byte(phase), 0600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(r.reader.lease.path, "selection.json")
	if record {
		before, err := canonicalJSON(r.intent.Previous)
		if err != nil {
			t.Fatal(err)
		}
		after, err := canonicalJSON(r.intent.Candidate)
		if err != nil {
			t.Fatal(err)
		}
		native := r.reader.files[filename].identity.Sys().(*syscall.Stat_t)
		body := fmt.Sprintf("{\"schema\":\"ardents-endpoint-installation-replacement-v1\",\"generation_digest\":%q,\"path\":%q,\"device\":%d,\"inode\":%d,\"mode\":416,\"gid\":65534,\"previous_digest\":%q,\"candidate_digest\":%q}\n", r.intent.Candidate.GenerationDigest, filename, native.Dev, native.Ino, digestHex(before), digestHex(after))
		if err := os.WriteFile(filepath.Join(r.journal, "replacements", digestHex([]byte(filename))+".json"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return r, filename
}

func observeFixtureSelection(t *testing.T, r *successorRecoveryNative, filename string, body []byte) {
	t.Helper()
	original := r.reader.files[filename].identity
	if err := os.WriteFile(filename, body, 0640); err != nil {
		t.Fatal(err)
	}
	reader := &installedFiles{files: make(map[string]fileObservation), directories: map[string]os.FileInfo{r.reader.lease.path: r.reader.lease.identity}}
	if _, err := reader.readObserved(t.Context(), filename, 4<<10, 0640, 65534, true); err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(original, reader.files[filename].identity) {
		t.Fatal("fixture replaced selection inode")
	}
	r.reader.files[filename] = reader.files[filename]
}

func TestInstallationNativeStagedSelectionRepairsOriginalPrefix(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, image := range []string{"old-unrecorded", "old", "empty", "old-prefix", "candidate-prefix", "candidate"} {
		t.Run(image, func(t *testing.T) {
			t.Parallel()
			r, filename := selectionPublishingFixture(t, image != "old-unrecorded")
			before, err := canonicalJSON(r.intent.Previous)
			if err != nil {
				t.Fatal(err)
			}
			after, err := canonicalJSON(r.intent.Candidate)
			if err != nil {
				t.Fatal(err)
			}
			body := before
			switch image {
			case "empty":
				body = nil
			case "old-prefix":
				body = before[:len(before)/2]
			case "candidate-prefix":
				body = after[:len(after)/2]
			case "candidate":
				body = after
			}
			observeFixtureSelection(t, r, filename, body)
			records, err := r.readStagedJournal(t.Context())
			if err != nil || r.checkStagedFixedResources() != nil || r.checkStagedSelection() != nil || !r.fixedMutation {
				t.Fatal("original selection prefix refused", err)
			}
			owner, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records, *r.stagedReplacements)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			stage := &installationTransaction{lease: r.reader.lease, selected: r.intent.Candidate, intent: r.reader.files[filepath.Join(r.reader.lease.path, "transition.json")], journal: owner, fixed: make(map[string]fixedFileObservation)}
			phase := records["0005.json"]
			for range 2 {
				if err := stage.recordSelectionPublication(t.Context()); err != nil {
					t.Fatal("original publishing record did not resync", err)
				}
			}
			original := fixedFileObservation{parent: r.reader.lease.identity, file: r.reader.files[filename]}
			name := digestHex([]byte(filename)) + ".json"
			if err := stage.recordFixedReplacement(t.Context(), filename, original, before, after); err != nil {
				t.Fatal("original selection preimage record refused", err)
			}
			replacement := owner.Bytes(journal.Replacements, name)
			recordPath := filepath.Join(r.journal, "replacements", name)
			recordBefore, err := os.Lstat(recordPath)
			if err != nil {
				t.Fatal(err)
			}
			written, err := stage.replaceRecordedFixedFile(t.Context(), filename, original, before, after)
			if err != nil || !os.SameFile(written.identity, original.file.identity) || !bytes.Equal(written.body, after) || !bytes.Equal(owner.Bytes(journal.Replacements, name), replacement) {
				t.Fatal("selection repair lost original inode/record", err)
			}
			actual, readErr := os.ReadFile(filename)
			recordAfter, statErr := os.Lstat(recordPath)
			if readErr != nil || statErr != nil || !bytes.Equal(actual, after) || !sameObservedFile(recordBefore, recordAfter) {
				t.Fatal("physical selection bytes or original replacement record differ", readErr, statErr)
			}
			current, err := os.Lstat(filepath.Join(r.journal, "0005.json"))
			if err != nil || !sameObservedFile(phase.Identity, current) {
				t.Fatal("publishing record rewritten", err)
			}
			if _, err := os.Lstat(filepath.Join(r.journal, "0006.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("physical repair fabricated reload", err)
			}
		})
	}
}

func TestInstallationNativeStagedSelectionRefusesForeignProvenance(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, mutation := range []string{"missing-record", "foreign-bytes", "suffix", "wrong-digest", "wrong-inode", "missing-fixed-record", "missing-fixed-completion", "wrong-phase", "early-record"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			r, filename := selectionPublishingFixture(t, true)
			before, err := canonicalJSON(r.intent.Previous)
			if err != nil {
				t.Fatal(err)
			}
			body := before[:len(before)/2]
			recordPath := filepath.Join(r.journal, "replacements", digestHex([]byte(filename))+".json")
			switch mutation {
			case "missing-all-records":
				for _, name := range generation.Names() {
					if err := os.Remove(filepath.Join(r.journal, generationFileRecordName(name))); err != nil {
						t.Fatal(err)
					}
				}
			case "missing-marker":
				if err := os.Remove(filepath.Join(r.journal, "generation-files.json")); err != nil {
					t.Fatal(err)
				}
			case "missing-record":
				if err := os.Remove(recordPath); err != nil {
					t.Fatal(err)
				}
			case "foreign-bytes":
				body = []byte("foreign selection\n")
			case "suffix":
				body = append(bytes.Clone(before), '!')
			case "wrong-digest", "wrong-inode":
				raw, err := os.ReadFile(recordPath)
				if err != nil {
					t.Fatal(err)
				}
				if mutation == "wrong-digest" {
					raw = bytes.Replace(raw, []byte(digestHex(before)), []byte(digestHex(body)), 1)
				} else {
					native := r.reader.files[filename].identity.Sys().(*syscall.Stat_t)
					raw = bytes.Replace(raw, []byte(fmt.Sprintf("\"inode\":%d", native.Ino)), []byte(fmt.Sprintf("\"inode\":%d", native.Ino+1)), 1)
				}
				if err := os.WriteFile(recordPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
			case "missing-fixed-record":
				resources, err := recoveryFixedBytes(r.checked)
				if err != nil {
					t.Fatal(err)
				}
				delete(resources, filename)
				paths := make([]string, 0, len(resources))
				for path := range resources {
					paths = append(paths, path)
				}
				sort.Strings(paths)
				if err := os.Remove(filepath.Join(r.journal, "replacements", digestHex([]byte(paths[8]))+".json")); err != nil {
					t.Fatal(err)
				}
			case "missing-fixed-completion":
				if err := os.Remove(filepath.Join(r.journal, "0004.json")); err != nil {
					t.Fatal(err)
				}
			case "wrong-phase":
				raw, err := os.ReadFile(filepath.Join(r.journal, "0005.json"))
				if err != nil {
					t.Fatal(err)
				}
				raw = bytes.Replace(raw, []byte("publishing-selection"), []byte("reloading-manager"), 1)
				if err := os.WriteFile(filepath.Join(r.journal, "0005.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			case "early-record":
				if err := os.Remove(filepath.Join(r.journal, "0005.json")); err != nil {
					t.Fatal(err)
				}
				body = before
			}
			observeFixtureSelection(t, r, filename, body)
			_, err = r.readStagedJournal(t.Context())
			if err == nil && r.checkStagedFixedResources() == nil && r.checkStagedSelection() == nil {
				t.Fatal("foreign selection provenance admitted", mutation)
			}
		})
	}
}

// Physical provenance only; no manager effects, proofs or runtime substitute.
func reloadRecoveryFixture(t *testing.T, completed bool) (*successorRecoveryNative, string) {
	t.Helper()
	r, filename := selectionPublishingFixture(t, true)
	body, err := canonicalJSON(r.intent.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	observeFixtureSelection(t, r, filename, body)
	phases := []string{"reloading-manager"}
	if completed {
		phases = append(phases, "successor-reloaded-stopped")
	}
	for index, phase := range phases {
		body := fmt.Sprintf("{\"schema\":\"ardents-endpoint-installation-transition-v1\",\"generation_digest\":%q,\"binding_digest\":%q,\"phase\":%q}\n", r.intent.Candidate.GenerationDigest, r.intent.Candidate.BindingDigest, phase)
		if err := os.WriteFile(filepath.Join(r.journal, fmt.Sprintf("000%d.json", index+6)), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return r, filename
}

func TestInstallationNativeStagedReloadRetainsOriginalRecords(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, completed := range []bool{false, true} {
		t.Run(fmt.Sprint(completed), func(t *testing.T) {
			t.Parallel()
			r, _ := reloadRecoveryFixture(t, completed)
			records, err := r.readStagedJournal(t.Context())
			if err != nil || r.checkStagedSelection() != nil || r.checkStagedFixedResources() != nil || !r.fixedMutation {
				t.Fatal("full reload provenance refused", err)
			}
			owner, err := journal.OpenTransition(t.Context(), filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records, *r.stagedReplacements)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			for _, name := range []string{"0006.json", "0007.json"} {
				original, exists := records[name]
				if !exists {
					continue
				}
				for range 2 {
					if err := owner.Resync(t.Context(), journal.Transitions, name); err != nil {
						t.Fatal(err)
					}
				}
				current, err := os.Lstat(filepath.Join(r.journal, name))
				if err != nil || !sameObservedFile(original.Identity, current) || !bytes.Equal(original.Bytes, owner.Bytes(journal.Transitions, name)) {
					t.Fatal("original reload record rewritten", err)
				}
			}
			if _, err := os.Lstat(filepath.Join(r.journal, "started-invocation.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("record resync fabricated start", err)
			}
		})
	}
}

func TestInstallationNativeStagedReloadRefusesIncompleteProvenance(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, mutation := range []string{"old-selection", "partial-selection", "missing-selection-record", "missing-publishing", "missing-reload", "wrong-reload-phase", "failure"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			r, filename := reloadRecoveryFixture(t, true)
			switch mutation {
			case "old-selection", "partial-selection":
				body, err := canonicalJSON(r.intent.Previous)
				if err != nil {
					t.Fatal(err)
				}
				if mutation == "partial-selection" {
					body = body[:len(body)/2]
				}
				observeFixtureSelection(t, r, filename, body)
			case "missing-selection-record":
				if err := os.Remove(filepath.Join(r.journal, "replacements", digestHex([]byte(filename))+".json")); err != nil {
					t.Fatal(err)
				}
			case "missing-publishing", "missing-reload":
				name := "0005.json"
				if mutation == "missing-reload" {
					name = "0006.json"
				}
				if err := os.Remove(filepath.Join(r.journal, name)); err != nil {
					t.Fatal(err)
				}
			case "wrong-reload-phase":
				name := filepath.Join(r.journal, "0006.json")
				body, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, bytes.Replace(body, []byte("reloading-manager"), []byte("publishing-selection"), 1), 0600); err != nil {
					t.Fatal(err)
				}
			case "failure":
				if err := os.WriteFile(filepath.Join(r.journal, "original-transition-failure.json"), []byte("failure\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := r.readStagedJournal(t.Context())
			if err == nil && r.checkStagedSelection() == nil && r.checkStagedFixedResources() == nil {
				t.Fatal("incomplete reload provenance admitted", mutation)
			}
		})
	}
}

// Detached typed grammar only; these facts never grant physical quiescence.
func reloadTypedFixture(t *testing.T, request installationRequest, digest string) (managerProperties, managerProperties) {
	t.Helper()
	writes, err := writableDirectories(request)
	if err != nil {
		t.Fatal(err)
	}
	unit, service := managerProperties{}, managerProperties{}
	put := func(properties managerProperties, name, signature string, data any) {
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		properties[name] = systemd.Value{Type: signature, Data: raw}
	}
	for name, value := range map[string]string{"Id": "ardents-endpoint.service", "LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "/etc/systemd/system/ardents-endpoint.service"} {
		put(unit, name, "s", value)
	}
	put(unit, "DropInPaths", "as", []string{})
	put(unit, "Requires", "as", []string{"sysinit.target", "ardents-text-publisher.socket", "ardents-text-reader.socket", "system.slice"})
	for name, value := range map[string]string{"User": "ardents-endpoint", "Group": "ardents-endpoint", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "RestartMode": "normal"} {
		put(service, name, "s", value)
	}
	put(service, "MainPID", "u", 0)
	put(service, "UMask", "u", 63)
	put(service, "TimeoutStopUSec", "t", uint64(30_000_000))
	put(service, "ReadWritePaths", "as", writes)
	put(service, "SupplementaryGroups", "as", []string{})
	for _, name := range []string{"CapabilityBoundingSet", "AmbientCapabilities", "LimitCORE"} {
		put(service, name, "t", uint64(0))
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		put(service, name, "b", true)
	}
	put(service, "RemainAfterExit", "b", false)
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		put(service, name, "a(sasbttttuii)", []any{})
	}
	put(service, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_UNIX", "AF_INET6", "AF_INET"}})
	program := path.Join(request.InstallationRoot, "generations", digest, "ardents-linux-amd64")
	put(service, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, "endpoint", "start-installed", request.InstallationRoot}, []string{"no-env-expand"}, 0, 0, 0, 0, 0, 0, 0}})
	return unit, service
}

func TestInstallationNativeRecoveryConfigurationMatchPreservesPhase(t *testing.T) {
	t.Parallel() // Independent private filesystem/typed observations; no manager effects.
	for _, admission := range []struct {
		phase                              int
		fixedComplete, previous, candidate bool
	}{
		{3, false, true, false},
		{4, true, true, true},
		{5, true, true, true},
		{5, false, true, false},
		{6, true, true, true},
		{7, true, false, true},
	} {
		phase := admission.phase
		for _, image := range []string{"previous", "candidate", "foreign", "running"} {
			t.Run(fmt.Sprintf("%d-%t-%s", phase, admission.fixedComplete, image), func(t *testing.T) {
				t.Parallel()
				// This pure rule consumes only detached declarations/selections.
				// Actual journal, inode and manager custody have separate probes.
				request, err := decodeInstallationRequest(requestFixture())
				if err != nil {
					t.Fatal(err)
				}
				previous := generationSelection{GenerationDigest: digestHex([]byte("previous configuration"))}
				candidate := generationSelection{GenerationDigest: digestHex([]byte("candidate configuration"))}
				r := &successorRecoveryNative{intent: successorTransitionIntent{Previous: previous, Candidate: candidate, Request: request}, previous: inspectedGeneration{request: request, selected: previous}, checked: inspectedGeneration{request: request, selected: candidate}}
				r.inventory = map[string]bool{"0004.json": admission.fixedComplete, "0006.json": phase >= 6, "0007.json": phase >= 7}
				digest := r.intent.Previous.GenerationDigest
				if image == "candidate" {
					digest = r.intent.Candidate.GenerationDigest
				}
				if image == "foreign" {
					digest = digestHex([]byte("foreign generation"))
				}
				unit, service := reloadTypedFixture(t, r.intent.Request, digest)
				if image == "running" {
					service["MainPID"] = systemd.Value{Type: "u", Data: json.RawMessage("123")}
				}
				owned := &successorPreparation{previous: r.previous, recovery: r}
				_, got, err := owned.matchQuiescentConfiguration("255.4", unit, service)
				want := (image == "previous" && admission.previous) || (image == "candidate" && admission.candidate)
				if want && (err != nil || got != digest) {
					t.Fatal("exact configuration refused", got, err)
				}
				if !want && err == nil {
					t.Fatal("phase admitted foreign/running configuration", image, phase)
				}
				if owned.predecessor != nil || owned.authorization != nil || owned.started != nil {
					t.Fatal("pure match fabricated retained authority")
				}
			})
		}
	}
}

// Isolate the sealed generation provenance reader with one actual staging
// operation. No predecessor, fixed-resource replacement, Release or manager
// acceptance is claimed. Reader observations are independent of the creator.
func stagedGenerationBirthFixture(t *testing.T) *successorRecoveryNative {
	t.Helper()
	lease, request, files, selected := nativeStagingFixture(t)
	stage, err := stageInitialGeneration(t.Context(), lease, request, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	var binding generationBinding
	if err := json.Unmarshal(files["binding.json"], &binding); err != nil {
		t.Fatal(err)
	}
	reader := &installedRoot{lease: lease, installedFiles: &installedFiles{
		directory: lease.path, gid: 65534, files: map[string]fileObservation{},
		directories: map[string]os.FileInfo{}, mutableDirectories: map[string]os.FileInfo{},
	}}
	directory := filepath.Join(lease.path, "generations", selected.GenerationDigest)
	if err := reader.pinGenerationDirectory(directory); err != nil {
		t.Fatal(err)
	}
	r := &successorRecoveryNative{reader: reader, journal: filepath.Join(lease.path, "journals", selected.GenerationDigest)}
	r.intent.Candidate = selected
	r.checked = inspectedGeneration{files: files, selected: selected, binding: binding}
	return r
}

func TestInstallationNativeStagedRecoveryRetainsGenerationBirthInventory(t *testing.T) {
	t.Parallel()
	r := stagedGenerationBirthFixture(t)
	records, err := r.readStagedJournal(t.Context())
	if err != nil || len(records) != 19 {
		t.Fatal("actual complete generation birth inventory refused", len(records), err)
	}
	for _, name := range generation.Names() {
		if _, exists := records[generationFileRecordName(name)]; !exists {
			t.Fatal("original artifact record absent", name)
		}
	}
}

func TestInstallationNativeStagedRecoveryRefusesForeignGenerationBirth(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"missing-record", "missing-all-records", "missing-marker", "inode", "digest", "size", "group", "mode", "same-byte-file-inode"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel() // Every case owns its own original roots, journal and descriptors.
			r := stagedGenerationBirthFixture(t)
			name := "source.json"
			path := filepath.Join(r.journal, generationFileRecordName(name))
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var birth generationFileBirth
			if err := json.Unmarshal(body, &birth); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "missing-all-records":
				for _, artifact := range generation.Names() {
					if err := os.Remove(filepath.Join(r.journal, generationFileRecordName(artifact))); err != nil {
						t.Fatal(err)
					}
				}
			case "missing-marker":
				if err := os.Remove(filepath.Join(r.journal, "generation-files.json")); err != nil {
					t.Fatal(err)
				}
			case "missing-record":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "same-byte-file-inode":
				file := filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest, name)
				original, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(file, filepath.Join(filepath.Dir(r.journal), "retained-source")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, original, 0640); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(file, 0, 65534); err != nil {
					t.Fatal(err)
				}
			default:
				switch mutation {
				case "inode":
					birth.Inode++
				case "digest":
					birth.Digest = r.intent.Candidate.BindingDigest
				case "size":
					birth.Size++
				case "group":
					birth.GID++
				case "mode":
					birth.Mode = 0644
				}
				body, err = json.Marshal(birth)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(body, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.readStagedJournal(t.Context()); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign original-file provenance admitted", mutation, err)
			}
		})
	}
}

func TestInstallationNativeStagedRecoveryRetainsOldDirectoryOnlyProvenance(t *testing.T) {
	t.Parallel()
	r := stagedGenerationBirthFixture(t)
	for _, name := range generation.Names() {
		if err := os.Remove(filepath.Join(r.journal, generationFileRecordName(name))); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(r.journal, "generation-files.json")); err != nil {
		t.Fatal(err)
	}
	records, err := r.readStagedJournal(t.Context())
	if err != nil || len(records) != 3 {
		t.Fatal("unchanged historical complete sealed prefix refused", err)
	}
	if _, present := records["generation-files.json"]; present {
		t.Fatal("old prefix fabricated new custody")
	}
}

// This fixture creates actual private files, Unix socket and durable removal
// records. It supplies no successful Release, process, manager, ACK or recovery.
func completionRecoveryFixture(t *testing.T, retainGuard bool, withAttempt ...bool) *successorRecoveryNative {
	t.Helper()
	stage := startBarrierFixture(t)
	base, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &completionCleanupContext{Context: base}
	barrier, err := prepareNativeStartBarrier(ctx, stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	if len(withAttempt) != 0 && withAttempt[0] {
		if err := barrier.recordStartAttempt(); err != nil {
			t.Fatal(err)
		}
	}
	if retainGuard {
		ctx.observe = func() {
			if _, err := os.Lstat(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "completion-guard-removal.json")); err == nil {
				cancel()
			}
		}
	}
	err = barrier.retireRecordedCompletion()
	if retainGuard && !errors.Is(err, context.Canceled) || !retainGuard && err != nil {
		t.Fatal("unexpected cleanup prefix", err)
	}
	return &successorRecoveryNative{
		reader:  &installedRoot{lease: stage.lease, installedFiles: &installedFiles{directory: stage.lease.path, gid: stage.lease.identity.Sys().(*syscall.Stat_t).Gid, files: make(map[string]fileObservation), directories: map[string]os.FileInfo{stage.lease.path: stage.lease.identity}, mutableDirectories: make(map[string]os.FileInfo)}},
		journal: filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest), intentBody: stage.intent.body,
	}
}

func TestInstallationNativeCompletionRecoveryBindsOptionalStartIntention(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"unchanged", "intent", "socket", "schema", "noncanonical", "hardlink"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel() // Independent original journal/socket; no manager effects.
			r := completionRecoveryFixture(t, true, true)
			filename := filepath.Join(r.journal, "start-attempt.json")
			body, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			var attempt candidateStartAttempt
			if err := json.Unmarshal(body, &attempt); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "intent":
				attempt.IntentDigest = strings.Repeat("a", 64)
			case "socket":
				attempt.SocketRecordDigest = strings.Repeat("a", 64)
			case "schema":
				attempt.Schema += "-foreign"
			case "hardlink":
				if err := os.Link(filename, filepath.Join(filepath.Dir(r.journal), "attempt-copy")); err != nil {
					t.Fatal(err)
				}
			}
			if mutation != "unchanged" && mutation != "hardlink" {
				body, err = json.Marshal(attempt)
				if err != nil {
					t.Fatal(err)
				}
				if mutation == "noncanonical" {
					body = append(body, ' ')
				}
				if err := os.WriteFile(filename, append(body, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err = r.readRemovals(t.Context())
			if mutation == "unchanged" {
				if err != nil {
					t.Fatal("exact original intention refused", err)
				}
				// Intention and cleanup records still grant no successful recovery.
				if _, err := completeSuccessorRecovery(t.Context(), r, time.Now(), nil, enrollment.Candidate{}, release.Inputs{}); !errors.Is(err, ErrInput) {
					t.Fatal("intention minted fresh authority", err)
				}
				if err := os.Remove(filename); err != nil {
					t.Fatal(err)
				}
				if err := r.readRemovals(t.Context()); !errors.Is(err, ErrBinding) {
					t.Fatal("loss of this opening's original intention became historical absence", err)
				}
			} else if !errors.Is(err, ErrBinding) {
				t.Fatal("foreign start intention admitted", mutation, err)
			}
		})
	}
}

func TestInstallationNativeCompletionRecoveryRepairsOriginalGuardOnly(t *testing.T) {
	r := completionRecoveryFixture(t, true)
	if err := r.readRemovals(t.Context()); err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(r.reader.lease.path, "start-guard.json")
	if _, err := os.Lstat(guard); err != nil {
		t.Fatal("fixture lost original guard", err)
	}
	if err := r.finishRemoval(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(guard); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("original guard remains", err)
	}
	if err := r.observeRemovals(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.reader.lease.writer.Stat(); err != nil {
		t.Fatal("mechanism released writer", err)
	}
}

func TestInstallationNativeCompletionRecoveryReopensPhysicalAbsence(t *testing.T) {
	r := completionRecoveryFixture(t, false)
	if err := r.readRemovals(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.finishRemoval(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Missing proof cannot become a successful recovery from physical absence.
	if _, err := completeSuccessorRecovery(t.Context(), r, time.Now(), nil, enrollment.Candidate{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatal("removal records minted authorization", err)
	}
}

func TestInstallationNativeCompletionRecoveryRefusesGuardSubstitution(t *testing.T) {
	for _, mutation := range []string{"inode", "same-inode-access", "lost-original"} {
		t.Run(mutation, func(t *testing.T) {
			r := completionRecoveryFixture(t, true)
			if err := r.readRemovals(t.Context()); err != nil {
				t.Fatal(err)
			}
			guard := filepath.Join(r.reader.lease.path, "start-guard.json")
			switch mutation {
			case "inode":
				// Keep the original linked so immediate inode reuse cannot hide substitution.
				if err := os.Rename(guard, guard+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(guard, r.intentBody, 0600); err != nil {
					t.Fatal(err)
				}
			case "same-inode-access":
				if err := os.Chmod(guard, 0640); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(guard, 0600); err != nil {
					t.Fatal(err)
				}
			case "lost-original":
				if err := os.Remove(guard); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.finishRemoval(t.Context()); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign/lost guard accepted", err)
			}
			if mutation != "lost-original" {
				if _, err := os.Lstat(guard); err != nil {
					t.Fatal("refusal removed guard", err)
				}
			}
		})
	}
}

func TestInstallationNativeCompletionRecoveryRejectsForeignGuardOnFirstOpening(t *testing.T) {
	r := completionRecoveryFixture(t, true)
	guard := filepath.Join(r.reader.lease.path, "start-guard.json")
	if err := os.Rename(guard, guard+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guard, r.intentBody, 0600); err != nil {
		t.Fatal(err)
	}
	// No in-memory observation of the guard exists yet. Only the durable
	// original removal identity can reject these same bytes on a foreign inode.
	if err := r.readRemovals(t.Context()); !errors.Is(err, ErrBinding) {
		t.Fatal("first recovery opening adopted same-byte foreign inode", err)
	}
	if _, err := os.Lstat(guard); err != nil {
		t.Fatal("refusal removed foreign guard", err)
	}
}

func TestInstallationNativeCompletionRecoveryRefusesMissingRemovalAndCancellation(t *testing.T) {
	r := completionRecoveryFixture(t, true)
	if err := r.readRemovals(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := r.finishRemoval(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation renewed cleanup", err)
	}
	if _, err := os.Lstat(filepath.Join(r.reader.lease.path, "start-guard.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(r.journal, "completion-socket-removal.json")); err != nil {
		t.Fatal(err)
	}
	if err := r.readRemovals(t.Context()); !errors.Is(err, ErrBinding) {
		t.Fatal("absence replaced original removal provenance", err)
	}
}
