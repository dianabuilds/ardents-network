//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
)

func TestServiceBindingRetainsContextCancellationCause(t *testing.T) {
	for _, side := range []string{"reader", "publisher"} {
		t.Run(side, func(t *testing.T) {
			client, publisher, _ := serviceFixture(t)
			t.Cleanup(func() {
				for _, binding := range []*serviceBinding{client, publisher} {
					binding.owner.retireJob(binding.job)
					if err := binding.owner.finishJobCleanup(binding.job, nil); err != nil {
						t.Error(err)
					}
				}
			})
			binding := client
			if side == "publisher" {
				binding = publisher
			}
			binding.owner.lease.Release()
			if err := binding.Current(); err != context.Canceled {
				t.Fatalf("context loss must retain its cause before child cleanup: %v", err)
			}
			binding.owner.retireJob(binding.job)
			if err := binding.owner.finishJobCleanup(binding.job, nil); err != nil {
				t.Fatal(err)
			}
			if err := binding.Current(); err != context.Canceled {
				t.Fatalf("joined job cleanup lost its parent's cancellation: %v", err)
			}
		})
	}
}

func TestServiceBindingIndependentRetirementRemainsDistinct(t *testing.T) {
	client, publisher, _ := serviceFixture(t)
	t.Cleanup(func() {
		for _, binding := range []*serviceBinding{client, publisher} {
			binding.owner.retireJob(binding.job)
			if err := binding.owner.finishJobCleanup(binding.job, nil); err != nil {
				t.Error(err)
			}
		}
	})
	client.owner.retireJob(client.job)
	if err := client.Current(); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("independent job retirement became parent cancellation: %v", err)
	}
	foreign := *publisher
	foreign.job = client.job
	publisher.owner.lease.Release()
	if err := foreign.Current(); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("foreign job failure became parent cancellation: %v", err)
	}
}
