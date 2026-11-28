package state

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// Detached policy fixtures grant no manager/cgroup proof. The public adapter
// cannot receive these dictionaries and never accepts a zero native lifetime.
// These are the cross-role equivalence oracles for the shared node-duty native
// skeleton (W4, #711): every role binds exactly its own fixed unit, declaration
// and root count, refuses every sibling role's facts, and preserves the causal
// stopped-policy refusal matrix.

func TestNodeDutyRepairCannotManufactureNativeQuiescence(t *testing.T) {
	if RepairRetainedForwardingProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized Forwarding repair")
	}
	if RepairRetainedIntroductionProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized Introduction repair")
	}
	if RepairRetainedJoinProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized JOIN repair")
	}
	if RepairRetainedDescriptorProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized Descriptor repair")
	}
	for _, original := range []*systemd.ForwardingRuntime{nil, {}} {
		if RepairRetainedForwardingProducer(context.Background(), Config{}, original, "", nil) == nil {
			t.Fatal("absent original supplied native proof")
		}
	}
}

func TestNodeDutyRepairDescriptorRootCountRefusal(t *testing.T) {
	// The fixed five-root count is enforced before any filesystem observation,
	// so the refusal is provable without the root-native parent chain.
	for _, count := range []int{0, 4, 6} {
		roots := make([]string, count)
		for i := range roots {
			roots[i] = "/var/lib/ardents/roles/descriptor/root-" + string(rune('a'+i))
		}
		if err := checkNodeDutyRepairRoots(descriptorRepairNative, roots); !errors.Is(err, ErrDescriptorRepairUnavailable) {
			t.Fatal("wrong Descriptor writable-root count accepted:", count, err)
		}
	}
}
