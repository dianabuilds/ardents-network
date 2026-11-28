package state

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// Detached policy fixtures grant no manager/cgroup proof. The public adapter
// cannot receive these dictionaries and never accepts a zero native lifetime.
func TestSourceRepairCannotManufactureNativeQuiescence(t *testing.T) {
	for _, original := range []*systemd.SourceRuntime{nil, {}} {
		if checkSourceRepairQuiescence(context.Background(), original, config{}, "") == nil {
			t.Fatal("absent original supplied native proof")
		}
	}
	if RepairRetainedSourceProducer(context.Background(), Config{}, nil, "") == nil {
		t.Fatal("absent original authorized repair")
	}
}
