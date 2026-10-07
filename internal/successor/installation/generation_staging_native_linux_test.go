//go:build installation_native

package installation

import (
	"bytes"
	"context"
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
	if readErr != nil || decodeErr != nil || failure.Phase != "successor-transition-failed" || failure.GenerationDigest != selected.GenerationDigest || failure.BindingDigest != selected.BindingDigest || failure.OriginalError != err.Error() {
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
