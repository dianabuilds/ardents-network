package serviceendpoint

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

// Run loads one role-local plan and owns its Endpoint process composition.
func Run(ctx context.Context, planPath string, ready func(string)) (serviceconn.Result, error) {
	plan, err := readPlan(planPath)
	if err != nil {
		return serviceconn.Result{}, err
	}
	return runEndpoint(ctx, plan, func() {
		if ready != nil {
			ready(plan.Role)
		}
	})
}

func runEndpoint(ctx context.Context, plan endpointPlan, ready func()) (serviceconn.Result, error) {
	setup, at, deadline, err := endpointSetup(plan)
	if err != nil {
		return serviceconn.Result{}, err
	}
	endpoint, err := serviceconn.New(setup)
	if err != nil {
		return serviceconn.Result{}, err
	}
	applicationListener, err := listenLocal(plan.ApplicationSocket, deadline)
	if err != nil {
		return serviceconn.Result{}, err
	}
	setup.Resources("timer", 1)
	defer setup.Resources("timer", -1)
	setup.Resources("control-file", 1)
	defer setup.Resources("control-file", -1)
	defer func() { _ = applicationListener.Close(); _ = os.Remove(plan.ApplicationSocket) }()
	routeListener, err := listenLocal(plan.RouteSocket, deadline)
	if err != nil {
		return serviceconn.Result{}, err
	}
	setup.Resources("timer", 1)
	defer setup.Resources("timer", -1)
	setup.Resources("control-file", 1)
	defer setup.Resources("control-file", -1)
	defer func() { _ = routeListener.Close(); _ = os.Remove(plan.RouteSocket) }()
	var published serviceconn.Result
	if plan.Role == "publisher" {
		var publishErr error
		published, publishErr = publishCurrent(endpoint, setup.Resources, plan, setup.AdministrationPrincipal, at, deadline, ready)
		if publishErr != nil {
			return serviceconn.Result{}, publishErr
		}
		defer setup.Resources("control-file", -2)
	} else if ready != nil {
		ready()
	}
	application, err := applicationListener.Accept()
	if err != nil {
		return serviceconn.Result{}, err
	}
	setup.Resources("accepted-ipc", 1)
	setup.Resources("application-accept", 1)
	defer setup.Resources("accepted-ipc", -1)
	defer application.Close()
	openAttachment := routeAttachmentOpener(routeListener, setup.Resources)
	route, err := openAttachment(ctx, serviceconn.Recovery{Generation: 1, Deadline: time.Now().Add(deadline)})
	if err != nil {
		application.Close()
		return serviceconn.Result{}, err
	}
	defer route.Close()
	session, err := admit(endpoint, setup.ConnectionPrincipal, "connection", at)
	if err != nil {
		application.Close()
		route.Close()
		return serviceconn.Result{}, err
	}
	request := serviceconn.Request{Action: plan.roleAction(), Principal: setup.ConnectionPrincipal,
		Session: session, Route: route, Application: application,
		BytesEachDirection: plan.BytesEachDirection, SendBytes: plan.SendBytes, ReceiveBytes: plan.ReceiveBytes, At: at}
	request.RecoveryBinding, err = plan.recoveryBinding()
	if err != nil {
		return serviceconn.Result{}, err
	}
	if plan.recoveryEnabled() {
		request.OpenAttachment = openAttachment
	}
	if plan.Role == "client" {
		if err := planfile.FixedHex(plan.Target, request.Target[:]); err != nil {
			return serviceconn.Result{}, err
		}
		file, openErr := os.Open(plan.PublicationFile)
		if openErr != nil {
			return serviceconn.Result{}, openErr
		}
		request.Publication, err = io.ReadAll(io.LimitReader(file, 4<<10+1))
		closeErr := file.Close()
		if err != nil || closeErr != nil || len(request.Publication) > 4<<10 {
			return serviceconn.Result{}, errors.Join(err, closeErr, errors.New("publication file is invalid or oversized"))
		}
	}
	setup.Resources("timer", 1)
	defer setup.Resources("timer", -1)
	operation, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	result, err := endpoint.Do(operation, request)
	result.ApplicationIPCAccepts = setup.Resources("application-accept", 0)
	result.RouteAttachmentsAccepted = setup.Resources("route-attachment-accept", 0)
	result.IntroductionReceipt = published.IntroductionReceipt
	if plan.Role == "publisher" {
		result.IntroductionAcknowledgement = published.IntroductionAcknowledgement
		err = errors.Join(err, os.Remove(plan.PublicationFile))
	}
	err = errors.Join(err, deliverResult(application, result))
	return result, err
}
func (plan endpointPlan) roleAction() string {
	if plan.Role == "client" {
		return "connect"
	}
	return "accept"
}
