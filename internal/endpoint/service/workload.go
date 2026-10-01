//go:build linux

package service

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// maximumStreamBytes bounds every directional workload contract.
const maximumStreamBytes = uint32(768 << 20)

// WorkloadBounds is the checked directional byte contract supplied
// by trusted worker composition. It is expressed in the reader direction so
// the Publisher cannot independently reinterpret the workload.
type WorkloadBounds struct {
	readerSend    uint32
	readerReceive uint32
}

func NewWorkloadBounds(readerSend, readerReceive uint32) (WorkloadBounds, error) {
	if readerSend == 0 || readerReceive == 0 || readerSend > maximumStreamBytes || readerReceive > maximumStreamBytes {
		return WorkloadBounds{}, errors.New("text Service workload bounds are unavailable")
	}
	return WorkloadBounds{readerSend: readerSend, readerReceive: readerReceive}, nil
}

func DocumentWorkloadBounds() (WorkloadBounds, error) {
	return NewWorkloadBounds(512, textdocument.MaximumBytes+13)
}

func StreamQualificationWorkloadBounds() (WorkloadBounds, error) {
	return NewWorkloadBounds(64<<20, 64<<20)
}

func (bounds WorkloadBounds) Direction(surface broker.Surface) (uint32, uint32, error) {
	if bounds.readerSend == 0 || bounds.readerReceive == 0 || bounds.readerSend > maximumStreamBytes || bounds.readerReceive > maximumStreamBytes {
		return 0, 0, errors.New("text Service workload bounds are unavailable")
	}
	switch surface {
	case broker.Connection:
		return bounds.readerSend, bounds.readerReceive, nil
	case broker.Administration:
		return bounds.readerReceive, bounds.readerSend, nil
	default:
		return 0, 0, errors.New("text Service workload direction is unavailable")
	}
}
