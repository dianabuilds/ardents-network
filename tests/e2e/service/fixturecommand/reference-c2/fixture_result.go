package main

import endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"

type result struct {
	Schema, Role, Class string
	Passed              bool
	Workload            *dynamicWorkloadResult     `json:"Workload,omitempty"`
	Runtime             *endpointapi.RuntimeResult `json:"Runtime,omitempty"`
	CarrierRelay        *carrierRelaySnapshot      `json:"CarrierRelay,omitempty"`
}

type publisherTerminal string

type transitFault string

const (
	publisherTerminalWithdrawal       publisherTerminal = "withdrawal"
	publisherTerminalApplicationReset publisherTerminal = "application-reset"
	publisherTerminalEndpointStop     publisherTerminal = "endpoint-stop"
	transitFaultCarrierLoss           transitFault      = "carrier-loss"
	transitFaultProductNodeLoss       transitFault      = "product-node-loss"
)
