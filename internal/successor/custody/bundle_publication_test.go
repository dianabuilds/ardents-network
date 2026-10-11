package custody

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnconfirmedBundleReplacementKeepsExactEncryptedEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retained.bundle")
	previous := []byte(independentEnvelope)
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := publishAndTestBundle(context.Background(), path, []byte("proposed encrypted replacement"), []byte("distinct recovery password"), AuthorityBinding{}, &sequenceSecrets{}); !errors.Is(err, ErrInvalid) {
		t.Fatal("unconfirmed replacement accepted", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, previous) {
		t.Fatal("unconfirmed replacement changed previous evidence", err)
	}
}

func TestCopyEncryptedBundleKeepsDestinationDuringReplacementPreparation(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "owner-chosen-bundle.json")
	previous := []byte(`{"already":"encrypted-by-its-own-envelope"}`)
	if err := os.WriteFile(destination, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := copyEncryptedBundle(destination, parent)
	if err != nil {
		t.Fatalf("copy previous encrypted bundle: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(backup) })
	retained, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read original destination: %v", err)
	}
	if !bytes.Equal(retained, previous) {
		t.Fatal("preparing rollback copy changed the current bundle")
	}
	copied, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read encrypted backup: %v", err)
	}
	if !bytes.Equal(copied, previous) {
		t.Fatal("rollback copy differs from the encrypted destination")
	}
}

func TestRestorePreviousBundleRestoresEncryptedBytesAfterFailedPublication(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "owner-chosen-bundle.json")
	backup := filepath.Join(parent, ".ardents-recovery-bundle-previous-test")
	previous := []byte(`{"previous":"encrypted-by-its-own-envelope"}`)
	replacement := []byte(`{"replacement":"encrypted-by-its-own-envelope"}`)
	if err := os.WriteFile(destination, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("final bundle test restore failed")
	returned, keepBackup := restorePreviousBundle(destination, backup, parent, cause)
	if !errors.Is(returned, cause) || keepBackup {
		t.Fatalf("restore result = (%v, keep=%t), want original failure and no retained backup", returned, keepBackup)
	}
	actual, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, previous) {
		t.Fatal("failed publication did not restore the previous encrypted bundle")
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("restored backup remains at temporary path: %v", err)
	}
}
