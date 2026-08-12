package node

import (
	"context"
	"encoding/json"
	"errors"
	"os"
)

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
