package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"time"

	serviceconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

type endpointConnection struct {
	application net.Conn
	result      net.Conn
	route       net.Conn
	outbound    *OutboundConnectionRequest
	inbound     *InboundConnectionRequest
}

type endpointOutcome struct {
	result RuntimeResult
	err    error
}

func serveEndpointConnections(ctx context.Context, plan endpointPlan, endpoint connectionEndpoint,
	setup Setup, applications, results *net.UnixListener,
	openAttachment func(context.Context, serviceconnection.Recovery) (net.Conn, error), published PublicationResult,
	at time.Time, setupDeadline, lifetime time.Duration,
) (RuntimeResult, error) {
	maximum := int(plan.MaximumConnections)
	if maximum == 0 {
		maximum = 1
	}
	outcomes := make(chan endpointOutcome, maximum)
	var started int
	for ; started < maximum; started++ {
		connection, err := acceptEndpointConnection(ctx, plan, endpoint, setup, applications, results,
			openAttachment, at, setupDeadline)
		if err != nil {
			return collectEndpointOutcomes(outcomes, started, setup, published, err)
		}
		go runEndpointConnection(ctx, endpoint, setup, connection, published, lifetime, outcomes)
	}
	return collectEndpointOutcomes(outcomes, started, setup, published, nil)
}

func acceptEndpointConnection(ctx context.Context, plan endpointPlan, endpoint connectionEndpoint,
	setup Setup, applications, results *net.UnixListener,
	openAttachment func(context.Context, serviceconnection.Recovery) (net.Conn, error), at time.Time, deadline time.Duration,
) (endpointConnection, error) {
	application, err := applications.Accept()
	if err != nil {
		return endpointConnection{}, err
	}
	if err := acceptApplication(application, time.Now().Add(deadline)); err != nil {
		return endpointConnection{}, errors.Join(err, application.Close())
	}
	result, err := acceptResult(results, time.Now().Add(deadline))
	if err != nil {
		return endpointConnection{}, errors.Join(err, application.Close())
	}
	setup.Resources("accepted-ipc", 1)
	setup.Resources("application-accept", 1)
	setup.Resources("result-ipc", 1)
	fail := func(cause error) (endpointConnection, error) {
		setup.Resources("accepted-ipc", -1)
		setup.Resources("result-ipc", -1)
		return endpointConnection{}, errors.Join(cause, application.Close(), closeConnection(result))
	}
	route, err := openAttachment(ctx, serviceconnection.Recovery{Generation: 1, Deadline: time.Now().Add(deadline)})
	if err != nil {
		return fail(err)
	}
	session, err := admit(endpoint, setup.ConnectionPrincipal, "connection", at)
	if err != nil {
		_ = route.Close()
		return fail(err)
	}
	binding, err := plan.recoveryBinding()
	if err != nil {
		_ = route.Close()
		return fail(err)
	}
	if plan.Role == "client" {
		request := OutboundConnectionRequest{Principal: setup.ConnectionPrincipal, Capability: session, Route: route,
			Application: application, RecoveryBinding: binding, BytesEachDirection: plan.BytesEachDirection,
			SendBytes: plan.SendBytes, ReceiveBytes: plan.ReceiveBytes, At: at}
		if plan.recoveryEnabled() {
			request.OpenAttachment = openAttachment
		}
		if err := addClientPublication(plan, &request); err != nil {
			_ = route.Close()
			return fail(err)
		}
		return endpointConnection{application: application, result: result, route: route, outbound: &request}, nil
	}
	request := InboundConnectionRequest{Principal: setup.ConnectionPrincipal, Capability: session, Route: route,
		Application: application, RecoveryBinding: binding, BytesEachDirection: plan.BytesEachDirection,
		SendBytes: plan.SendBytes, ReceiveBytes: plan.ReceiveBytes, At: at}
	if plan.recoveryEnabled() {
		request.OpenAttachment = openAttachment
	}
	return endpointConnection{application: application, result: result, route: route, inbound: &request}, nil
}

func runEndpointConnection(ctx context.Context, endpoint connectionEndpoint, setup Setup,
	connection endpointConnection, published PublicationResult, lifetime time.Duration, output chan<- endpointOutcome,
) {
	setup.Resources("timer", 1)
	defer setup.Resources("timer", -1)
	defer setup.Resources("accepted-ipc", -1)
	defer connection.application.Close()
	defer connection.route.Close()
	defer setup.Resources("result-ipc", -1)
	defer connection.result.Close()
	operation, cancel := context.WithTimeout(ctx, lifetime)
	defer cancel()
	var result RuntimeResult
	var err error
	if connection.outbound != nil {
		result, err = endpoint.Connect(operation, *connection.outbound)
	} else {
		result, err = endpoint.Accept(operation, *connection.inbound)
	}
	result.ApplicationIPCAccepts = setup.Resources("application-accept", 0)
	result.RouteAttachmentsAccepted = setup.Resources("route-attachment-accept", 0)
	err = errors.Join(err, connection.application.SetDeadline(time.Time{}))
	err = errors.Join(err, deliverResult(connection.result, result))
	output <- endpointOutcome{result: result, err: err}
}

func collectEndpointOutcomes(input <-chan endpointOutcome, count int, setup Setup,
	published PublicationResult, initial error,
) (RuntimeResult, error) {
	result := RuntimeResult{Class: "clean service connection close", AuthenticatedTarget: published.AuthenticatedTarget}
	err := initial
	var accepted, received uint64
	for index := range count {
		outcome := <-input
		err = errors.Join(err, outcome.err)
		if index == 0 || outcome.err != nil {
			result = outcome.result
		}
		accepted += uint64(outcome.result.AcceptedBytes)
		received += uint64(outcome.result.ReceivedBytes)
	}
	if accepted > uint64(^uint32(0)) || received > uint64(^uint32(0)) {
		return result, errors.Join(err, errors.New("aggregate Service Connection evidence exceeds its bound"))
	}
	result.AcceptedBytes, result.ReceivedBytes = uint32(accepted), uint32(received)
	result.ApplicationIPCAccepts = setup.Resources("application-accept", 0)
	result.RouteAttachmentsAccepted = setup.Resources("route-attachment-accept", 0)
	return result, err
}

func addClientPublication(plan endpointPlan, request *OutboundConnectionRequest) error {
	if plan.Role != "client" {
		return nil
	}
	if err := decodeEndpointFixedHex(plan.Target, request.Target[:]); err != nil {
		return err
	}
	file, err := os.Open(plan.PublicationFile)
	if err != nil {
		return err
	}
	request.Publication, err = io.ReadAll(io.LimitReader(file, 4<<10+1))
	closeErr := file.Close()
	if err != nil || closeErr != nil || len(request.Publication) > 4<<10 {
		return errors.Join(err, closeErr, errors.New("publication file is invalid or oversized"))
	}
	return nil
}

func closeConnection(connection net.Conn) error {
	if connection == nil {
		return nil
	}
	return connection.Close()
}
