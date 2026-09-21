//go:build linux

package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// textServiceWorkloadBounds is the checked directional byte contract supplied
// by trusted worker composition. It is expressed in the reader direction so
// the Publisher cannot independently reinterpret the workload.
type textServiceWorkloadBounds struct {
	readerSend    uint32
	readerReceive uint32
}

func newTextServiceWorkloadBounds(readerSend, readerReceive uint32) (textServiceWorkloadBounds, error) {
	if readerSend == 0 || readerReceive == 0 || readerSend > maximumStreamBytes || readerReceive > maximumStreamBytes {
		return textServiceWorkloadBounds{}, errors.New("text Service workload bounds are unavailable")
	}
	return textServiceWorkloadBounds{readerSend: readerSend, readerReceive: readerReceive}, nil
}

func textDocumentServiceWorkloadBounds() (textServiceWorkloadBounds, error) {
	return newTextServiceWorkloadBounds(512, textdocument.MaximumBytes+13)
}

func streamQualificationServiceWorkloadBounds() (textServiceWorkloadBounds, error) {
	return newTextServiceWorkloadBounds(64<<20, 64<<20)
}

func (bounds textServiceWorkloadBounds) direction(surface broker.Surface) (uint32, uint32, error) {
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
