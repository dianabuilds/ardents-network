package node

import (
	"errors"
	"net"
	"strconv"
)

// closedListenAddress keeps the State endpoint as peer-visible authority while
// allowing an installed Node to bind one literal private target behind an
// operator-owned relay. The local port may differ because the relay owns the
// advertised port; neither address changes authenticated peer selection.
func closedListenAddress(advertised, override string) (string, error) {
	if override == "" {
		return advertised, nil
	}
	advertisedHost, advertisedPort, advertisedErr := net.SplitHostPort(advertised)
	overrideHost, overridePort, overrideErr := net.SplitHostPort(override)
	advertisedNumber, advertisedPortErr := strconv.Atoi(advertisedPort)
	overrideNumber, overridePortErr := strconv.Atoi(overridePort)
	advertisedIP, overrideIP := net.ParseIP(advertisedHost), net.ParseIP(overrideHost)
	if advertisedErr != nil || advertisedIP == nil || advertisedIP.IsUnspecified() || advertisedPortErr != nil || advertisedNumber < 1 || advertisedNumber > 65535 ||
		overrideErr != nil || overrideIP == nil || overrideIP.To4() == nil || overridePortErr != nil || !(overrideIP.IsLoopback() || overrideIP.IsPrivate()) || overrideNumber < 1 || overrideNumber > 65535 {
		return "", errors.New("closed Node private listen override is invalid")
	}
	return override, nil
}
