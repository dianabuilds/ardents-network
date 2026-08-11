package networkstate

import (
	"crypto/sha256"
	"testing"
	"time"
)

func TestInterruptedCycleResumesOnlyUnstartedLatestAttempt(t *testing.T) {
	root := t.TempDir()
	if err := inspectRoot(root); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireRootLease(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.release()
	if err := prepareRoot(root); err != nil {
		t.Fatal(err)
	}
	config := config{root: root, orderSeed: sha256.Sum256([]byte("resume-order"))}
	config.sources[0].identity = sha256.Sum256([]byte("resume-source-one"))
	config.sources[1].identity = sha256.Sum256([]byte("resume-source-two"))
	initial := &store{config: config}
	if err := initial.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_100, 0).UTC()
	order, deadline, err := initial.startSourceWave(now)
	if err != nil {
		t.Fatal(err)
	}
	if started, _, err := initial.beginLatestAttempt(order[0]); err != nil || !started {
		t.Fatalf("begin first LATEST: started=%t err=%v", started, err)
	}

	restarted := &store{config: config}
	if err := restarted.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	recoveredOrder, recoveredDeadline, err := restarted.startSourceWave(now.Add(time.Second))
	if err != nil || recoveredOrder != order || !recoveredDeadline.Equal(deadline) {
		t.Fatalf("resume order=%v deadline=%v err=%v", recoveredOrder, recoveredDeadline, err)
	}
	if started, outcome, err := restarted.beginLatestAttempt(order[0]); err != nil || started || outcome != sourceOutcomeInterrupted {
		t.Fatalf("repeated started attempt: started=%t outcome=%d err=%v", started, outcome, err)
	}
	if started, _, err := restarted.beginLatestAttempt(order[1]); err != nil || !started {
		t.Fatalf("unstarted attempt did not resume: started=%t err=%v", started, err)
	}
}
