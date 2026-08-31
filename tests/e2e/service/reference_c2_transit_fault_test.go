//go:build referencec2

package service_test

type referenceC2TransitFault string

const (
	referenceC2TransitFaultCarrierLoss     referenceC2TransitFault = "carrier-loss"
	referenceC2TransitFaultProductNodeLoss referenceC2TransitFault = "product-node-loss"
)
