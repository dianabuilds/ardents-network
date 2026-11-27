package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// An operational cause must not disappear inside a wrapped or joined local
// close/cancellation error.
func TestSourceInspectionLocalClosureClassification(t *testing.T) {
	foreign := errors.New("foreign operational failure")
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, true},
		{"own-sentinel", errSourceInspectionClosed, true},
		{"canceled", context.Canceled, true},
		{"foreign", foreign, false},
		{"wrapped-own-sentinel", fmt.Errorf("outer: %w", errSourceInspectionClosed), true},
		{"wrapped-canceled", fmt.Errorf("outer: %w", context.Canceled), true},
		{"wrapped-foreign", fmt.Errorf("outer: %w", foreign), false},
		{"joined-local-causes", errors.Join(errSourceInspectionClosed, context.Canceled), true},
		{"joined-foreign-last", errors.Join(errSourceInspectionClosed, foreign), false},
		{"joined-foreign-first", errors.Join(foreign, errSourceInspectionClosed), false},
		{"nested-join-local", errors.Join(fmt.Errorf("outer: %w", errSourceInspectionClosed), errors.Join(context.Canceled, nil)), true},
		{"nested-join-foreign-leaf", errors.Join(errSourceInspectionClosed, errors.Join(context.Canceled, fmt.Errorf("deep: %w", foreign))), false},
		{"wrapped-join-local", fmt.Errorf("outer: %w", errors.Join(errSourceInspectionClosed, context.Canceled)), true},
		{"wrapped-nil-leaf", fmt.Errorf("outer: %w", error(nil)), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceInspectionLocalClosure(tc.err); got != tc.want {
				t.Fatalf("sourceInspectionLocalClosure(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
