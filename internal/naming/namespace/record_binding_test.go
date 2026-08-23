package namespace_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

func TestResolvedBindingAcceptsOnlyCurrentSameTargetLineage(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	value := record.Record{
		Name: "alice", Generation: 3, Revision: 7,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: "authority-a", Target: [32]byte{1},
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix(),
	}
	binding, _, err := record.ResolveBindingLegacy(value, now.Unix(), nil)
	if err != nil || binding.Target != value.Target || binding.Commitment == ([32]byte{}) {
		t.Fatalf("binding=%+v err=%v", binding, err)
	}

	renewed := value
	renewed.Revision++
	renewed.LeaseExpiresAt = now.Add(2 * time.Hour).Unix()
	renewed.GraceExpiresAt = now.Add(3 * time.Hour).Unix()
	renewedBinding, _, err := record.ResolveBindingLegacy(renewed, now.Add(time.Minute).Unix(), nil)
	if err != nil || renewedBinding.Target != binding.Target || renewedBinding.Revision <= binding.Revision {
		t.Fatal("same-Target monotonic renewal invalidated the binding")
	}

	cases := map[string]func(*record.Record){
		"recovery pending": func(value *record.Record) { value.Recovery = "recovery-pending" },
		"released": func(value *record.Record) {
			value.Lease = "released"
			value.LeaseExpiresAt, value.GraceExpiresAt = 0, 0
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := renewed
			mutate(&changed)
			if _, _, err := record.ResolveBindingLegacy(changed, now.Add(time.Minute).Unix(), nil); err == nil {
				t.Fatalf("unresolvable Record produced a binding: %+v", changed)
			}
		})
	}
}
