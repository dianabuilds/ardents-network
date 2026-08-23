package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// Admit issues one local capability for the exact Broker grant. Endpoint's
// role-specific operations consume it before touching publication or a Route.
func (endpoint *endpoint) Admit(principal [32]byte, surface broker.Surface) ([32]byte, error) {
	if endpoint == nil || (surface != broker.Connection && surface != broker.Administration) {
		return [32]byte{}, errors.New("local interface surface is not granted")
	}
	return endpoint.admission.Admit(principal, surface)
}

func (endpoint *endpoint) consume(capability, principal [32]byte, surface string) (broker.Receipt, error) {
	return endpoint.admission.Consume(capability, principal, broker.Surface(surface))
}

func projectReceipt(result *RuntimeResult, receipt broker.Receipt) {
	result.Admission = receipt
}
