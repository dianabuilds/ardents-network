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
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
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

func (acquisition *joinRetiredAfterRecipient) DataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	acquisition.current = false
	return acquisition.node, acquisition.generation, acquisition.deadline, nil
}

func (*joinRetiredAfterRecipient) Join(context.Context, client.ClosedTokenPresenter,
	client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	return nil, errors.New("retired JOIN acquisition used")
}

func (acquisition *joinRetiredAfterRecipient) CurrentLocked(*dutyContext) bool {
	return acquisition.current
}

func (acquisition *joinRetiredAfterRecipient) IssuancePrefixLocked(*dutyContext) (*source.Handle, bool) {
	return nil, acquisition.current
}

func (*joinRetiredAfterRecipient) Release() {}

func TestTextJoinedTransportCloseReleasesSourceAcquisition(t *testing.T) {
	lifecycle := &source.Lifecycle{}
	plantSourceHandle(lifecycle)
	acquisition := sourceJoinAcquisition{lifecycle.AcquireJoinLocked()}
	local, remote := net.Pipe()
	t.Cleanup(func() { _ = remote.Close() })
	transport := &joinedTransport{acquisition: acquisition, Conn: local, stop: func() {}, finish: func(err error) error { return err }}
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := acquisition.DataJoinRecipient(); err == nil {
		t.Fatal("joined transport close retained its Source acquisition")
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("repeated joined transport close changed its outcome: %v", err)
	}
}

func TestTextJoinOldAcquisitionCannotAttachAfterSourceReplacement(t *testing.T) {
	endpoint, principal := dutyContextEndpoint(t)
	owner := admittedDutyContext(t, endpoint, principal, broker.Connection)
	job := liveCapsuleJob(t, owner)
	run, _ := qualification.NewRun(streamqualification.ReaderRole, streamqualification.ClientToPublisher, fixtureID(210))
	job.qualification = run
	attempt := &introductionAttempt{binding: &serviceBinding{owner: owner, job: job}}

	owner.mu.Lock()
	plantSourceHandle(&owner.source)
	join := owner.source.AcquireJoinLocked()
	acquisition := sourceJoinAcquisition{join}
	owner.mu.Unlock()
	if join == nil {
		t.Fatal("JOIN acquisition unavailable")
	}
	t.Cleanup(func() {
		owner.mu.Lock()
		source.TransplantLive(&owner.source, nil)
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
			acquisition.Release()
		}
	}()
	if !detach() {
		t.Fatal("JOIN caller did not transfer its cleanup lifetime")
	}

	owner.mu.Lock()
	source.TransplantLive(&owner.source, nil)
	plantSourceHandle(&owner.source)
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
	acquisition.Release()
	finished = true
	if _, _, _, err := acquisition.DataJoinRecipient(); err == nil {
		t.Fatal("refused JOIN completion retained its acquisition")
	}
	owner.mu.Lock()
	source.TransplantLive(&owner.source, nil)
	owner.mu.Unlock()
}

func TestTextJoinSourceReplacementBeforeStockIssuanceDoesNotReserveAllocation(t *testing.T) {
	_, owner, sourceState := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, sourceState)
	node := sourceState.view.Nodes[4]
	deadline := time.Now().Add(time.Minute)
	acquisition := &joinRetiredAfterRecipient{node: node.NodeID, generation: node.DutyGeneration,
		deadline: deadline.Add(time.Minute), current: true}
	attempt := &introductionAttempt{plaintext: introductioncapsule.Plaintext{
		RendezvousNode: node.NodeID, RendezvousDutyGeneration: node.DutyGeneration, Deadline: deadline,
	}}
	owner.mu.Lock()
	reserved := owner.tokens.Permission.Reserved
	owner.mu.Unlock()
	if err := owner.prepareJoinStock(t.Context(), attempt, acquisition); err == nil {
		t.Fatal("retired JOIN acquisition prepared stock")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.tokens.Permission.Reserved != reserved || owner.tokens.Permission.Pending != nil || owner.tokens.Issuance != nil {
		t.Fatal("retired JOIN acquisition reserved allocation or started issuance")
	}
}

func TestTextPublisherJoinIssuanceRetainsExactLiveSource(t *testing.T) {
	owner := &dutyContext{dutyContextState: dutyContextState{surface: broker.Administration}}
	responder := &responderPrefixHandle{rolePrefixHandleCore: rolePrefixHandleCore{owner: &owner.responder.rolePrefixCore, cancel: func() {}}}
	responder.prefix.Store(&client.ClosedSourcePrefix{})
	issuer := plantSourceHandle(&owner.source)
	owner.responder.live = responder
	acquisition := owner.responder.acquireJoinLocked(issuer)
	if expected, current := acquisition.IssuancePrefixLocked(owner); !current || expected != issuer ||
		!joinIssuanceCurrentLocked(owner, acquisition, expected) {
		t.Fatal("Publisher JOIN issuance rejected its retained live Source")
	}
	source.TransplantLive(&owner.source, nil)
	plantSourceHandle(&owner.source)
	if acquisition.CurrentLocked(owner) {
		t.Fatal("Publisher JOIN acquisition accepted replacement Source on stocked path")
	}
	if expected, current := acquisition.IssuancePrefixLocked(owner); current || expected != issuer ||
		joinIssuanceCurrentLocked(owner, acquisition, expected) {
		t.Fatal("Publisher JOIN issuance accepted a replacement Source")
	}
}
