package main

import "errors"

// unavailableOutput models a writer that rejects a command receipt.
type unavailableOutput struct{}

func (unavailableOutput) Write([]byte) (int, error) { return 0, errors.New("private-output-error") }
