//go:build linux

package processfixture

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
)

// ReceivingSpendEvidence retains copies of actually admitted tokens for a joined reopen check.
type ReceivingSpendEvidence struct {
	owner   *receiving.Owner
	root    string
	binding receiving.Receiver
	observe func() (receiving.Observation, error)
	mu      sync.Mutex
	tokens  []struct {
		class admission.Class
		raw   []byte
	}
}

// ObserveReceivingSpends records admitted tokens against the original receiving owner.
func ObserveReceivingSpends(t testing.TB, owner *receiving.Owner, root string, binding receiving.Receiver, observe func() (receiving.Observation, error)) *ReceivingSpendEvidence {
	t.Helper()
	spent := &ReceivingSpendEvidence{owner: owner, root: root, binding: binding, observe: observe}
	t.Cleanup(func() {
		spent.mu.Lock()
		defer spent.mu.Unlock()
		for _, token := range spent.tokens {
			clear(token.raw)
		}
	})
	return spent
}

// Record copies a token after the original receiving owner admitted it.
func (spent *ReceivingSpendEvidence) Record(class admission.Class, raw []byte) {
	spent.mu.Lock()
	spent.tokens = append(spent.tokens, struct {
		class admission.Class
		raw   []byte
	}{class: class, raw: bytes.Clone(raw)})
	spent.mu.Unlock()
}

// RequireRetained rejoins the durable journal and requires replay refusal with balanced reservation accounting.
func (spent *ReceivingSpendEvidence) RequireRetained(t testing.TB, ctx context.Context, reserve func(time.Time) (func() error, error)) int32 {
	t.Helper()
	var replayed int32
	if err := spent.owner.Close(); err != nil {
		t.Fatal("joined original spend owner", err)
	}
	reopened, err := receiving.Open(spent.root, spent.binding, spent.observe)
	if err != nil {
		t.Fatal("retained spend reopen", err)
	}
	var closed bool
	defer func() {
		if !closed {
			if err := reopened.Close(); err != nil {
				t.Error("close reopened receiving owner:", err)
			}
		}
	}()
	spent.mu.Lock()
	tokens := append(spent.tokens[:0:0], spent.tokens...)
	spent.mu.Unlock()
	for _, token := range tokens {
		current, err := spent.observe()
		if err != nil {
			t.Fatal("replay current signed authority", err)
		}
		if _, err := receiving.VerifyToken(current.Profile, spent.binding, token.class, token.raw, current.Now); err != nil {
			t.Fatal("replay must still be a valid signed token", err)
		}
		reserved, returned := 0, 0
		deadline := current.Now.Add(5 * time.Second)
		grant, replayErr := reopened.Accept(ctx, token.class, token.raw, deadline, func() (func() error, error) {
			reserved++
			release, err := reserve(deadline)
			if err != nil {
				return nil, err
			}
			return func() error { returned++; return release() }, nil
		})
		if releaseErr := grant.Release(); releaseErr != nil {
			t.Fatal("replay grant cleanup", releaseErr)
		}
		if replayErr == nil || !strings.Contains(replayErr.Error(), "closed token is already spent") || reserved != 1 || returned != 1 {
			t.Fatalf("reopened replay = %v, reservation %d/%d", replayErr, reserved, returned)
		}
		replayed++
	}
	err = reopened.Close()
	closed = true
	if err != nil {
		t.Fatal("joined reopened spend owner", err)
	}
	return replayed
}
