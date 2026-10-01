//go:build linux

package installation

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledStartRefusesPendingIntentAndFailureBeforeManagerEffects(t *testing.T) {
	root := t.TempDir()
	if err := refusePendingTransition(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"transition.json", "transition-failure.json"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("pending"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := refusePendingTransition(root); err == nil {
			t.Fatal("pending installation permitted implicit restart")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStartFailureRecoveryArchivesPendingFailureWithoutErasingOriginal(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	selected := selection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	intent := transitionIntent{Schema: "ardents-endpoint-installation-successor-v1", Candidate: selected, Request: Request{InstallationRoot: root}}
	journal := filepath.Join(root, "journals", selected.GenerationDigest)
	if err := os.MkdirAll(journal, 0700); err != nil {
		t.Fatal(err)
	}
	if err := restoreTransitionIntent(intent); err != nil {
		t.Fatal(err)
	}
	if err := archiveTransitionIntent(root, journal, intent); err != nil {
		t.Fatal(err)
	}
	if err := restoreTransitionIntent(intent); err != nil {
		t.Fatal(err)
	}
	if err := retainTransitionFailure(root, selected, errors.New("original start refused")); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(root, "transition-failure.json"))
	if err := archiveTransitionIntent(root, journal, intent); err != nil {
		t.Fatal(err)
	}
	if err := refusePendingTransition(root); err != nil {
		t.Fatal("completed explicit recovery retained a startup-blocking cursor", err)
	}
	archived, _ := os.ReadFile(filepath.Join(journal, "original-transition-failure.json"))
	if !bytes.Equal(archived, original) {
		t.Fatal("recovery replaced the original start refusal")
	}
	if err := restoreTransitionIntent(intent); err != nil {
		t.Fatal(err)
	}
	if err := retainTransitionFailure(root, selected, errors.New("later recovery refused")); err != nil {
		t.Fatal(err)
	}
	if err := archiveTransitionIntent(root, journal, intent); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(journal, "original-transition-failure.json"))
	if !bytes.Equal(again, original) {
		t.Fatal("later explicit recovery erased the first failure")
	}
}
