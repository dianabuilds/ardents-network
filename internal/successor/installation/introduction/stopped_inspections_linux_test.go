package introduction

import (
	"context"
	"errors"
	"testing"
)

func TestStoppedInspectionsRefuseMissingOriginalInputs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		completed *completedInitial
		want      error
	}{
		{"missing owner", nil, ErrInput},
		{"missing plan", &completedInitial{}, ErrBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := inspectStoppedHistories(context.Background(), tc.completed)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("inspection = %v, error = %v; want %v", got, err, tc.want)
			}
		})
	}
}
