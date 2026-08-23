package endpoint

import (
	"context"
	"errors"
	"os"
)

// Run loads one role-local plan and owns its Endpoint process composition.
func Run(ctx context.Context, planPath string, ready func(string)) (RuntimeResult, error) {
	plan, err := readPlan(planPath)
	if err != nil {
		return RuntimeResult{}, err
	}
	return runEndpoint(ctx, plan, func() {
		if ready != nil {
			ready(plan.Role)
		}
	})
}

func runEndpoint(ctx context.Context, plan endpointPlan, ready func()) (RuntimeResult, error) {
	setup, at, deadline, err := endpointSetup(plan)
	if err != nil {
		return RuntimeResult{}, err
	}
	lifetime, err := plan.connectionLifetime(deadline)
	if err != nil {
		return RuntimeResult{}, err
	}
	endpoint, err := New(setup)
	if err != nil {
		return RuntimeResult{}, err
	}
	applicationListener, err := listenLocal(plan.ApplicationSocket, deadline)
	if err != nil {
		return RuntimeResult{}, err
	}
	setup.Resources("timer", 1)
	defer setup.Resources("timer", -1)
	setup.Resources("control-file", 1)
	defer setup.Resources("control-file", -1)
	defer func() { _ = applicationListener.Close(); _ = os.Remove(plan.ApplicationSocket) }()
	resultPath, resultListener, err := listenResult(plan.ApplicationSocket, deadline)
	if err != nil {
		return RuntimeResult{}, err
	}
	setup.Resources("control-file", 1)
	defer setup.Resources("control-file", -1)
	defer func() { _ = resultListener.Close(); _ = os.Remove(resultPath) }()
	routeListener, err := listenLocal(plan.RouteSocket, deadline)
	if err != nil {
		return RuntimeResult{}, err
	}
	setup.Resources("timer", 1)
	defer setup.Resources("timer", -1)
	setup.Resources("control-file", 1)
	defer setup.Resources("control-file", -1)
	defer func() { _ = routeListener.Close(); _ = os.Remove(plan.RouteSocket) }()
	stopListeners := context.AfterFunc(ctx, func() {
		_ = applicationListener.Close()
		_ = resultListener.Close()
		_ = routeListener.Close()
	})
	defer stopListeners()
	var published PublicationResult
	if plan.Role == "publisher" {
		var publishErr error
		published, publishErr = publishCurrent(endpoint, setup.Resources, plan, setup.AdministrationPrincipal, at, deadline, ready)
		if publishErr != nil {
			return RuntimeResult{}, publishErr
		}
		defer setup.Resources("control-file", -2)
	} else if ready != nil {
		ready()
	}
	openAttachment := routeAttachmentOpener(routeListener, setup.Resources)
	result, runErr := serveEndpointConnections(ctx, plan, endpoint, setup, applicationListener, resultListener,
		openAttachment, published, at, deadline, lifetime)
	if plan.Role == "publisher" {
		runErr = errors.Join(runErr, os.Remove(plan.PublicationFile))
	}
	return result, runErr
}
