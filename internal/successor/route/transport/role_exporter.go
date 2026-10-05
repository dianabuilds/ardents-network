package transport

import (
	"crypto/tls"
	"errors"
	"net"
)

// ClosedRoleTLSExporter obtains the exporter of an actual authenticated role
// connection without importing its physical adapter or exposing QUIC state.
// A Node Carrier is a different principal and has no role exporter contract.
func ClosedRoleTLSExporter(connection net.Conn) (ClosedTLSExporter, error) {
	if retained, ok := connection.(*retainedConn); ok {
		return ClosedRoleTLSExporter(retained.Conn)
	}
	if secured, ok := connection.(*tls.Conn); ok {
		if secured != nil {
			return RoleExporter(secured.ConnectionState())
		}
		return nil, errors.New("closed role TLS exporter is unavailable")
	}
	if secured, ok := connection.(interface {
		RoleTLSExporter() (ClosedTLSExporter, error)
	}); ok {
		return secured.RoleTLSExporter()
	}
	return nil, errors.New("closed role TLS exporter is unavailable")
}

// RoleExporter retains only the derived-byte operation from a negotiated role
// state. Profile validation is not Network authenticity or an Admission right;
// the physical adapter still owns peer authentication and principal separation.
func RoleExporter(state tls.ConnectionState) (ClosedTLSExporter, error) {
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile {
		return nil, errors.New("closed role TLS exporter is unavailable")
	}
	return func(label string, context []byte, length int) ([]byte, error) {
		return state.ExportKeyingMaterial(label, context, length)
	}, nil
}
