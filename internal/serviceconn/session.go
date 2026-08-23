package serviceconn

import "github.com/dianabuilds/ardents-network/internal/application/broker"

func (endpoint *endpoint) admit(input Request) (Result, error) {
	switch input.Surface {
	case "connection":
	case "administration":
	default:
		return denied("local interface surface is not granted")
	}
	capability, err := endpoint.admission.Admit(input.Principal, broker.Surface(input.Surface))
	if err != nil {
		return failed("local authorization or policy denial", err.Error(), err)
	}
	return Result{Class: "authorized", Session: capability}, nil
}

func (endpoint *endpoint) consume(capability, principal [32]byte, surface string) (broker.Receipt, error) {
	return endpoint.admission.Consume(capability, principal, broker.Surface(surface))
}

func projectReceipt(result *Result, receipt broker.Receipt) {
	result.PrincipalCommitment = receipt.Principal
	result.SessionCommitment = receipt.Session
	result.GrantSurface = string(receipt.Surface)
	result.SessionConsumed = true
	result.BrokerCommitment = receipt.Broker
	result.GrantCommitment = receipt.Grant
	result.SessionIssuedAt = receipt.IssuedAt
	result.SessionExpiresAt = receipt.ExpiresAt
}
