//go:build linux

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestInitialRepairRequiresBirthRecordDurabilityBeforeMutation(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	journal := filepath.Join(root, "journal")
	if err := os.Mkdir(journal, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "installed")
	body := []byte("authenticated complete resource")
	selected := selection{GenerationDigest: digestHex(body)}
	if err := createInstallationFile(context.Background(), path, body, 0644, 0, journal, selected); err != nil {
		t.Fatal(err)
	}
	prefix := body[:5]
	if err := os.WriteFile(path, prefix, 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(journal, "creations", digestHex([]byte(path))+".json")
	original, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("initial birth record directory sync refused")
	err = repairCreatedInstallationFileWithSync(context.Background(), path, body, 0644, 0, journal, selected, func(directory string) error {
		if directory == filepath.Dir(recordPath) {
			return refusal
		}
		return syncDirectory(directory)
	})
	if !errors.Is(err, refusal) {
		t.Fatal("visible birth record bypassed durability refusal", err)
	}
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, prefix) {
		t.Fatal("initial repair mutated resource before birth record durability", err)
	}
	if retained, err := os.ReadFile(recordPath); err != nil || !bytes.Equal(retained, original) {
		t.Fatal("initial repair changed birth record", err)
	}
	// Cancellation after journal durability must still precede resource repair.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = repairCreatedInstallationFileWithSync(ctx, path, body, 0644, 0, journal, selected, func(directory string) error {
		if err := syncDirectory(directory); err != nil {
			return err
		}
		if directory == filepath.Dir(recordPath) {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("post-journal cancellation lost", err)
	}
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, prefix) {
		t.Fatal("cancelled initial repair changed resource", err)
	}
	if retained, err := os.ReadFile(recordPath); err != nil || !bytes.Equal(retained, original) {
		t.Fatal("cancelled initial repair changed birth record", err)
	}
}

func TestInitialRepairRetainsPostWriteSyncFailureForExplicitRetry(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	journal := filepath.Join(root, "journal")
	if err := os.Mkdir(journal, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "installed")
	body := []byte("authenticated complete resource")
	selected := selection{GenerationDigest: digestHex(body)}
	if err := createInstallationFile(context.Background(), path, body, 0644, 0, journal, selected); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body[:5], 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(journal, "creations", digestHex([]byte(path))+".json")
	original, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("initial repaired resource directory sync refused")
	err = repairCreatedInstallationFileWithSync(context.Background(), path, body, 0644, 0, journal, selected, func(directory string) error {
		if directory == filepath.Dir(path) {
			return refusal
		}
		return syncDirectory(directory)
	})
	if !errors.Is(err, refusal) {
		t.Fatal("post-write initial repair refusal lost", err)
	}
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, body) {
		t.Fatal("post-write refusal misrepresented repaired bytes", err)
	}
	if retained, err := os.ReadFile(recordPath); err != nil || !bytes.Equal(retained, original) {
		t.Fatal("post-write refusal changed birth record", err)
	}
	if err := repairCreatedInstallationFile(context.Background(), path, body, 0644, 0, journal, selected); err != nil {
		t.Fatal("explicit initial repair retry refused", err)
	}
	if after, err := os.Lstat(path); err != nil || !os.SameFile(info, after) {
		t.Fatal("explicit retry replaced recorded inode", err)
	}
}

func TestInitialCreationJournalFailureRetainsEmptyFileAndOriginalRecord(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	journal := filepath.Join(root, "journal")
	records := filepath.Join(journal, "creations")
	if err := os.MkdirAll(records, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "installed")
	recordPath := filepath.Join(records, digestHex([]byte(path))+".json")
	original := []byte("existing record must not be overwritten")
	if err := os.WriteFile(recordPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	selected := selection{GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	first := createInstallationFile(context.Background(), path, []byte("authorized bytes"), 0644, 0, journal, selected)
	if !errors.Is(first, os.ErrExist) {
		t.Fatal("expected actual exclusive journal write refusal", first)
	}
	body, err := os.ReadFile(path)
	if err != nil || len(body) != 0 {
		t.Fatal("body was written before durable creation record", err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("failed creation lost its private empty inode", err)
	}
	second := createInstallationFile(context.Background(), path, []byte("authorized bytes"), 0644, 0, journal, selected)
	if !errors.Is(second, os.ErrExist) {
		t.Fatal("retry adopted unrecorded partial file", second)
	}
	retained, _ := os.ReadFile(recordPath)
	if !bytes.Equal(retained, original) {
		t.Fatal("failed creation or retry overwrote the original record")
	}
	if err := repairCreatedInstallationFile(context.Background(), path, []byte("authorized bytes"), 0644, 0, journal, selected); err == nil {
		t.Fatal("unrecorded empty file gained recovery ownership")
	}
}

func TestInitialCreationRecordsExactInodeAndRefusesExistingFile(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	journal := filepath.Join(root, "journal")
	if err := os.Mkdir(journal, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "installed")
	body := []byte("authenticated resource bytes")
	selected := selection{GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	if err := createInstallationFile(context.Background(), path, body, 0555, 1001, journal, selected); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	identity := info.Sys().(*syscall.Stat_t)
	recordPath := filepath.Join(journal, "creations", digestHex([]byte(path))+".json")
	encoded, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	var record replacementRecord
	if err := decodeCanonical(encoded, 4096, &record); err != nil {
		t.Fatal(err)
	}
	if record.Schema != "ardents-endpoint-installation-creation-v1" || record.Path != path || record.Device != uint64(identity.Dev) ||
		record.Inode != identity.Ino || record.GenerationDigest != selected.GenerationDigest || record.PreviousDigest != "" ||
		record.CandidateDigest != digestHex(body) || record.Mode != 0555 || record.GID != 1001 || identity.Gid != 1001 || info.Mode().Perm() != 0555 {
		t.Fatal("created file did not match its durable birth record")
	}
	if err := validateCreatedInstallationFile(path, body, 0555, 1001, journal, selected); err != nil {
		t.Fatal(err)
	}
	if err := createInstallationFile(context.Background(), path, []byte("foreign"), 0555, 1001, journal, selected); err == nil {
		t.Fatal("initial writer adopted an existing inode")
	}
	observed, _ := os.ReadFile(path)
	after, _ := os.ReadFile(recordPath)
	if !bytes.Equal(body, observed) || !bytes.Equal(encoded, after) {
		t.Fatal("refused retry changed original file or provenance")
	}
	if err := os.WriteFile(path, []byte("foreign"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := validateCreatedInstallationFile(path, body, 0555, 1001, journal, selected); err == nil {
		t.Fatal("owned inode with foreign bytes passed initial pre-effect recovery")
	}
	if err := os.WriteFile(path, body[:5], 0555); err != nil {
		t.Fatal(err)
	}
	if err := validateCreatedInstallationFile(path, body, 0555, 1001, journal, selected); err != nil {
		t.Fatal("owned authenticated prefix was refused", err)
	}
	if err := repairCreatedInstallationFile(context.Background(), path, body, 0555, 1001, journal, selected); err != nil {
		t.Fatal(err)
	}
	repaired, _ := os.ReadFile(path)
	repairedInfo, err := os.Lstat(path)
	if err != nil || !bytes.Equal(repaired, body) || !os.SameFile(info, repairedInfo) {
		t.Fatal("initial repair did not complete the recorded inode")
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveGenerationFile(path, body, 0555, 1001); err != nil {
		t.Fatal(err)
	}
	if err := validateCreatedInstallationFile(path, body, 0555, 1001, journal, selected); err == nil {
		t.Fatal("matching bytes on a substituted inode acquired initial recovery ownership")
	}
	if err := repairCreatedInstallationFile(context.Background(), path, body, 0555, 1001, journal, selected); err == nil {
		t.Fatal("repair wrote a substituted inode")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := repairCreatedInstallationFile(context.Background(), path, body, 0555, 1001, journal, selected); err == nil {
		t.Fatal("repair recreated a missing recorded inode")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("refused repair recreated the missing file")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	absent := filepath.Join(root, "cancelled")
	if err := createInstallationFile(cancelled, absent, body, 0644, 0, journal, selected); err == nil {
		t.Fatal("cancelled creation passed")
	}
	if _, err := os.Lstat(absent); !os.IsNotExist(err) {
		t.Fatal("cancelled creation touched its destination")
	}
}
