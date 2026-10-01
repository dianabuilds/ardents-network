//go:build linux

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPendingTransitionBarrierRefusesIntentAndFailure(t *testing.T) {
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

func TestTransitionStartBarrierNeedsExplicitCompletionBeforeDeadline(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "transition.json")
	if err := os.WriteFile(path, []byte("pending owner"), 0600); err != nil {
		t.Fatal(err)
	}
	interrupted, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := awaitStartCompletion(interrupted, root, selection{}, [16]byte{}); err == nil {
		t.Fatal("interrupted owner opened the start barrier", err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("waiting Endpoint changed the root recovery cursor", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	complete, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := awaitStartCompletion(complete, root, selection{}, [16]byte{}); err != nil {
		t.Fatal("explicit completion did not open the barrier", err)
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

func TestTransitionArchiveRefusesEmptyOriginalFailure(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	selected := selection{GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	intent := transitionIntent{Candidate: selected, Request: Request{InstallationRoot: root}}
	journal := filepath.Join(root, "journals", selected.GenerationDigest)
	if err := os.MkdirAll(journal, 0700); err != nil {
		t.Fatal(err)
	}
	if err := restoreTransitionIntent(intent); err != nil {
		t.Fatal(err)
	}
	if err := appendGenerationRecord(journal, "original-transition-failure.json", selected, "successor-transition-failed", nil); err != nil {
		t.Fatal(err)
	}
	if err := archiveTransitionIntent(root, journal, intent); err == nil {
		t.Fatal("empty archived original error accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, "transition.json")); err != nil {
		t.Fatal("refusal removed pending recovery provenance", err)
	}
}

func TestTransitionAdmissionRemainsClosedDuringFailedArchiveSync(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	selected := selection{GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	intent := transitionIntent{Candidate: selected, Request: Request{InstallationRoot: root}}
	journal := filepath.Join(root, "journals", selected.GenerationDigest)
	if err := os.MkdirAll(journal, 0700); err != nil {
		t.Fatal(err)
	}
	if err := restoreTransitionIntent(intent); err != nil {
		t.Fatal(err)
	}
	ownerCtx, ownerCancel := context.WithTimeout(context.Background(), time.Second)
	defer ownerCancel()
	completion, err := prepareStartCompletion(ownerCtx, root, intent, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer completion.close()
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	original := errors.New("causal archive sync refusal")
	go func() {
		first := true
		done <- archiveTransitionIntentWithSync(root, journal, intent, func(string) error {
			if first {
				first = false
				close(entered)
				<-release
			}
			return original
		})
	}()
	select {
	case <-entered:
	case archiveErr := <-done:
		t.Fatal("archive did not reach the durability boundary", archiveErr)
	case <-time.After(time.Second):
		close(release)
		t.Fatal("archive did not reach the durability boundary before deadline")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	invocation := [16]byte{1}
	err = awaitStartCompletion(ctx, root, selected, invocation)
	cancel()
	close(release)
	if archiveErr := <-done; !errors.Is(archiveErr, original) {
		t.Fatal("original archive failure lost", archiveErr)
	}
	if err == nil {
		t.Fatal("Endpoint admitted before successful archive sync", err)
	}
}
