//go:build installation_native

package installation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// This fixture supplies filesystem-mechanism bytes only. It grants no genuine
// Enrollment/Release/account proof; preparation must obtain those separately.
func nativeStagingFixture(t *testing.T) (*installationLease, Request, map[string][]byte, generationSelection) {
	t.Helper()
	directory := filepath.Join(nativeRequestDirectory(t), "installation")
	lease, err := createInitialLease(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.close(); err != nil {
			t.Error(err)
		}
	})
	request, err := DecodeRequest(t.Context(), []byte(strings.Replace(string(requestFixture()), `"installation_root":"/installation"`, `"installation_root":"`+directory+`"`, 1)), true)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for _, name := range strings.Fields("ardents-linux-amd64 ardents-text-linux-amd64 ardents-text-reader@.service ardents-text-publisher@.service ardents-text-reader.socket ardents-text-publisher.socket 50-ardents-text.rules ardents-text.conf ardents-endpoint.service protected-endpoint.json request.json headless.json source.json endpoint-unit.service") {
		files[name] = []byte("mechanism bytes for " + name + "\n")
	}
	digest := digestHex(files["protected-endpoint.json"])
	binding := generationBinding{Schema: "ardents-endpoint-installation-binding-v1", InstallationRoot: directory, GenerationDigest: digest, UID: 65534, GID: 65534, Files: make(map[string]string)}
	for name, body := range files {
		binding.Files[name] = digestHex(body)
	}
	files["binding.json"], err = canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	return lease, request, files, generationSelection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: digest, BindingDigest: digestHex(files["binding.json"])}
}

func TestInstallationNativeGenerationStagesExactBytesBeforeSelection(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
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
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(lease.path, "selection.json")); !os.IsNotExist(err) {
		t.Fatal("staging selected a generation")
	}
	directory := filepath.Join(lease.path, "generations", selected.GenerationDigest)
	for name, wanted := range files {
		actual, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || !bytes.Equal(actual, wanted) {
			t.Fatalf("%s differs: %v", name, err)
		}
		info, err := os.Lstat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		native := info.Sys().(*syscall.Stat_t)
		if info.Mode() != mode || native.Uid != 0 || native.Gid != 65534 || native.Nlink != 1 {
			t.Fatalf("%s protection differs", name)
		}
		files[name][0] ^= 1
	}
	if err := stage.observe(); err != nil {
		t.Fatal("caller mutation changed the frozen stage", err)
	}
	journal := filepath.Join(lease.path, "journals", selected.GenerationDigest)
	for index, phase := range []string{"writing-generation", "generation-staged"} {
		actual, err := os.ReadFile(filepath.Join(journal, fmt.Sprintf("%04d.json", index+1)))
		wanted := fmt.Sprintf("{\"schema\":\"ardents-endpoint-installation-transition-v1\",\"generation_digest\":\"%s\",\"binding_digest\":\"%s\",\"phase\":\"%s\"}\n", selected.GenerationDigest, selected.BindingDigest, phase)
		if err != nil || string(actual) != wanted {
			t.Fatal("independent journal bytes differ", err)
		}
	}
}

func TestInstallationNativeGenerationRefusesTemplateSocketNamesBeforeEffects(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	lease, request, files, selected := nativeStagingFixture(t)
	for _, role := range []string{"reader", "publisher"} {
		name := "ardents-text-" + role + ".socket"
		files["ardents-text-"+role+"@.socket"] = files[name]
		delete(files, name)
	}
	var binding generationBinding
	if err := json.Unmarshal(files["binding.json"], &binding); err != nil {
		t.Fatal(err)
	}
	binding.Files = make(map[string]string)
	for name, body := range files {
		if name != "binding.json" {
			binding.Files[name] = digestHex(body)
		}
	}
	var err error
	files["binding.json"], err = canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	selected.BindingDigest = digestHex(files["binding.json"])
	if stage, err := stageInitialGeneration(t.Context(), lease, request, files, selected, 65534); stage != nil || !errors.Is(err, ErrBinding) {
		t.Fatal("foreign template socket names accepted", err)
	}
	for _, name := range []string{"transition.json", "journals", "generations"} {
		if _, err := os.Lstat(filepath.Join(lease.path, name)); !os.IsNotExist(err) {
			t.Fatal("invalid inventory caused native effects", name, err)
		}
	}
}

func TestInstallationNativeGenerationRefusesChangedOwnedStage(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	for _, mutation := range []string{"file-bytes", "file-inode", "file-hardlink", "foreign-file", "lost-birth", "birth-bytes", "foreign-journal", "foreign-generation-parent", "generation-inode", "intent-bytes"} {
		t.Run(mutation, func(t *testing.T) {
			lease, request, files, selected := nativeStagingFixture(t)
			stage, err := stageInitialGeneration(t.Context(), lease, request, files, selected, 65534)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := stage.close(); !errors.Is(err, ErrBinding) {
					t.Error("close lost the original binding refusal", err)
				}
			}()
			directory := filepath.Join(lease.path, "generations", selected.GenerationDigest)
			journal := filepath.Join(lease.path, "journals", selected.GenerationDigest)
			file := filepath.Join(directory, "source.json")
			switch mutation {
			case "file-bytes":
				err = os.WriteFile(file, []byte("different bytes\n"), 0640)
			case "file-inode":
				err = os.Rename(file, file+".original")
				if err == nil {
					err = os.WriteFile(file, files["source.json"], 0640)
				}
				if err == nil {
					err = os.Chown(file, 0, 65534)
				}
			case "file-hardlink":
				err = os.Link(file, filepath.Join(nativeRequestDirectory(t), "other-link"))
			case "foreign-file":
				err = os.WriteFile(filepath.Join(directory, "foreign"), nil, 0600)
			case "lost-birth":
				err = os.Remove(filepath.Join(journal, "generation-directory.json"))
			case "birth-bytes":
				err = os.WriteFile(filepath.Join(journal, "generation-directory.json"), []byte("different birth\n"), 0600)
			case "foreign-journal":
				err = os.WriteFile(filepath.Join(journal, "foreign"), nil, 0600)
			case "foreign-generation-parent":
				err = os.Mkdir(filepath.Join(lease.path, "generations", "foreign"), 0700)
			case "generation-inode":
				err = os.Rename(directory, directory+".original")
				if err == nil {
					err = os.Mkdir(directory, 0750)
				}
				if err == nil {
					err = os.Chown(directory, 0, 65534)
				}
			case "intent-bytes":
				err = os.WriteFile(filepath.Join(lease.path, "transition.json"), []byte("different intent\n"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := stage.observe(); !errors.Is(err, ErrBinding) {
				t.Fatal("changed owned stage accepted", err)
			}
			if err := stage.observe(); !errors.Is(err, ErrBinding) {
				t.Fatal("later observation renewed a refused stage", err)
			}
		})
	}
}

type cancellationAtStagedRecord struct {
	context.Context
	cancel   context.CancelFunc
	filename string
}

func (c *cancellationAtStagedRecord) Err() error {
	if body, err := os.ReadFile(c.filename); err == nil && bytes.Contains(body, []byte(`"phase":"generation-staged"`)) {
		c.cancel()
	}
	return c.Context.Err()
}

func TestInstallationNativeGenerationRetainsCompletedCancellation(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	lease, request, files, selected := nativeStagingFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	journal := filepath.Join(lease.path, "journals", selected.GenerationDigest)
	stage, err := stageInitialGeneration(&cancellationAtStagedRecord{Context: ctx, cancel: cancel, filename: filepath.Join(journal, "0002.json")}, lease, request, files, selected, 65534)
	if stage != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation accepted or lost", err)
	}
	if body, err := os.ReadFile(filepath.Join(journal, "0002.json")); err != nil || !bytes.Contains(body, []byte(`"phase":"generation-staged"`)) {
		t.Fatal("actual completed record erased", err)
	}
	body, readErr := os.ReadFile(filepath.Join(journal, "original-transition-failure.json"))
	var failure struct {
		Phase            string `json:"phase"`
		GenerationDigest string `json:"generation_digest"`
		BindingDigest    string `json:"binding_digest"`
		OriginalError    string `json:"original_error"`
	}
	decodeErr := json.Unmarshal(body, &failure)
	// The record retains the first failure before physical Close joins child
	// terminal results; the returned error may additionally contain those results.
	if readErr != nil || decodeErr != nil || failure.Phase != "successor-transition-failed" || failure.GenerationDigest != selected.GenerationDigest || failure.BindingDigest != selected.BindingDigest || failure.OriginalError != context.Canceled.Error() {
		t.Fatalf("late original failure differs: %s; read %v; decode %v; returned %v", body, readErr, decodeErr, err)
	}
	probe, err := os.OpenFile(filepath.Join(lease.path, "writer.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := probe.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("staging failure returned the owner's lease", err)
	}
	if _, err := os.Lstat(filepath.Join(lease.path, "selection.json")); !os.IsNotExist(err) {
		t.Fatal("failed stage selected")
	}
}

func accessStage(t *testing.T) *generationStage {
	t.Helper()
	stage := fixedCreationStage(t)
	// This profile exercises metadata and the kernel lease only. It cannot
	// supply successful global fixed files, NSS or manager admission.
	if err := stage.fixedPhase(t.Context(), "0004.json", "fixed-resources-installed"); err != nil {
		t.Fatal(err)
	}
	return stage
}

func TestInstallationNativeGenerationAccessKeepsOriginalLeaseAndInodes(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := accessStage(t)
	rootBefore := stage.lease.identity
	parentPath := filepath.Join(stage.lease.path, "generations")
	parentBefore, err := os.Lstat(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.promoteAccess(t.Context(), 65534); err != nil {
		t.Fatal(err)
	}
	for filename, original := range map[string]os.FileInfo{stage.lease.path: rootBefore, parentPath: parentBefore} {
		info, err := os.Lstat(filename)
		if err != nil {
			t.Fatal(err)
		}
		native := info.Sys().(*syscall.Stat_t)
		if !os.SameFile(original, info) || info.Mode() != os.ModeDir|0750 || native.Uid != 0 || native.Gid != 65534 {
			t.Fatal("read access changed original inode or bound owner")
		}
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	if err := stage.promoteAccess(t.Context(), 65534); err != nil {
		t.Fatal("same original access observation refused", err)
	}
	probe, err := os.OpenFile(filepath.Join(stage.lease.path, "writer.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := probe.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("promotion released writer", err)
	}
	if _, err := os.Stat(filepath.Join(stage.lease.path, "selection.json")); !os.IsNotExist(err) {
		t.Fatal("read access selected generation")
	}
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.lease.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("physical close did not release writer", err)
	}
}

func TestInstallationNativeGenerationAccessRefusesWrongGroupAndCancellation(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := accessStage(t)
	if err := stage.promoteAccess(t.Context(), 65533); !errors.Is(err, ErrBinding) {
		t.Fatal("unbound group accepted", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := stage.promoteAccess(ctx, 65534); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	for _, filename := range []string{stage.lease.path, filepath.Join(stage.lease.path, "generations")} {
		info, err := os.Lstat(filename)
		if err != nil || !privateJournalDirectory(info) {
			t.Fatal("refused promotion changed metadata", err)
		}
	}
}

// This filesystem observer loses the original caller only after the first
// original birth record is visible while the artifact remains empty. It does
// not supply Release authorization or a successful system-manager observation.
type fileBirthRecordCancellation struct {
	context.Context
	file, record string
}

func (ctx *fileBirthRecordCancellation) Err() error {
	file, err := os.Lstat(ctx.file)
	if err == nil && file.Size() == 0 {
		if record, err := os.Lstat(ctx.record); err == nil && record.Size() > 0 {
			return context.Canceled
		}
	}
	return ctx.Context.Err()
}

func TestInstallationNativeGenerationRecordsEachOriginalFileBirth(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
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
	for name, body := range files {
		info, err := os.Lstat(filepath.Join(lease.path, "generations", selected.GenerationDigest, name))
		if err != nil {
			t.Fatal(err)
		}
		native := info.Sys().(*syscall.Stat_t)
		mode := uint32(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		nameDigest, bodyDigest := sha256.Sum256([]byte(name)), sha256.Sum256(body)
		path := filepath.Join(lease.path, "journals", selected.GenerationDigest, fmt.Sprintf("generation-file-%x.json", nameDigest))
		actual, err := os.ReadFile(path)
		wanted := fmt.Sprintf("{\"schema\":\"ardents-endpoint-generation-file-v1\",\"generation_digest\":\"%s\",\"name\":\"%s\",\"device\":%d,\"inode\":%d,\"digest\":\"%x\",\"size\":%d,\"mode\":%d,\"gid\":65534}\n", selected.GenerationDigest, name, uint64(native.Dev), native.Ino, bodyDigest, len(body), mode)
		if err != nil || string(actual) != wanted {
			t.Fatal("independent original-file birth record differs", name, err)
		}
	}
}

func TestInstallationNativeGenerationBirthRecordPrecedesFirstArtifactBytes(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	lease, request, files, selected := nativeStagingFixture(t)
	// The declared deterministic filename order starts with the selected rules.
	name := "50-ardents-text.rules"
	digest := sha256.Sum256([]byte(name))
	file := filepath.Join(lease.path, "generations", selected.GenerationDigest, name)
	record := filepath.Join(lease.path, "journals", selected.GenerationDigest, fmt.Sprintf("generation-file-%x.json", digest))
	ctx := &fileBirthRecordCancellation{Context: t.Context(), file: file, record: record}
	stage, err := stageInitialGeneration(ctx, lease, request, files, selected, 65534)
	if stage != nil || !errors.Is(err, context.Canceled) {
		if stage != nil {
			_ = stage.close()
		}
		t.Fatal("first body was not ordered after durable birth provenance", err)
	}
	info, err := os.Lstat(file)
	if err != nil || info.Size() != 0 || info.Mode() != 0600 {
		t.Fatal("late record cancellation allowed artifact mutation", err)
	}
	body, err := os.ReadFile(record)
	var birth generationFileBirth
	if err != nil || json.Unmarshal(body, &birth) != nil {
		t.Fatal("birth record lost", err)
	}
	native := info.Sys().(*syscall.Stat_t)
	if birth.Device != uint64(native.Dev) || birth.Inode != native.Ino {
		t.Fatal("record lost actual original birth inode")
	}
	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil || len(entries) != 1 || entries[0].Name() != name {
		t.Fatal("cancellation began another file", err)
	}
}

func TestInstallationNativeSelectionRequiresPromotionAndExactOwnedBytes(t *testing.T) {
	stage := accessStage(t)
	if err := stage.promoteAccess(t.Context(), 65534); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(stage.lease.path, "selection.json")
	body, err := canonicalJSON(stage.selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.createFixedFile(t.Context(), filename, body, 0640, 65534); !errors.Is(err, ErrBinding) {
		t.Fatal("selection without original phase accepted", err)
	}
	if _, err := os.Lstat(filename); !os.IsNotExist(err) {
		t.Fatal("pre-intent selection appeared")
	}
	if err := stage.fixedPhase(t.Context(), "0005.json", "publishing-selection"); err != nil {
		t.Fatal(err)
	}
	if err := stage.createFixedFile(t.Context(), filename, []byte("foreign selection\n"), 0640, 65534); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign selection accepted", err)
	}
	if err := stage.createFixedFile(t.Context(), filename, body, 0640, 65534); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("selected bytes differ", err)
	}
	if err := stage.fixedPhase(t.Context(), "0006.json", "reloading-manager"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "0007.json")); !os.IsNotExist(err) {
		t.Fatal("selection fabricated installed completion")
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
}
