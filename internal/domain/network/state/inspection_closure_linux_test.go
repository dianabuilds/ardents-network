package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// The retained Node and Source inspections share one local-closure classifier
// but keep distinct closed sentinels. Every leaf of a wrapped/joined tree must
// be a local cause of the owning role; the other role's sentinel or any
// operational failure keeps the refusal terminal.
func TestInspectionLocalClosureClassification(t *testing.T) {
	foreign := errors.New("foreign operational failure")
	for _, role := range []struct {
		name     string
		sentinel error
		other    error
	}{
		{"Node", errNodeInspectionClosed, errSourceInspectionClosed},
		{"Source", errSourceInspectionClosed, errNodeInspectionClosed},
	} {
		t.Run(role.name, func(t *testing.T) {
			cases := []struct {
				name string
				err  error
				want bool
			}{
				{"nil", nil, true},
				{"own-sentinel", role.sentinel, true},
				{"canceled", context.Canceled, true},
				{"foreign", foreign, false},
				{"other-role-sentinel", role.other, false},
				{"wrapped-own-sentinel", fmt.Errorf("outer: %w", role.sentinel), true},
				{"wrapped-canceled", fmt.Errorf("outer: %w", context.Canceled), true},
				{"wrapped-foreign", fmt.Errorf("outer: %w", foreign), false},
				{"wrapped-other-sentinel", fmt.Errorf("outer: %w", role.other), false},
				{"joined-local-causes", errors.Join(role.sentinel, context.Canceled), true},
				{"joined-foreign-last", errors.Join(role.sentinel, foreign), false},
				{"joined-foreign-first", errors.Join(foreign, role.sentinel), false},
				{"joined-other-sentinel", errors.Join(role.sentinel, role.other), false},
				{"nested-join-local", errors.Join(fmt.Errorf("outer: %w", role.sentinel), errors.Join(context.Canceled, nil)), true},
				{"nested-join-foreign-leaf", errors.Join(role.sentinel, errors.Join(context.Canceled, fmt.Errorf("deep: %w", foreign))), false},
				{"wrapped-join-local", fmt.Errorf("outer: %w", errors.Join(role.sentinel, context.Canceled)), true},
				{"wrapped-nil-leaf", fmt.Errorf("outer: %w", error(nil)), true},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					if got := inspectionLocalClosure(tc.err, role.sentinel); got != tc.want {
						t.Fatalf("inspectionLocalClosure(%v) = %v, want %v", tc.err, got, tc.want)
					}
				})
			}
		})
	}
}
