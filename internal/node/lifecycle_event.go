package node

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

const eventSchema = "ardents-node-event-v1"

// Event is one bounded external observation of Node lifecycle state.
type Event struct {
	Elapsed          time.Duration           `json:"elapsed,omitempty"`
	Hosting          *resource.HostingSample `json:"hosting,omitempty"`
	Schema           string                  `json:"schema"`
	Kind             string                  `json:"kind"`
	State            string                  `json:"state"`
	At               time.Time               `json:"at"`
	Epoch            uint64                  `json:"epoch,omitempty"`
	Generation       string                  `json:"generation,omitempty"`
	Assignment       string                  `json:"assignment,omitempty"`
	CarrierProfile   string                  `json:"carrier_profile,omitempty"`
	AssignmentDigest [32]byte                `json:"assignment_digest,omitempty"`
	Reason           string                  `json:"reason,omitempty"`
	Resource         *resource.Sample        `json:"resource,omitempty"`
}

// Result describes the observed terminal lifecycle outcome. FAILED may report
// cleanup that did not complete or could not be proven inside its bound.
type Result struct {
	State            string
	Epoch            uint64
	Assignment       string
	CarrierProfile   string
	AssignmentDigest [32]byte
	Reason           string
}

// EventEmitter returns a lifecycle evidence sink whose writes obey the caller's deadline.
func EventEmitter(output *os.File) func(context.Context, Event) error {
	return func(ctx context.Context, event Event) error {
		if len(event.Schema) > 64 || len(event.Kind) > 32 || len(event.State) > 32 ||
			len(event.Generation) > 128 || len(event.Assignment) > 128 || len(event.Reason) > 256 {
			return errors.New("node lifecycle event fields exceed their bounds")
		}
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if len(raw) > 2047 {
			return errors.New("node lifecycle event exceeds its bound")
		}
		raw = append(raw, '\n')
		_, err = writeEvent(ctx, output, raw)
		return err
	}
}
