package route

import (
	"net"
	"strconv"
)

// literalEndpoint admits only a literal IP and one valid TCP/QUIC port. Route
// never sends a State-selected hop through ambient DNS resolution.
func literalEndpoint(endpoint string) bool {
	host, port, err := net.SplitHostPort(endpoint)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}
