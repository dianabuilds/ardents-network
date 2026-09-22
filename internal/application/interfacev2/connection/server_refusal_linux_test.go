//go:build linux

package connection

import (
	"context"
	"errors"
	"testing"
)

func TestRefusalPrioritizesSetupCancellation(t *testing.T) {
	classified := Refuse(Outcome{Class: ServiceUnavailable, Reason: "selected service is unavailable"})
	for _, test := range []struct {
		name string
		err  error
		want OutcomeClass
	}{
		{name: "cancellation", err: errors.Join(classified, context.Canceled), want: LocalCancellation},
		{name: "deadline", err: errors.Join(classified, context.DeadlineExceeded), want: LocalTimeout},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := refusal(test.err); got.Class != test.want {
				t.Fatalf("setup refusal = %+v, want class %q", got, test.want)
			}
		})
	}
}
