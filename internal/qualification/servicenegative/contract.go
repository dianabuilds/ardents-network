package servicenegative

import (
	"context"
	"errors"
	"fmt"
)

// Receipt is the complete mandatory negative-case observation.
type Receipt struct {
	Schema     string            `json:"schema"`
	Negatives  map[string]bool   `json:"negatives"`
	Mechanisms map[string]string `json:"mechanisms"`
	Operations map[string]bool   `json:"operations"`
	Classes    map[string]string `json:"classes"`
	Counts     map[string]uint32 `json:"counts"`
}

// Run exercises every distinct negative case and fails if any forbidden action is accepted.
func Run(ctx context.Context) (Receipt, error) {
	fixture, err := newFixture()
	if err != nil {
		return Receipt{}, err
	}
	negatives, mechanisms := fixture.run(ctx)
	operations, classes, counts := fixture.streamObservations(ctx)
	result := Receipt{Schema: "ardents-h3-service-negative-v1", Negatives: negatives, Mechanisms: mechanisms,
		Operations: operations, Classes: classes, Counts: counts}
	for _, passed := range result.Negatives {
		if !passed {
			return result, errors.New("one or more Stage 3 negative cases failed")
		}
	}
	for name, passed := range result.Operations {
		if !passed {
			return result, fmt.Errorf("stage 3 stream observation failed: %s", name)
		}
	}
	return result, nil
}
