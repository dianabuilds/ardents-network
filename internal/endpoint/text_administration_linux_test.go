//go:build linux

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

func TestTextAdministrationRequiresCurrentSeparateAuthority(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	reader := admittedTextContext(t, endpoint, principal, broker.Connection)
	if _, err := reader.openTextAdministration(); err == nil {
		t.Fatal("Connection context acquired Service Administration")
	}
	publisher := admittedTextContext(t, endpoint, principal, broker.Administration)
	administration, err := publisher.openTextAdministration()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := administration.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := administration.Publish(t.Context()); err == nil {
		t.Fatal("bodyless publication accepted an implicit document")
	}
	if err := endpoint.admission.Revoke(principal, broker.Administration); err != nil {
		t.Fatal(err)
	}
	if err := administration.PublishSnapshot(t.Context(), []byte("document")); err == nil {
		t.Fatal("revoked Administration published")
	}
	if err := administration.Withdraw(t.Context()); err == nil {
		t.Fatal("revoked Administration withdrew")
	}
	if err := administration.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTextAdministrationRefusesCancelledAndInvalidSnapshots(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	publisher := admittedTextContext(t, endpoint, principal, broker.Administration)
	administration, err := publisher.openTextAdministration()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := administration.Close(); err != nil {
			t.Error(err)
		}
	})
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	if err := administration.PublishSnapshot(caller, nil); err == nil {
		t.Fatal("cancelled publication succeeded")
	}
	if err := administration.PublishSnapshot(t.Context(), []byte{0xff}); err == nil {
		t.Fatal("invalid UTF-8 published")
	}
	if err := administration.PublishSnapshot(t.Context(), make([]byte, (4<<20)+1)); err == nil {
		t.Fatal("oversized document published")
	}
	if err := administration.Close(); err != nil {
		t.Fatal(err)
	}
	if err := administration.PublishSnapshot(t.Context(), nil); err == nil {
		t.Fatal("closed Administration published")
	}
}

// Hold the real Endpoint launch gate before any host activation. Cancellation
// must join that pending launch; the fixture never supplies isolation success.
func TestTextAdministrationJoinsStartupBeforeWithdrawalOrClose(t *testing.T) {
	for _, action := range []string{"withdraw", "close"} {
		t.Run(action, func(t *testing.T) {
			endpoint, principal := textContextEndpoint(t)
			publisher := admittedTextContext(t, endpoint, principal, broker.Administration)
			owner, err := publisher.openTextAdministration()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := owner.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := owner.Withdraw(t.Context()); err == nil {
				t.Fatal("idle Administration reported withdrawal")
			}
			release, err := endpoint.acquireTextLaunch(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			published := make(chan error, 1)
			go func() { published <- owner.PublishSnapshot(t.Context(), []byte("document")) }()
			deadline := time.Now().Add(5 * time.Second)
			for {
				publisher.mu.Lock()
				job := publisher.job
				publisher.mu.Unlock()
				if job != nil {
					break
				}
				select {
				case err := <-published:
					t.Fatalf("startup did not reserve launch: %v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("startup did not reach held launch gate")
				}
				time.Sleep(time.Millisecond)
			}
			if action == "withdraw" {
				if err := owner.Withdraw(t.Context()); err == nil {
					t.Fatal("uncommitted publication reported withdrawn")
				}
			} else if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if err := <-published; err == nil {
				t.Fatal("cancelled startup reported published")
			}
			publisher.mu.Lock()
			retained := publisher.job
			publisher.mu.Unlock()
			owner.mu.Lock()
			late := owner.run
			owner.mu.Unlock()
			if late != nil {
				t.Fatal("cancelled startup retained a late publication")
			}
			if retained != nil {
				t.Fatal("terminal operation returned before launch retirement")
			}
		})
	}
}
