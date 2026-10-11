//go:build !linux

package main

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"io"
)

// The unsupported composition has no original native registration owner.
type routeRegistration struct {
	close    func() error
	withdraw func(context.Context) error
	done     <-chan struct{}
	slot     [32]byte
	facts    introduction.RegistrationFacts
}

func startRoutePrefix(context.Context, routePrefixPlan, admissionAuthority, *stock.Owner) (routeHandle, error) {
	return routeHandle{}, errors.New("route requires Linux")
}
func startRouteBootstrap(context.Context, routePrefixPlan, admissionAuthority, *stock.Owner) (routeHandle, error) {
	return routeHandle{}, errors.New("route selection and durable roots require Linux")
}
func newRouteJoinContext(context.Context, routePrefixPlan, admissionAuthority, *stock.Owner) (routeJoinContext, error) {
	return routeJoinContext{}, errors.New("route requires Linux")
}
func runRoute(context.Context, []string, io.Writer, io.Writer) int { return 1 }
