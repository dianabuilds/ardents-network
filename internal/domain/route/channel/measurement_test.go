package channel

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func TestMeasuredBudgetRefusalAndStaleReleasePreserveOriginalClaims(t *testing.T) {
	capture, err := measurement.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Stop()
	budget := NewBudget(16 << 10)
	release, err := budget.HoldChild(7)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if other, err := budget.HoldControl(); err == nil || other != nil {
		t.Fatal("control exceeded real bound")
	}
	sample := func(id runtimeevent.Instrument) runtimeevent.Measurement {
		t.Helper()
		capture.Sample()
		spec, _ := id.Spec()
		for _, v := range capture.Latest().Values {
			if v.Name == spec.Name {
				return v
			}
		}
		t.Fatal("instrument absent")
		return runtimeevent.Measurement{}
	}
	if got := sample(runtimeevent.RouteReservedBytes); got.Current != 7 || got.Peak != 7 || !got.Available {
		t.Fatal("refusal fabricated reservation", got)
	}
	release()
	replacement, err := budget.HoldChild(9)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement()
	release()
	if got := sample(runtimeevent.RouteReservedBytes); got.Current != 9 || got.Peak != 9 {
		t.Fatal("stale return changed replacement", got)
	}
	replacement()
	if got := sample(runtimeevent.RouteChildren); got.Current != 0 || got.Peak != 1 {
		t.Fatal(got)
	}
}
