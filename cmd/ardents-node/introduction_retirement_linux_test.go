package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIntroductionExplicitStopMarkerCannotHideLatePhysicalFailure(t *testing.T) {
	marker := &introductionExplicitStop{context.Canceled}
	if !completedIntroductionExplicitStop(marker) {
		t.Fatal("exact completed marker grammar refused")
	}
	for _, err := range []error{
		context.Canceled, errors.New("introduction explicitly stopped"),
		errors.Join(marker, errors.New("late physical Close failed")),
		fmt.Errorf("foreign wrapper: %w", marker),
		&introductionExplicitStop{fmt.Errorf("physical Close: %w", context.Canceled)},
		&introductionExplicitStop{errors.Join(context.Canceled, errors.New("State Close failed"))},
		&introductionExplicitStop{context.DeadlineExceeded},
	} {
		if completedIntroductionExplicitStop(err) {
			t.Fatal("foreign/late result became clean process exit", err)
		}
	}
}
