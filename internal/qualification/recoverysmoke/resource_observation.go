package recoverysmoke

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func finalResourceSample(samples []recovery.ResourceSample, terminalNanos int64) (recovery.ResourceSample, error) {
	if len(samples) == 0 || terminalNanos <= 0 {
		return recovery.ResourceSample{}, errors.New("terminal host resource sample is missing")
	}
	result := samples[len(samples)-1]
	if result.AtNanos <= 0 || result.AtNanos > terminalNanos+int64(1500*time.Millisecond) ||
		terminalNanos-result.AtNanos > int64(1500*time.Millisecond) ||
		result.ClientReceived+result.ClientSent == 0 || result.PublisherReceived+result.PublisherSent == 0 {
		return recovery.ResourceSample{}, errors.New("terminal host resource sample does not cover the active interval")
	}
	return result, nil
}

func sameResourceObservation(left, right recovery.ResourceSample) bool {
	interval := right.AtNanos - left.AtNanos
	if interval < 0 || interval > int64(600*time.Millisecond) {
		return false
	}
	left.AtNanos, right.AtNanos = 0, 0
	return left == right
}
