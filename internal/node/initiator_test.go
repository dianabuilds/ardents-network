package node

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestInitiatorRelaysOnlyAfterExactSetupAndReady(t *testing.T) {
	rendezvous, material, rendezvousConfig := rendezvousFixture(t)
	attachment := [32]byte{21}
	responder, err := openRendezvousLeg(t.Context(), rendezvousConfig.ListenAddress, material.responder, material.serverPublic,
		legFor(material, attachment, route.ResponderRole, rendezvousConfig.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	presentation := entry.Presentation{InviteID: [32]byte{22}, Invite: []byte{2, 4, 6, 8}}
	initiator, err := StartInitiator(InitiatorConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: material.initiator,
		NetworkID: rendezvousConfig.NetworkID, EpochDigest: rendezvousConfig.EpochDigest, NodeID: [32]byte{4},
		NodePublicKey: material.initiatorPublic, Epoch: rendezvousConfig.Epoch, NotAfter: rendezvousConfig.NotAfter,
		Rendezvous: InitiatorPeer{NodeID: rendezvousConfig.NodeID, PublicKey: material.serverPublic, Endpoint: rendezvousConfig.ListenAddress},
		Admit:      initiatorAdmission(presentation, attachment, rendezvousConfig), HandshakeLimit: 2, RelayLimit: 1,
		RelayByteLimit: 1024, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	candidate := entry.Candidate{NodeID: [32]byte{4}, PublicKey: material.initiatorPublic, Endpoint: initiator.listener.Addr().String()}
	acquirer := initiatorEntryAcquirer{candidate: candidate, presentation: presentation}
	connection, cleanup, err := route.OpenEntryAttachment(t.Context(), acquirer, route.EntryAttachmentRequest{
		NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, Epoch: rendezvousConfig.Epoch,
		AttachmentID: attachment, Deadline: rendezvousConfig.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	setup := route.RelaySetup{NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, AttachmentID: attachment,
		Epoch: rendezvousConfig.Epoch, TransitRole: route.InitiatorRole, NextRole: route.RendezvousRole,
		TransitNodeID: [32]byte{4}, NextNodeID: rendezvousConfig.NodeID, NextNodePublicKey: material.serverPublic,
		NotAfter: rendezvousConfig.NotAfter}
	if err := route.WriteRelaySetup(connection, setup); err != nil {
		t.Fatal(err)
	}
	ready, err := route.ReadRelayReady(connection)
	if err != nil || setup.VerifyRelayReady(ready) != nil {
		t.Fatalf("RelayReady err=%v verify=%v", err, setup.VerifyRelayReady(ready))
	}
	if _, err := connection.Write([]byte("from user")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, responder, len("from user")); string(got) != "from user" {
		t.Fatalf("Responder bytes = %q", got)
	}
	if _, err := responder.Write([]byte("from responder")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, connection, len("from responder")); string(got) != "from responder" {
		t.Fatalf("User bytes = %q", got)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := initiator.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if usage := initiator.Usage(); usage.ActiveRelays != 0 || usage.Connections != 0 || usage.CompletedRelays != 1 {
		t.Fatalf("Initiator terminal usage = %+v", usage)
	}
	if err := rendezvous.Drain(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestInitiatorDutyUsesOnlyStateAssignedRendezvous(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 31, "state-initiator")
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := dutyFacts{NetworkID: [32]byte{31}, Epoch: 32, Digest: [32]byte{33}, Profile: route.Profile,
		NodeID: [32]byte{34}, NodePublicKey: public, Assignment: "initiator", ProbeEndpoint: "127.0.0.1:30234",
		EpochValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(time.Minute),
		Candidates: [64]dutyCandidate{{NodeID: [32]byte{35}, PublicKey: [32]byte{36}, Endpoint: "127.0.0.1:30235",
			Assignment: "rendezvous", ValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute)}}, CandidateCount: 1}
	profile := InitiatorProfile{Certificate: certificate, Admit: func([]byte, [32]byte, [32]byte, time.Time) (route.EntryAdmission, error) {
		return route.EntryAdmission{}, nil
	}, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 1024, DrainTimeout: time.Second}
	plan, err := initiatorDuty(profile, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rendezvous.NodeID != snapshot.Candidates[0].NodeID || plan.Rendezvous.PublicKey != snapshot.Candidates[0].PublicKey ||
		plan.Rendezvous.Endpoint != snapshot.Candidates[0].Endpoint {
		t.Fatalf("Initiator State duty peer = %+v", plan.Rendezvous)
	}
	snapshot.Candidates[1] = dutyCandidate{NodeID: [32]byte{37}, PublicKey: [32]byte{38}, Endpoint: "127.0.0.1:30236",
		Assignment: "rendezvous", ValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute)}
	snapshot.CandidateCount = 2
	if _, err := initiatorDuty(profile, snapshot); err == nil {
		t.Fatal("Initiator accepted an ambiguous State Rendezvous peer set")
	}
}

func initiatorAdmission(presentation entry.Presentation, attachment [32]byte, config RendezvousConfig) route.EntryBindingAdmitter {
	return func(invite []byte, received, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
		if string(invite) != string(presentation.Invite) || received != attachment || key == [32]byte{} || !notAfter.Equal(config.NotAfter) {
			return route.EntryAdmission{}, errors.New("unexpected Entry admission")
		}
		return route.EntryAdmission{InviteID: presentation.InviteID, NetworkID: config.NetworkID, Digest: config.EpochDigest,
			Epoch: config.Epoch, InitiatorNodeID: [32]byte{4}, NotAfter: config.NotAfter}, nil
	}
}

type initiatorEntryAcquirer struct {
	candidate    entry.Candidate
	presentation entry.Presentation
}

func (input initiatorEntryAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}
