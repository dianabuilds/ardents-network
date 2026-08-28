package node

import (
	"errors"
	"net"
	"strconv"
)

func rendezvousListenAddress(advertised, override string) (string, error) {
	if override == "" {
		return advertised, nil
	}
	advertisedHost, advertisedPort, advertisedErr := net.SplitHostPort(advertised)
	overrideHost, overridePort, overrideErr := net.SplitHostPort(override)
	advertisedNumber, advertisedPortErr := strconv.Atoi(advertisedPort)
	overrideNumber, overridePortErr := strconv.Atoi(overridePort)
	advertisedIP, overrideIP := net.ParseIP(advertisedHost), net.ParseIP(overrideHost)
	if advertisedErr != nil || advertisedIP == nil || advertisedIP.IsUnspecified() || advertisedPortErr != nil ||
		overrideErr != nil || overrideIP == nil || overridePortErr != nil || !overrideIP.IsLoopback() ||
		overrideNumber < 1 || overrideNumber > 65535 || advertisedNumber != overrideNumber {
		return "", errors.New("Rendezvous loopback listen override is invalid or differs from the State port")
	}
	return override, nil
}
