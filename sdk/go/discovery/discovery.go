// Package discovery defines the public Go SDK service-locator contract. It
// does not own Application transport, admission, service connections, or
// service authentication.
package discovery

import (
	"context"

	identitycontract "ardents/api/ardents/identity/v1"
)

type Scheme string

const (
	SchemeHTTPS Scheme = identitycontract.ApplicationDiscoverySchemeHTTPS
	SchemeHTTP  Scheme = identitycontract.ApplicationDiscoverySchemeHTTP
	SchemeTCP   Scheme = identitycontract.ApplicationDiscoverySchemeTCP
)

type Query struct {
	ServiceType     string
	AcceptedSchemes []Scheme
}

type Target struct {
	ServiceID string
	Endpoint  string
	Scheme    Scheme
}

type Service interface {
	Resolve(context.Context, Query) ([]Target, error)
}
