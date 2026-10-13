package connection

import (
	"context"
	"errors"
	"testing"
	"time"
)

// This invalid-role retirement fixture has no successful Operation, Instance,
// JOIN or TLS. It isolates synchronous cancellation before a blocked join lock.
func TestLogicalRetirementDeniesBeforeWaitingForOwnerLock(t *testing.T) {
	logical, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner := &AuthenticatedAttachment{logicalContext: logical, cancelLogical: cancel}
	owner.self = owner
	owner.mu.Lock()
	joined := make(chan error, 1)
	go func() { joined <- owner.Close() }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	denied := false
	select {
	case <-logical.Done():
		denied = true
	case <-timer.C:
	}
	// Always release and join the original failed owner, including a failed
	// cancellation oracle. A failed assertion must not leave a goroutine behind.
	owner.mu.Unlock()
	result := <-joined
	if !denied || !errors.Is(result, errIntegrity) {
		t.Fatal("logical retirement waited for physical owner lock before denial", result)
	}
	if repeated := owner.Close(); !errors.Is(repeated, errIntegrity) {
		t.Fatal("original failed retirement result was lost", repeated)
	}
}
