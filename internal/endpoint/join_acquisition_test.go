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
	"github.com/dianabuilds/ardents-network/internal/qualification"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

type joinRetiredAfterRecipient struct {
	node       [32]byte
	generation uint64
	deadline   time.Time
	current    bool
}

func (acquisition *joinRetiredAfterRecipient) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	acquisition.current = false
	return acquisition.node, acquisition.generation, acquisition.deadline, nil
}

func (*joinRetiredAfterRecipient) join(context.Context, client.ClosedTokenPresenter,
	client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	return nil, errors.New("retired JOIN acquisition used")
}

func (acquisition *joinRetiredAfterRecipient) currentLocked(*textContext) bool {
	return acquisition.current
}

func (acquisition *joinRetiredAfterRecipient) issuancePrefixLocked(*textContext) (*sourceHandle, bool) {
	return nil, acquisition.current
}

func (*joinRetiredAfterRecipient) release() {}

func TestTextJoinedTransportCloseReleasesSourceAcquisition(t *testing.T) {
	lifecycle := &sourceLifecycle{}
	handle := &sourceHandle{owner: lifecycle, cancel: func() {}}
	handle.prefix.Store(&client.ClosedSourcePrefix{})
	lifecycle.live = handle
	acquisition := lifecycle.acquireJoinLocked()
	local, remote := net.Pipe()
	t.Cleanup(func() { _ = remote.Close() })
	transport := &joinedTransport{acquisition: acquisition, Conn: local, stop: func() {}, finish: func(err error) error { return err }}
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
	job := liveCapsuleJob(t, owner)
	run, _ := qualification.NewRun(streamqualification.ReaderRole, streamqualification.ClientToPublisher, fixtureID(210))
	job.qualification = run
	attempt := &introductionAttempt{binding: &serviceBinding{owner: owner, job: job}}

	owner.mu.Lock()
	old := &sourceHandle{owner: &owner.source, cancel: func() {}}
	old.prefix.Store(&client.ClosedSourcePrefix{})
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
	_, flight, detach, finish, err := owner.beginServiceTransportExchange(caller, job, broker.Connection)
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

	replacement := &sourceHandle{owner: &owner.source, cancel: func() {}}
	replacement.prefix.Store(&client.ClosedSourcePrefix{})
	owner.mu.Lock()
	old.prefix.Store(nil)
	owner.source.live = replacement
	owner.mu.Unlock()

	joined := &client.ClosedJoinedStream{}
	if owner.retainJoinedTransport(job, attempt, flight, acquisition, joined) {
		t.Fatal("late old JOIN acquisition attached after Source replacement")
	}
	attached := job.qualification.Retains(joined)
	owner.mu.Lock()
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
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	node := source.view.Nodes[4]
	deadline := time.Now().Add(time.Minute)
	acquisition := &joinRetiredAfterRecipient{node: node.NodeID, generation: node.DutyGeneration,
		deadline: deadline.Add(time.Minute), current: true}
	attempt := &introductionAttempt{plaintext: introductioncapsule.Plaintext{
		RendezvousNode: node.NodeID, RendezvousDutyGeneration: node.DutyGeneration, Deadline: deadline,
	}}
	owner.mu.Lock()
	reserved := owner.tokens.permission.reserved
	owner.mu.Unlock()
	if err := owner.prepareJoinStock(t.Context(), attempt, acquisition); err == nil {
		t.Fatal("retired JOIN acquisition prepared stock")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.tokens.permission.reserved != reserved || owner.tokens.permission.pending != nil || owner.tokens.issuance != nil {
		t.Fatal("retired JOIN acquisition reserved allocation or started issuance")
	}
}

func TestTextPublisherJoinIssuanceRetainsExactLiveSource(t *testing.T) {
	owner := &textContext{textContextState: textContextState{surface: broker.Administration}}
	responder := &responderPrefixHandle{rolePrefixHandleCore: rolePrefixHandleCore{owner: &owner.responder.rolePrefixCore, cancel: func() {}}}
	responder.prefix.Store(&client.ClosedSourcePrefix{})
	issuer := &sourceHandle{owner: &owner.source, cancel: func() {}}
	issuer.prefix.Store(&client.ClosedSourcePrefix{})
	owner.responder.live = responder
	owner.source.live = issuer
	acquisition := owner.responder.acquireJoinLocked(issuer)
	if expected, current := acquisition.issuancePrefixLocked(owner); !current || expected != issuer ||
		!joinIssuanceCurrentLocked(owner, acquisition, expected) {
		t.Fatal("Publisher JOIN issuance rejected its retained live Source")
	}
	replacement := &sourceHandle{owner: &owner.source, cancel: func() {}}
	replacement.prefix.Store(&client.ClosedSourcePrefix{})
	owner.source.live = replacement
	issuer.prefix.Store(nil)
	if acquisition.currentLocked(owner) {
		t.Fatal("Publisher JOIN acquisition accepted replacement Source on stocked path")
	}
	if expected, current := acquisition.issuancePrefixLocked(owner); current || expected != issuer ||
		joinIssuanceCurrentLocked(owner, acquisition, expected) {
		t.Fatal("Publisher JOIN issuance accepted a replacement Source")
	}
}
