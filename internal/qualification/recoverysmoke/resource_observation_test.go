package recoverysmoke

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func TestFinalResourceSampleAllowsOneIncompleteDockerStatsRound(t *testing.T) {
	terminal := int64(5 * time.Second)
	samples := []recovery.ResourceSample{{AtNanos: int64(3100 * time.Millisecond),
		ClientSent: 1, PublisherReceived: 1}}
	if _, err := finalResourceSample(samples, terminal); err != nil {
		t.Fatalf("complete sample from the prior Docker stats round rejected: %v", err)
	}
	samples[0].AtNanos = int64(2999 * time.Millisecond)
	if _, err := finalResourceSample(samples, terminal); err == nil {
		t.Fatal("sample older than two seconds accepted")
	}
}
