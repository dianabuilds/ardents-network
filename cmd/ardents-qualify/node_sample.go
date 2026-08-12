package main

import (
	"errors"
	"time"

	nodequalification "github.com/dianabuilds/ardents-network/internal/qualification/node"
)

func evaluateNodeSample(arguments []string) ([]byte, error) {
	if len(arguments) != 0 {
		return nil, errors.New("sample-node has unexpected arguments")
	}
	return nodequalification.SampleContainerResources(time.Now())
}
