package updatetransaction

import (
	"errors"
	"fmt"
	"testing"
)

type unavailableNonComparable struct{ values []byte }

func (unavailableNonComparable) Error() string { return "non-comparable wrapper" }
func (unavailableNonComparable) Unwrap() error { return ErrSelfTestUnavailable }

type unavailableCycle struct{}

func (*unavailableCycle) Error() string       { return "cycle" }
func (cycle *unavailableCycle) Unwrap() error { return cycle }

type unavailablePanic struct{}

func (unavailablePanic) Error() string { return "panic" }
func (unavailablePanic) Unwrap() error { panic("caller error unwrap panic") }

func TestSelfTestUnavailableOnly(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "leaf", err: ErrSelfTestUnavailable, want: true},
		{name: "wrapped", err: fmt.Errorf("wrapped: %w", ErrSelfTestUnavailable), want: true},
		{name: "join", err: errors.Join(ErrSelfTestUnavailable, ErrSelfTestUnavailable), want: true},
		{name: "mixed", err: errors.Join(ErrSelfTestUnavailable, errors.New("local failure")), want: false},
		{name: "local", err: errors.New("local failure"), want: false},
		{name: "non-comparable", err: unavailableNonComparable{values: []byte{1}}, want: true},
		{name: "cycle", err: &unavailableCycle{}, want: false},
		{name: "panic", err: unavailablePanic{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selfTestUnavailableOnly(test.err); got != test.want {
				t.Fatalf("selfTestUnavailableOnly(%T) = %t, want %t", test.err, got, test.want)
			}
		})
	}
}
