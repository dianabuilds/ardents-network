package connection

import (
	"context"
	"testing"
	"time"
)

func TestDetachedInitiatorSourceRefusesWithoutOwnerEffects(t *testing.T) {
	for _, source := range []*InitiatorSource{nil, {}} {
		if owner, err := source.AuthenticateInitial(); err == nil || owner != nil {
			t.Fatal("detached Source supplied authenticated state")
		}
	}
	if source, err := NewInitiatorSource(InitiatorSourceConfig{}); err == nil || source != nil {
		t.Fatal("missing original owners supplied a Source")
	}
	original := &InitiatorSource{}
	original.self = original
	copied := InitiatorSource{self: original}
	if err := copied.Close(); err == nil {
		t.Fatal("copied Source could retire the original")
	}
	if owner, err := copied.AuthenticateInitial(); err == nil || owner != nil {
		t.Fatal("copied Source could authenticate an Attachment")
	}
}

func TestAttachmentSetupKeepsEarlierOriginalBounds(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 500000000, time.UTC)
	for _, test := range []struct {
		name   string
		caller time.Duration
		bounds []time.Duration
		want   time.Duration
	}{
		{name: "ceiling", caller: time.Minute, want: 30 * time.Second},
		{name: "caller", caller: 7 * time.Second, want: 7 * time.Second},
		{name: "work-safety", caller: time.Minute, bounds: []time.Duration{12 * time.Second, 50 * time.Second}, want: 12 * time.Second},
		{name: "registration", caller: time.Minute, bounds: []time.Duration{50 * time.Second, 9 * time.Second}, want: 9 * time.Second},
		{name: "recovery", caller: time.Minute, bounds: []time.Duration{4 * time.Second, 50 * time.Second, 40 * time.Second}, want: 4 * time.Second},
		{name: "expired", caller: time.Minute, bounds: []time.Duration{-time.Second}, want: -time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), now.Add(test.caller))
			defer cancel()
			bounds := make([]time.Time, len(test.bounds))
			for i, bound := range test.bounds {
				bounds[i] = now.Add(bound)
			}
			got := attachmentSetupDeadline(ctx, now, bounds...)
			want := now.Add(test.want).Truncate(time.Second)
			if !got.Equal(want) {
				t.Fatalf("setup bound %v, want %v", got, want)
			}
			if got.After(now.Add(test.caller)) {
				t.Fatal("rounding extended original caller")
			}
		})
	}
}
