//go:build linux

package main

import "time"

type ownerNetworkVerdict struct {
	Started, Stopped                                         time.Time
	InterfaceTx, InterfaceRx                                 uint64
	CountedBytes, HostingLedgerDelta                         uint64
	UsefulTx, UsefulRx                                       uint64
	DirectionalWire, DirectionalUseful                       uint64
	DirectionalOverhead                                      uint64
	DirectionalCarrierRatio                                  float64
	TxP95BitsPerSecond                                       float64
	RxP95BitsPerSecond                                       float64
	OneSecondSampleCount                                     int
	P95RSSBytes                                              uint64
	MeanCPUPercent                                           float64
	HostingProvider                                          string
	HostingPeriodStart, HostingPeriodEnd                     time.Time
	HostingUnit, HostingDirections                           string
	HostingQuantity, HostingInitialUsed, HostingLowWatermark uint64
}
