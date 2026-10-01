//go:build linux

package installation

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartGuardRetainsExactIntentAcrossCursorArchival(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	intent := transitionIntent{Request: Request{InstallationRoot: root}, Candidate: selection{GenerationDigest: digestHex([]byte("candidate"))}}
	if err := retainStartGuard(root, intent); err != nil {
		t.Fatal(err)
	}
	wanted, err := canonicalJSON(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := restoreTransitionIntent(intent); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(root, "journals", intent.Candidate.GenerationDigest)
	if err := os.MkdirAll(journal, 0700); err != nil {
		t.Fatal(err)
	}
	if err := archiveTransitionIntent(root, journal, intent); err != nil {
		t.Fatal(err)
	}
	guard, err := readStartGuard(root)
	if err != nil || !bytes.Equal(guard, wanted) {
		t.Fatal("cursor archival lost guarded recovery provenance", err)
	}
	foreign := intent
	foreign.Candidate.GenerationDigest = digestHex([]byte("foreign"))
	if err := retainStartGuard(root, foreign); err == nil {
		t.Fatal("another transition replaced a retained start guard")
	}
	guard, err = readStartGuard(root)
	if err != nil || !bytes.Equal(guard, wanted) {
		t.Fatal("refusal changed guarded recovery provenance", err)
	}
}

func TestStartGuardCleanupRefusesDifferentSocketInode(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	intent := transitionIntent{Request: Request{InstallationRoot: root}, Candidate: selection{GenerationDigest: digestHex([]byte("candidate"))}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	completion, err := prepareStartCompletion(ctx, root, intent, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := completion.close(); err != nil {
			t.Error(err)
		}
	}()
	path := filepath.Join(root, "start-completion.socket")
	if err := os.Rename(path, filepath.Join(root, "original-socket-inode")); err != nil {
		t.Fatal(err)
	}
	foreign, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	foreign.SetUnlinkOnClose(false)
	defer func() {
		if err := foreign.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Chown(path, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	foreignInfo, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := clearStartGuard(root, intent); err == nil {
		t.Fatal("cleanup adopted another socket inode with matching owner and mode")
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(current, foreignInfo) {
		t.Fatal("refusal removed the other listener", err)
	}
	if _, err := readStartGuard(root); err != nil {
		t.Fatal("refusal lost recovery guard", err)
	}
}

func TestStartGuardCleanupRefusesSubstitutedSocket(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	intent := transitionIntent{Request: Request{InstallationRoot: root}, Candidate: selection{GenerationDigest: digestHex([]byte("candidate"))}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	completion, err := prepareStartCompletion(ctx, root, intent, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := completion.close(); err != nil {
			t.Error(err)
		}
	}()
	originalGuard, err := readStartGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "start-completion.socket")
	if err := os.Rename(path, filepath.Join(root, "owned-listener")); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("unrelated file substituted at socket path")
	if err := os.WriteFile(path, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	if err := clearStartGuard(root, intent); err == nil {
		t.Fatal("cleanup removed a foreign completion path")
	}
	currentGuard, err := readStartGuard(root)
	if err != nil || !bytes.Equal(currentGuard, originalGuard) {
		t.Fatal("refused cleanup lost exact recovery provenance", err)
	}
	currentFile, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(currentFile, foreign) {
		t.Fatal("refused cleanup changed substituted bytes", err)
	}
	if err := refusePendingTransition(root); err == nil {
		t.Fatal("failed cleanup opened ordinary restart")
	}
}

func TestStartGuardRecoveryAtSocketBirthAndRemovalBoundaries(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	for _, boundary := range []string{"socket-before-birth-record", "socket-removed-before-record"} {
		t.Run(boundary, func(t *testing.T) {
			root := replacementTestRoot(t)
			intent := transitionIntent{Request: Request{InstallationRoot: root}, Candidate: selection{GenerationDigest: digestHex([]byte("candidate"))}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			completion, err := prepareStartCompletion(ctx, root, intent, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := completion.close(); err != nil {
				t.Fatal(err)
			}
			if boundary == "socket-before-birth-record" {
				if err := os.Remove(filepath.Join(root, "start-socket.json")); err != nil {
					t.Fatal(err)
				}
				before, err := os.Lstat(filepath.Join(root, "start-completion.socket"))
				if err != nil {
					t.Fatal(err)
				}
				if err := clearStartGuard(root, intent); err == nil {
					t.Fatal("unrecorded socket admitted cleanup")
				}
				after, err := os.Lstat(filepath.Join(root, "start-completion.socket"))
				if err != nil || !os.SameFile(before, after) {
					t.Fatal("refusal changed unrecorded socket", err)
				}
				if _, err := readStartGuard(root); err != nil {
					t.Fatal("refusal lost recovery intent", err)
				}
				if err := refusePendingTransition(root); err == nil {
					t.Fatal("unrecorded socket opened ordinary restart")
				}
				return
			}
			if err := os.Remove(filepath.Join(root, "start-completion.socket")); err != nil {
				t.Fatal(err)
			}
			if err := clearStartGuard(root, intent); err != nil {
				t.Fatal("owned interrupted socket removal could not complete", err)
			}
			for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
				if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Fatal("completed cleanup retained artifact", name, err)
				}
			}
		})
	}
}
