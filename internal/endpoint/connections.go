package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type endpointConnection struct {
	application net.Conn
	result      net.Conn
	route       net.Conn
	request     serviceconn.Request
}

type endpointOutcome struct {
	result serviceconn.Result
	err    error
}

func serveEndpointConnections(ctx context.Context, plan endpointPlan, endpoint connectionEndpoint,
	setup serviceconn.Setup, applications, results *net.UnixListener,
	openAttachment func(context.Context, serviceconn.Recovery) (net.Conn, error), published serviceconn.Result,
	at time.Time, setupDeadline, lifetime time.Duration,
) (serviceconn.Result, error) {
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
	setup serviceconn.Setup, applications, results *net.UnixListener,
	openAttachment func(context.Context, serviceconn.Recovery) (net.Conn, error), at time.Time, deadline time.Duration,
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
	route, err := openAttachment(ctx, serviceconn.Recovery{Generation: 1, Deadline: time.Now().Add(deadline)})
	if err != nil {
		return fail(err)
	}
	session, err := admit(endpoint, setup.ConnectionPrincipal, "connection", at)
	if err != nil {
		_ = route.Close()
		return fail(err)
	}
	request := serviceconn.Request{Action: plan.roleAction(), Principal: setup.ConnectionPrincipal,
		Session: session, Route: route, Application: application, BytesEachDirection: plan.BytesEachDirection,
		SendBytes: plan.SendBytes, ReceiveBytes: plan.ReceiveBytes, At: at}
	request.RecoveryBinding, err = plan.recoveryBinding()
	if err != nil {
		_ = route.Close()
		return fail(err)
	}
	if plan.recoveryEnabled() {
		request.OpenAttachment = openAttachment
	}
	if err := addClientPublication(plan, &request); err != nil {
		_ = route.Close()
		return fail(err)
	}
	return endpointConnection{application: application, result: result, route: route, request: request}, nil
}

func runEndpointConnection(ctx context.Context, endpoint connectionEndpoint, setup serviceconn.Setup,
	connection endpointConnection, published serviceconn.Result, lifetime time.Duration, output chan<- endpointOutcome,
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
	result, err := endpoint.Do(operation, connection.request)
	result.ApplicationIPCAccepts = setup.Resources("application-accept", 0)
	result.RouteAttachmentsAccepted = setup.Resources("route-attachment-accept", 0)
	result.IntroductionReceipt = published.IntroductionReceipt
	result.IntroductionAcknowledgement = published.IntroductionAcknowledgement
	err = errors.Join(err, connection.application.SetDeadline(time.Time{}))
	err = errors.Join(err, deliverResult(connection.result, result))
	output <- endpointOutcome{result: result, err: err}
}

func collectEndpointOutcomes(input <-chan endpointOutcome, count int, setup serviceconn.Setup,
	published serviceconn.Result, initial error,
) (serviceconn.Result, error) {
	result := serviceconn.Result{Class: "clean service connection close", AuthenticatedTarget: published.AuthenticatedTarget,
		IntroductionReceipt:         published.IntroductionReceipt,
		IntroductionAcknowledgement: published.IntroductionAcknowledgement}
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

func addClientPublication(plan endpointPlan, request *serviceconn.Request) error {
	if plan.Role != "client" {
		return nil
	}
	if err := planfile.FixedHex(plan.Target, request.Target[:]); err != nil {
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
