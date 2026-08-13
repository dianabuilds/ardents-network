package main

import (
	"context"
	"crypto/ed25519"
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
	if plan.Role == "publisher" {
		if err := publishCurrent(endpoint, plan, at, deadline, ready); err != nil {
			return serviceconn.Result{}, err
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
		request.Publication, err = os.ReadFile(plan.PublicationFile)
		if err != nil {
			return serviceconn.Result{}, err
		}
	}
	return endpoint.Do(ctx, request)
}

func (plan endpointPlan) roleAction() string {
	if plan.Role == "client" {
		return "connect"
	}
	return "accept"
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
