//go:build linux

package endpoint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// Pause the real resolution server at its State recheck after the actual
// Store has written revision 2 and before it can emit the Descriptor RESULT.
// Concurrent resolution State polls may also wait. State facts, Store writes,
// token admission, registration and all network replies remain real owners.
type textDescriptorACKGate struct {
	mu       sync.Mutex
	root     string
	baseline map[string]os.FileInfo
	held     chan struct{}
	release  chan struct{}
	once     sync.Once
	released sync.Once
}

func newTextDescriptorACKGate() *textDescriptorACKGate {
	return &textDescriptorACKGate{held: make(chan struct{}), release: make(chan struct{})}
}

func (gate *textDescriptorACKGate) open() { gate.released.Do(func() { close(gate.release) }) }

func (gate *textDescriptorACKGate) configure(t *testing.T) func(int, *node.Config) {
	return func(index int, config *node.Config) {
		if index != 5 {
			return
		}
		current := config.CurrentClosedRoute
		root := config.ClosedResolution.Root
		gate.root = root
		config.CurrentClosedRoute = func() (state.ClosedRouteView, bool) {
			view, ok := current()
			if ok && gate.committed(root, view.Profile) {
				gate.once.Do(func() { close(gate.held) })
				select {
				case <-gate.release:
				case <-t.Context().Done():
				}
			}
			return view, ok
		}
	}
}

func (gate *textDescriptorACKGate) committed(root string, profile state.ClosedProfileView) bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.baseline == nil {
		return false
	}
	entries, err := os.ReadDir(filepath.Join(root, "records"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if len(entry.Name()) != 64 {
			continue
		}
		// Directory metadata does not hold the replaced record open. Read bytes
		// only after the first publication's file has changed; the barrier then
		// prevents any further replacement until this observation has finished.
		info, err := entry.Info()
		if err != nil {
			continue
		}
		prior := gate.baseline[entry.Name()]
		if prior != nil && prior.Size() == info.Size() && prior.ModTime().Equal(info.ModTime()) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, "records", entry.Name()))
		if err != nil || len(raw) < 3 {
			continue
		}
		proof, err := reachability.VerifyPrivatePublication(raw[2:], profile.NetworkID, profile.Digest, time.Now().UTC())
		if err == nil && proof.Descriptor.Private.Revision == 2 {
			return true
		}
	}
	return false
}

func deliverTextBeforeDescriptorACK(t *testing.T, gate *textDescriptorACKGate, reader, publisher *textContext, readerJob, publisherJob *textJobIdentity, prepared *textIntroductionAttempt) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	received, sent := make(chan error, 1), make(chan error, 1)
	go func() {
		accepted, err := publisher.receiveTextIntroduction(ctx, publisherJob)
		if err == nil && (accepted.digest != prepared.digest || accepted.plaintext != prepared.plaintext) {
			err = fmt.Errorf("accepted capsule changed")
		}
		received <- err
	}()
	go func() { sent <- reader.submitTextIntroduction(ctx, readerJob, prepared) }()
	select {
	case err := <-received:
		if err != nil {
			gate.open()
			<-sent
			t.Fatalf("old capsule before Descriptor ACK: %v", err)
		}
	case <-ctx.Done():
		// Release before joining the failing implementation's blocked mutex.
		gate.open()
		<-received
		<-sent
		t.Fatal("old capsule acceptance waited for replacement Descriptor ACK")
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
}

// A hostile submitter can see the committed public Descriptor before its
// Publisher receives the ACK. Build that candidate with actual Instance bytes;
// it must not gain local admission merely because its signature is valid.
func refuseTextBeforeDescriptorACK(t *testing.T, gate *textDescriptorACKGate, owner *textContext, job *textJobIdentity, prior *textIntroductionAttempt) []byte {
	t.Helper()
	owner.mu.Lock()
	registered := owner.registration
	raw := append([]byte(nil), registered.descriptor...)
	owner.mu.Unlock()
	proof, err := reachability.VerifyPrivatePublication(raw, prior.plaintext.Network, prior.plaintext.ProfileDigest, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	plaintext := prior.plaintext
	plaintext.Revision = proof.Descriptor.Private.Revision
	capsule := route.ClosedIntroductionCapsule{Slot: proof.Descriptor.Private.Slot, Revision: plaintext.Revision,
		Expiry: plaintext.Deadline, DeliveryNonce: fixtureID(231)}
	sealed, _, err := route.SealClosedIntroduction(capsule, proof.Descriptor.Private.RecipientKey, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := route.EncodeClosedIntroductionSubmission(fixtureID(232), sealed)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := owner.acceptTextIntroduction(ctx, job, operation)
		result <- err
	}()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("unacknowledged registration accepted a valid new capsule")
		}
	case <-ctx.Done():
		gate.open()
		<-result
		t.Fatal("unacknowledged capsule waited for ACK instead of refusing")
	}
	return operation
}

// Arm only after the first publication and before forcing its replacement.
// Polling the old record's contents races atomic replacement on Windows.
func (gate *textDescriptorACKGate) arm(t *testing.T) {
	t.Helper()
	gate.mu.Lock()
	defer gate.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(gate.root, "records"))
	if err != nil {
		t.Fatal(err)
	}
	gate.baseline = make(map[string]os.FileInfo)
	for _, entry := range entries {
		if len(entry.Name()) != 64 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		gate.baseline[entry.Name()] = info
	}
	if len(gate.baseline) != 1 {
		t.Fatalf("expected one committed publication, got %d", len(gate.baseline))
	}
}
