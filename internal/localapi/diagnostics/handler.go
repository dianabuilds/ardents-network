// Package diagnostics owns diagnostic protocol handlers and mappings.
// It does not own diagnostic truth creation.
package diagnostics

import (
	domain "ardents/internal/diagnostics"
	protocol "ardents/internal/localapi/protocol"
)

type Endpoint struct {
	service domain.Service
}

func NewHandler(service domain.Service) *Endpoint {
	return &Endpoint{service: service}
}

func operationStatus(state, reason string, accepted bool) *protocol.OperationStatus {
	return &protocol.OperationStatus{State: state, Reason: reason, Accepted: accepted}
}
