//go:build installation_native

package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
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
	if err != nil || !ownedRequestFile(info) || info.Size() != 0 || info.Mode().Perm() != 0600 {
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
	if err != nil || !sameStagingDirectory(original, current) {
		t.Fatal("cancelled promotion changed the original directory", err)
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatal("promotion intent lost", err)
	}
	if err := stage.changeFixedDirectoryMode(t.Context(), directory, 0555); err == nil {
		t.Fatal("initial operation silently adopted retained promotion intent")
	}
}
