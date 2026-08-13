package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

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
	defer closeLocal(applicationListener, plan.ApplicationSocket)
	routeListener, err := listenLocal(plan.RouteSocket, deadline)
	if err != nil {
		return serviceconn.Result{}, err
	}
	defer closeLocal(routeListener, plan.RouteSocket)
	var published serviceconn.Result
	if plan.Role == "publisher" {
		var publishErr error
		published, publishErr = publishCurrent(endpoint, plan, at, deadline, ready)
		if publishErr != nil {
			return serviceconn.Result{}, publishErr
		}
	} else if ready != nil {
		ready()
	}
	application, err := applicationListener.Accept()
	if err != nil {
		return serviceconn.Result{}, err
	}
	route, err := routeListener.Accept()
	if err != nil {
		application.Close()
		return serviceconn.Result{}, err
	}
	session, err := admit(endpoint, setup.ConnectionPrincipal, "connection", at)
	if err != nil {
		application.Close()
		route.Close()
		return serviceconn.Result{}, err
	}
	request := serviceconn.Request{Action: plan.roleAction(), Principal: setup.ConnectionPrincipal,
		Session: session, Route: route, Application: application, BytesEachDirection: plan.BytesEachDirection, At: at}
	if plan.Role == "client" {
		if err := fixedHex(plan.Target, request.Target[:]); err != nil {
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
	operation, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	result, err := endpoint.Do(operation, request)
	result.IntroductionReceipt = published.IntroductionReceipt
	if plan.Role == "publisher" {
		result.IntroductionAcknowledgement = published.IntroductionAcknowledgement
		err = errors.Join(err, os.Remove(plan.PublicationFile))
	}
	return result, err
}
func endpointSetup(plan endpointPlan) (serviceconn.Setup, time.Time, time.Duration, error) {
	var setup serviceconn.Setup
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{
		{plan.NetworkID, setup.NetworkID[:]}, {plan.BrokerID, setup.BrokerID[:]},
		{plan.ConnectionPrincipal, setup.ConnectionPrincipal[:]}, {plan.AdministrationPrincipal, setup.AdministrationPrincipal[:]}} {
		if field.encoded != "" {
			if err := fixedHex(field.encoded, field.destination); err != nil {
				return setup, time.Time{}, 0, err
			}
		}
	}
	setup.AuthorityPublic = make([]byte, ed25519.PublicKeySize)
	if err := fixedHex(plan.AuthorityPublic, setup.AuthorityPublic); err != nil {
		return setup, time.Time{}, 0, err
	}
	setup.IntroductionPublic = make([]byte, ed25519.PublicKeySize)
	if err := fixedHex(plan.IntroductionPublic, setup.IntroductionPublic); err != nil {
		return setup, time.Time{}, 0, err
	}
	setup.GenerationStateFile = plan.GenerationStateFile
	at, err := time.Parse(time.RFC3339, plan.At)
	if err != nil {
		return setup, time.Time{}, 0, err
	}
	deadline, err := time.ParseDuration(plan.Deadline)
	return setup, at, deadline, err
}
func admit(endpoint *serviceconn.Endpoint, principal [32]byte, surface string, at time.Time) ([32]byte, error) {
	result, err := endpoint.Do(context.Background(), serviceconn.Request{Action: "admit", Surface: surface, Principal: principal, At: at})
	return result.Session, err
}
