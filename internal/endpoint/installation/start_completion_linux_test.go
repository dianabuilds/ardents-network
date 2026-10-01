//go:build linux

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestStartCompletionBindsLivePeerAndInvocation(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	for _, name := range []string{"complete", "foreign-pid", "stale-invocation", "owner-death", "cancel"} {
		t.Run(name, func(t *testing.T) {
			root := replacementTestRoot(t)
			selected := selection{GenerationDigest: digestHex([]byte("candidate")), BindingDigest: digestHex([]byte("binding"))}
			intent := transitionIntent{Request: Request{InstallationRoot: root}, Candidate: selected}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			completion, err := prepareStartCompletion(ctx, root, intent, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer completion.close()
			invocation := [16]byte{1}
			done := make(chan error, 1)
			go func() { done <- awaitStartCompletion(ctx, root, selected, invocation) }()
			pid := uint32(os.Getpid())
			expected := invocation
			if name == "foreign-pid" {
				pid++
			}
			if name == "stale-invocation" {
				expected[0]++
			}
			connection, err := completion.accept(ctx, selected, pid, 0, expected)
			if name == "foreign-pid" || name == "stale-invocation" {
				if err == nil {
					connection.Close()
					t.Fatal("substituted completion identity accepted")
				}
				if err := <-done; err == nil {
					t.Fatal("refused peer composed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			select {
			case err := <-done:
				t.Fatal("Endpoint admitted without completion", err)
			default:
			}
			if name == "cancel" {
				cancel()
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatal("completion lost original cancellation", err)
					}
				case <-time.After(200 * time.Millisecond):
					t.Fatal("completion ignored cancellation until the original deadline")
				}
				return
			}
			if name == "owner-death" {
				if err := connection.Close(); err != nil {
					t.Fatal(err)
				}
				if err := <-done; err == nil {
					t.Fatal("owner death opened admission")
				}
				if _, err := readStartGuard(root); err != nil {
					t.Fatal("owner death lost provenance", err)
				}
				return
			}
			if err := sendStartCompletion(ctx, connection, selected, invocation); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal("bound explicit completion refused", err)
			}
			if err := clearStartGuard(root, intent); err != nil {
				t.Fatal(err)
			}
			if err := refusePendingTransition(root); err != nil {
				t.Fatal("completed guard blocked ordinary restart", err)
			}
		})
	}
}

func TestStartCompletionWithUnprivilegedEndpointProcess(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	const endpointID = 65534
	root, err := os.MkdirTemp("/run", "ardents-start-component-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error("component fixture cleanup", err)
		}
	})
	if err := os.Chown(root, 0, endpointID); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0750); err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	childProgram := filepath.Join(root, "endpoint-component.test")
	if err := writeExclusiveGenerationFile(childProgram, body, 0555, endpointID); err != nil {
		t.Fatal(err)
	}
	selected := selection{GenerationDigest: digestHex([]byte("candidate")), BindingDigest: digestHex([]byte("binding"))}
	intent := transitionIntent{Request: Request{InstallationRoot: root}, Candidate: selected, CandidateBinding: localBinding{GID: endpointID}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	completion, err := prepareStartCompletion(ctx, root, intent, endpointID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := completion.close(); err != nil {
			t.Error(err)
		}
	}()
	child := exec.CommandContext(ctx, childProgram, "-test.run=^TestStartCompletionUnprivilegedChild$")
	var childOutput bytes.Buffer
	child.Stdout, child.Stderr = &childOutput, &childOutput
	child.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "ARDENTS_COMPLETION_CHILD_ROOT=" + root}
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: endpointID, Gid: endpointID}}
	if err := child.Start(); err != nil {
		t.Fatal("invalid component credential environment", err)
	}
	joined := false
	defer func() {
		if !joined {
			cancel()
			if err := child.Wait(); err != nil {
				t.Log("refused component child joined", err, childOutput.String())
			}
		}
	}()
	invocation := [16]byte{1}
	connection, err := completion.accept(ctx, selected, uint32(child.Process.Pid), endpointID, invocation)
	if err != nil {
		t.Fatal("actual unprivileged Endpoint connection refused", err)
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := sendStartCompletion(ctx, connection, selected, invocation); err != nil {
		t.Fatal(err)
	}
	err = child.Wait()
	joined = true
	if err != nil {
		t.Fatal("unprivileged completion consumer failed", err, childOutput.String())
	}
	if err := clearStartGuard(root, intent); err != nil {
		t.Fatal(err)
	}
}

func TestStartCompletionUnprivilegedChild(t *testing.T) {
	root := os.Getenv("ARDENTS_COMPLETION_CHILD_ROOT")
	if root == "" {
		return
	}
	if os.Geteuid() != 65534 || os.Getegid() != 65534 {
		t.Fatal("component child did not change actual kernel credentials")
	}
	groups, err := os.Getgroups()
	if err != nil || len(groups) != 0 {
		t.Fatal("component child retained supplementary groups", groups, err)
	}
	if _, err := readStartGuard(root); err == nil {
		t.Fatal("Endpoint component could read Root-private recovery intent")
	}
	if _, err := os.ReadFile(filepath.Join(root, "start-guard.json")); !errors.Is(err, os.ErrPermission) {
		t.Fatal("private intent refusal was not enforced by kernel permissions", err)
	}
	file, err := os.OpenFile(filepath.Join(root, "endpoint-created"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if file != nil {
		if closeErr := file.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatal("Endpoint could mutate the Root-controlled completion directory", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	selected := selection{GenerationDigest: digestHex([]byte("candidate")), BindingDigest: digestHex([]byte("binding"))}
	if err := awaitStartCompletion(ctx, root, selected, [16]byte{1}); err != nil {
		t.Fatal(err)
	}
}
