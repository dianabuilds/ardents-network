// Package diagnostics owns diagnostic protocol handlers and mappings.
// It does not own diagnostic truth creation.
package diagnostics

import (
	domain "ardents/internal/diagnostics"
	localauth "ardents/internal/localapi/auth"
	protocol "ardents/internal/localapi/protocol"
)

type Endpoint struct {
	service domain.Service
	auth    localauth.Config
}

func NewHandler(service domain.Service, auth localauth.Config) *Endpoint {
	return &Endpoint{service: service, auth: auth}
}

func operationStatus(state, reason string, accepted bool) *protocol.OperationStatus {
	return &protocol.OperationStatus{State: state, Reason: reason, Accepted: accepted}
}
