//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
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
