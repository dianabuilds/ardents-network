package main

import (
	"encoding/json"
	"io"
	"sync"
)

type eventOutput struct {
	mu     sync.Mutex
	target io.Writer
}

func newEventOutput(target io.Writer) *eventOutput { return &eventOutput{target: target} }
func (output *eventOutput) encode(value any) error {
	output.mu.Lock()
	defer output.mu.Unlock()
	return json.NewEncoder(output.target).Encode(value)
}

func (output *eventOutput) append(raw []byte) error {
	output.mu.Lock()
	defer output.mu.Unlock()
	written, err := output.target.Write(raw)
	if err == nil && written != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}
