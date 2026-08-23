package main

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

type applicationResult struct {
	result endpoint.Result
	err    error
}

func waitForResult(stream interface {
	Result() (endpoint.Result, error)
	SetReadDeadline(time.Time) error
}) <-chan applicationResult {
	completed := make(chan applicationResult, 1)
	go func() {
		result, err := stream.Result()
		completed <- applicationResult{result: result, err: err}
		if err == nil && result.Class != "clean service connection close" {
			_ = stream.SetReadDeadline(time.Now())
		}
	}()
	return completed
}
