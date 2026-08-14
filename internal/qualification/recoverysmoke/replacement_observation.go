package recoverysmoke

import (
	"errors"
	"fmt"
)

func orderedReplacementObservation(startObservers, inspectProcesses func() error) error {
	if err := startObservers(); err != nil {
		return err
	}
	return inspectProcesses()
}

func replacementObservationError(primary, trafficErr, sampleErr error) error {
	if trafficErr != nil {
		trafficErr = fmt.Errorf("cleanup replacement traffic observers: %w", trafficErr)
	}
	if sampleErr != nil {
		sampleErr = fmt.Errorf("stop replacement resource sampler: %w", sampleErr)
	}
	return errors.Join(primary, trafficErr, sampleErr)
}

func finalizeReplacementObservation(traffic *trafficObservers, sampler **statsSampler,
	finish func(*trafficObservers, *statsSampler) (replacementCell, error)) (replacementCell, error) {
	result, err := finish(traffic, *sampler)
	*sampler = nil
	return result, err
}
