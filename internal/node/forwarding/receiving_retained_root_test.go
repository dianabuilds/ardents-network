package forwarding

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
)

func TestReceivingStartupRefusesLostSpendJournal(t *testing.T) {
	root := t.TempDir()
	binding := spending.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	clock := func() time.Time { return window.Add(time.Minute) }
	resources, err := openReceivingResources(root, binding, clock)
	if err != nil {
		t.Fatal(err)
	}
	token := make([]byte, 354)
	token[0] = 42
	if err := resources.spends.Spend(token, window, clock()); err != nil {
		t.Fatal(err)
	}
	if err := resources.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "closed-token-spends"), filepath.Join(t.TempDir(), "retained")); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		reopened, err := openReceivingResources(root, binding, clock)
		if reopened != nil {
			_ = reopened.Close()
		}
		if err == nil || reopened != nil {
			t.Fatal("receiving resources accepted an incomplete spend root")
		}
	}
}
