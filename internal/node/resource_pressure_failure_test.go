package node

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestResourceSampleFailureStageRetainsFixedOwnerCategory(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "deadline", err: fmt.Errorf("sample: %w", context.DeadlineExceeded), want: "deadline"},
		{name: "canceled", err: fmt.Errorf("sample: %w", context.Canceled), want: "canceled"},
		{name: "lock", err: errors.New("hosting lock is unavailable"), want: "hosting-lock"},
		{name: "state", err: errors.New("hosting state checksum differs"), want: "hosting-state"},
		{name: "observation", err: errors.New("hosting observation continuity is unavailable"), want: "hosting-observation"},
		{name: "interface", err: errors.New("hosting interface counter is unavailable"), want: "hosting-interface"},
		{name: "cgroup", err: errors.New("owner cgroup inventory exceeds bound"), want: "owner-cgroup"},
		{name: "unknown", err: errors.New("private implementation error"), want: "other"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := resourceSampleFailureStage(test.err); got != test.want {
				t.Fatalf("resource sample failure stage = %q, want %q", got, test.want)
			}
		})
	}
}
