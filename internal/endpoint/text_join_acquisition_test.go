//go:build linux

package endpoint

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type textJoinRetiredAfterRecipient struct {
	node       [32]byte
	generation uint64
	deadline   time.Time
	current    bool
}

func (acquisition *textJoinRetiredAfterRecipient) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	acquisition.current = false
	return acquisition.node, acquisition.generation, acquisition.deadline, nil
}

func (*textJoinRetiredAfterRecipient) join(context.Context, route.ClosedTokenPresenter,
	route.ClosedJoinIntent) (*route.ClosedJoinedStream, error) {
	return nil, errors.New("retired JOIN acquisition used")
}

func (acquisition *textJoinRetiredAfterRecipient) currentLocked(*textContext) bool {
	return acquisition.current
}

func (acquisition *textJoinRetiredAfterRecipient) issuancePrefixLocked(*textContext) (*textSourceHandle, bool) {
	return nil, acquisition.current
}

func (*textJoinRetiredAfterRecipient) release() {}

func TestTextJoinedTransportCloseReleasesSourceAcquisition(t *testing.T) {
	lifecycle := &textSourceLifecycle{}
	handle := &textSourceHandle{owner: lifecycle, cancel: func() {}}
	handle.prefix.Store(&route.ClosedSourcePrefix{})
	lifecycle.live = handle
	acquisition := lifecycle.acquireJoinLocked()
	local, remote := net.Pipe()
	t.Cleanup(func() { _ = remote.Close() })
	transport := &textJoinedTransport{acquisition: acquisition, Conn: local, stop: func() {}, finish: func(err error) error { return err }}
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	if acquisition.handle.Load() != nil {
		t.Fatal("joined transport close retained its Source acquisition")
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("repeated joined transport close changed its outcome: %v", err)
	}
}

func TestTextJoinOldAcquisitionCannotAttachAfterSourceReplacement(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job := liveTextCapsuleJob(t, owner)
	job.qualification = &streamqualification.Init{Role: streamqualification.ReaderRole}
	attempt := &textIntroductionAttempt{binding: &textServiceBinding{owner: owner, job: job}}

	owner.mu.Lock()
	old := &textSourceHandle{owner: &owner.source, cancel: func() {}}
	old.prefix.Store(&route.ClosedSourcePrefix{})
	owner.source.live = old
	acquisition := owner.source.acquireJoinLocked()
	owner.mu.Unlock()
	if acquisition == nil {
		t.Fatal("JOIN acquisition unavailable")
	}
	t.Cleanup(func() {
		owner.mu.Lock()
		owner.source.live = nil
		old.prefix.Store(nil)
		owner.mu.Unlock()
	})

	caller, cancel := context.WithCancel(t.Context())
	_, flight, detach, finish, err := owner.beginTextServiceTransportExchange(caller, job, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			cancel()
			_ = finish(context.Canceled)
			acquisition.release()
		}
	}()
	if !detach() {
		t.Fatal("JOIN caller did not transfer its cleanup lifetime")
	}

	replacement := &textSourceHandle{owner: &owner.source, cancel: func() {}}
	replacement.prefix.Store(&route.ClosedSourcePrefix{})
	owner.mu.Lock()
	old.prefix.Store(nil)
	owner.source.live = replacement
	owner.mu.Unlock()

	joined := &route.ClosedJoinedStream{}
	if owner.retainTextJoinedTransport(job, attempt, flight, acquisition, joined) {
		t.Fatal("late old JOIN acquisition attached after Source replacement")
	}
	owner.mu.Lock()
	_, attached := job.qualificationJoins[joined]
	retained := flight.retained
	owner.mu.Unlock()
	if attached || retained {
		t.Fatalf("late old JOIN retained transport: attached=%v retained=%v", attached, retained)
	}
	cancel()
	_ = finish(context.Canceled)
	acquisition.release()
	finished = true
	if acquisition.handle.Load() != nil {
		t.Fatal("refused JOIN completion retained its acquisition")
	}
	owner.mu.Lock()
	owner.source.live = nil
	replacement.prefix.Store(nil)
	owner.mu.Unlock()
}

func TestTextJoinSourceReplacementBeforeStockIssuanceDoesNotReserveAllocation(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
	node := source.view.Nodes[4]
	deadline := time.Now().Add(time.Minute)
	acquisition := &textJoinRetiredAfterRecipient{node: node.NodeID, generation: node.DutyGeneration,
		deadline: deadline.Add(time.Minute), current: true}
	attempt := &textIntroductionAttempt{plaintext: route.ClosedIntroductionPlaintext{
		RendezvousNode: node.NodeID, RendezvousDutyGeneration: node.DutyGeneration, Deadline: deadline,
	}}
	owner.mu.Lock()
	reserved := owner.permission.reserved
	owner.mu.Unlock()
	if err := owner.prepareTextJoinStock(t.Context(), attempt, acquisition); err == nil {
		t.Fatal("retired JOIN acquisition prepared stock")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.permission.reserved != reserved || owner.permission.pending != nil || owner.issuance != nil {
		t.Fatal("retired JOIN acquisition reserved allocation or started issuance")
	}
}

func TestTextPublisherJoinIssuanceRetainsExactLiveSource(t *testing.T) {
	owner := &textContext{textContextState: textContextState{surface: broker.Administration}}
	responder := &route.ClosedSourcePrefix{}
	issuer := &textSourceHandle{owner: &owner.source, cancel: func() {}}
	issuer.prefix.Store(&route.ClosedSourcePrefix{})
	owner.responder.prefix = responder
	owner.source.live = issuer
	acquisition := textPublisherJoinPrefix{prefix: responder, issuer: issuer}
	if expected, current := acquisition.issuancePrefixLocked(owner); !current || expected != issuer ||
		!textJoinIssuanceCurrentLocked(owner, acquisition, expected) {
		t.Fatal("Publisher JOIN issuance rejected its retained live Source")
	}
	replacement := &textSourceHandle{owner: &owner.source, cancel: func() {}}
	replacement.prefix.Store(&route.ClosedSourcePrefix{})
	owner.source.live = replacement
	issuer.prefix.Store(nil)
	if expected, current := acquisition.issuancePrefixLocked(owner); current || expected != issuer ||
		textJoinIssuanceCurrentLocked(owner, acquisition, expected) {
		t.Fatal("Publisher JOIN issuance accepted a replacement Source")
	}
}
