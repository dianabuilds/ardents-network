package stage6evidence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"

	serviceconn "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

type connectionCellEvidence struct {
	Initial        namespace.Binding
	Replacement    namespace.Binding
	NameOrigin     bool
	ClientClass    string
	PublisherClass string
	Target         [32]byte
}

type connectionOutcome struct {
	role   string
	result serviceconn.RuntimeResult
	err    error
}

func runConnectionCell(trace *traceRecord) error {
	fixture, err := newConnectionFixture()
	if err != nil {
		return err
	}
	defer fixture.Close()
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 2, Lease: "active",
		Consistency: "current", Recovery: "stable", Authority: "authority", Target: fixture.credential.Target,
		LeaseExpiresAt: fixture.now.Add(time.Hour).Unix(), GraceExpiresAt: fixture.now.Add(2 * time.Hour).Unix(),
		RecordNotAfter: fixture.now.Add(30 * time.Minute).UnixMilli(), Continuity: 1}
	binding, _, err := namespace.ResolveBindingLegacy(record, fixture.now.Unix(), nil)
	if err != nil {
		return err
	}
	replacement := record
	replacement.Revision++
	replacement.Target = [32]byte{99}
	replacementBinding, _, err := namespace.ResolveBindingLegacy(replacement, fixture.now.Add(time.Second).Unix(), nil)
	if err != nil {
		return err
	}
	updates := make(chan serviceconn.DestinationBinding, 1)
	evidence := connectionCellEvidence{Initial: binding, Replacement: replacementBinding, Target: fixture.credential.Target}
	if trace.Cell == "C2" {
		evidence.NameOrigin = true
	}
	outcomes, applications, err := startEvidenceConnection(fixture, binding, updates, evidence.NameOrigin)
	if err != nil {
		return err
	}
	defer applications[0].Close()
	defer applications[1].Close()
	if evidence.NameOrigin {
		updates <- evidenceServiceBinding(replacementBinding)
	} else if err := exchangeEvidenceBytes(applications); err != nil {
		return err
	}
	for range 2 {
		select {
		case outcome := <-outcomes:
			if outcome.role == "client" {
				evidence.ClientClass = outcome.result.Class
			} else {
				evidence.PublisherClass = outcome.result.Class
			}
		case <-time.After(2 * time.Second):
			return errors.New("bounded connection cell did not terminate")
		}
	}
	if evidence.NameOrigin && evidence.ClientClass != "abrupt connection loss" {
		return errors.New("name-origin stream silently retargeted")
	}
	if !evidence.NameOrigin && evidence.ClientClass != "clean service connection close" {
		return errors.New("direct Target stream did not remain pinned")
	}
	trace.Input, err = packRecords([]namespace.Record{record})
	if err != nil {
		return err
	}
	trace.Output, err = packRecords([]namespace.Record{replacement})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	trace.Auxiliary = raw
	trace.Fields = []string{evidence.ClientClass, evidence.PublisherClass}
	return nil
}

func startEvidenceConnection(fixture connectionFixture, binding namespace.Binding, updates chan serviceconn.DestinationBinding,
	nameOrigin bool,
) (<-chan connectionOutcome, [2]net.Conn, error) {
	clientSession, err := admitConnection(fixture.clientEndpoint, "connection", fixture.client, fixture.now)
	if err != nil {
		return nil, [2]net.Conn{}, err
	}
	publisherSession, err := admitConnection(fixture.publisherNode, "connection", fixture.publisher, fixture.now)
	if err != nil {
		return nil, [2]net.Conn{}, err
	}
	clientRoute, publisherRoute := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	outcomes := make(chan connectionOutcome, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = cancel
	go runEvidenceInbound(ctx, outcomes, fixture.publisherNode, serviceconn.InboundConnectionRequest{
		Principal: fixture.publisher, Capability: publisherSession, Route: publisherRoute,
		Application: publisherEndpoint, BytesEachDirection: 1, At: fixture.now})
	request := serviceconn.OutboundConnectionRequest{Principal: fixture.client, Capability: clientSession,
		Target: fixture.credential.Target, Publication: fixture.publication, Route: clientRoute,
		Application: clientEndpoint, BytesEachDirection: 1, At: fixture.now}
	if nameOrigin {
		request.NameBinding, request.NameUpdates = evidenceServiceBinding(binding), updates
	}
	go runEvidenceOutbound(ctx, outcomes, fixture.clientEndpoint, request)
	return outcomes, [2]net.Conn{clientApplication, publisherApplication}, nil
}

func evidenceServiceBinding(value namespace.Binding) serviceconn.DestinationBinding {
	return serviceconn.DestinationBinding{Name: value.Name, Generation: value.Generation, Revision: value.Revision,
		Authority: value.Authority, Target: value.Target, ParentName: value.ParentName,
		ParentGeneration: value.ParentGeneration, RecordDigest: value.RecordDigest, Commitment: value.Commitment}
}

func runEvidenceInbound(ctx context.Context, outcomes chan<- connectionOutcome, endpoint evidenceEndpoint,
	request serviceconn.InboundConnectionRequest,
) {
	result, err := endpoint.Accept(ctx, request)
	outcomes <- connectionOutcome{"publisher", result, err}
}

func runEvidenceOutbound(ctx context.Context, outcomes chan<- connectionOutcome, endpoint evidenceEndpoint,
	request serviceconn.OutboundConnectionRequest,
) {
	result, err := endpoint.Connect(ctx, request)
	outcomes <- connectionOutcome{"client", result, err}
}

func exchangeEvidenceBytes(applications [2]net.Conn) error {
	errorsOut := make(chan error, 4)
	go func() { _, err := applications[0].Write([]byte{1}); errorsOut <- err }()
	go func() { _, err := applications[1].Write([]byte{2}); errorsOut <- err }()
	go func() { _, err := io.ReadFull(applications[0], make([]byte, 1)); errorsOut <- err }()
	go func() { _, err := io.ReadFull(applications[1], make([]byte, 1)); errorsOut <- err }()
	for range 4 {
		if err := <-errorsOut; err != nil {
			return err
		}
	}
	return nil
}
