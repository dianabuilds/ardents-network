package replication

import (
	"context"
	"testing"
)

func TestReplicaControlRetriesLostResponse(t *testing.T) {
	attempts := 0
	err := retryReplicaControl(context.Background(), func(context.Context) error {
		attempts++
		if attempts == 1 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry replica control: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}
