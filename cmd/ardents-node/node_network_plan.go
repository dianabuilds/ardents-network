package main

import (
	"errors"
	"net"
	"strconv"
)

func validatePlanCarrierRelayEndpoint(plan nodePlan) error {
	if plan.ClosedForwarding == nil || plan.ClosedForwarding.CarrierRelayEndpoint == "" {
		return nil
	}
	host, portText, err := net.SplitHostPort(plan.ClosedForwarding.CarrierRelayEndpoint)
	if err != nil {
		return errors.New("closed forwarding Carrier relay endpoint is invalid")
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 || ip == nil || ip.IsUnspecified() {
		return errors.New("closed forwarding Carrier relay endpoint is invalid")
	}
	return nil
}
