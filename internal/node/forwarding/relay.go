package forwarding

import (
	"errors"
	"net"
	"strconv"
)

// DialAddress retains the State-selected recipient while routing
// this Node's physical Carrier through one operator-owned transparent relay.
// Authentication still uses the selected recipient key after the relay dial.
func DialAddress(advertised, relay string) (string, error) {
	if relay == "" {
		return advertised, nil
	}
	if !ValidCarrierEndpoint(advertised) || !ValidCarrierEndpoint(relay) {
		return "", errors.New("closed forwarding Carrier relay endpoint is invalid")
	}
	return relay, nil
}

// ValidCarrierEndpoint requires a literal, specified IP and nonzero port.
func ValidCarrierEndpoint(endpoint string) bool {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portText, 10, 16)
	return err == nil && port != 0 && ip != nil && !ip.IsUnspecified()
}
