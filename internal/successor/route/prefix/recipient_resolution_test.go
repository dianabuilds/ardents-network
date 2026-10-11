package prefix

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

func TestResolveRendezvousRequiresOriginalSourceBeforeObservation(t *testing.T) {
	observations := 0
	detached := &Prefix{config: Config{Current: func() (network.RuntimeView, error) {
		observations++
		return network.RuntimeView{}, nil
	}}}
	for _, source := range []*Prefix{nil, detached} {
		duty, end, err := source.ResolveRendezvous([32]byte{1}, 1, nil)
		if err == nil || duty != (network.RetainedDuty{}) || end != (time.Time{}) {
			t.Fatal("detached Source supplied independently checked recipient")
		}
	}
	if observations != 0 {
		t.Fatal("detached role reached Network effects")
	}
}
