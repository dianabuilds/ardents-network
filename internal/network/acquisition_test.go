package network

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestAcquisitionRecoveryConsumesOriginalSelectorsAndDeadline(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	history, err := RestoreAcquisition(AcquisitionFacts{})
	if err != nil {
		t.Fatal(err)
	}
	started, changed, expired, err := history.Start(now, [32]byte{1})
	if err != nil || !changed || expired || started.Facts().Deadline != now.Add(15*time.Second).Unix() || started.Facts().Order != [2]uint8{1, 0} {
		t.Fatalf("start = %+v (%v)", started.Facts(), err)
	}
	contact, disposition, err := started.BeginLatest(1, [32]byte{10})
	if err != nil || !disposition.Contact || !disposition.Changed {
		t.Fatalf("first contact = %+v (%v)", disposition, err)
	}
	if started.Facts().Attempts[1] != 0 {
		t.Fatal("proposal mutated committed history")
	}
	digest, err := contact.BeginDigest(0, [32]byte{20}, [32]byte{11})
	if err != nil {
		t.Fatal(err)
	}
	response, err := digest.CompleteDigest(0, true)
	if err != nil {
		t.Fatal(err)
	}
	// Received bytes are not a recorded authenticated outcome. Recovery must
	// consume this selector instead of interpreting it as available again.
	reopened, err := RestoreAcquisition(response.Facts())
	if err != nil {
		t.Fatal(err)
	}
	resumed, changed, expired, err := reopened.Start(now.Add(3*time.Second), [32]byte{2})
	if err != nil || !changed || expired || resumed.Facts().Deadline != started.Facts().Deadline || resumed.Facts().Seed != started.Facts().Seed || resumed.Facts().Outcomes[2] != 9 {
		t.Fatalf("resume = %+v (%v)", resumed.Facts(), err)
	}
	consumed, disposition, err := resumed.BeginLatest(1, [32]byte{10})
	if err != nil || disposition.Contact || !disposition.Changed || disposition.Outcome != 9 {
		t.Fatalf("interrupted Latest repeated: %+v (%v)", disposition, err)
	}
	if _, err := consumed.BeginDigest(0, [32]byte{21}, [32]byte{11}); err == nil {
		t.Fatal("consumed digest selector replaced")
	}
	again, disposition, err := consumed.BeginLatest(1, [32]byte{10})
	if err != nil || disposition.Contact || disposition.Changed || !reflect.DeepEqual(again.Facts(), consumed.Facts()) {
		t.Fatal("recorded interruption changed on second recovery")
	}
	terminal, changed, expired, err := consumed.Start(now.Add(15*time.Second), [32]byte{2})
	if err != nil || !changed || !expired || terminal.Facts().Active || terminal.Facts().Deadline != started.Facts().Deadline || terminal.Facts().Failures != 1 {
		t.Fatalf("deadline was renewed: %+v (%v)", terminal.Facts(), err)
	}
	if _, _, _, err := terminal.Start(now.Add(16*time.Second), [32]byte{2}); !errors.Is(err, ErrAcquisitionBackoff) {
		t.Fatal("new cycle bypassed durable backoff")
	}
}

func TestAcquisitionClosureRetainsConsumedResults(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	history, err := RestoreAcquisition(AcquisitionFacts{})
	if err != nil {
		t.Fatal(err)
	}
	started, _, _, err := history.Start(now, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	contact, _, err := started.BeginLatest(0, [32]byte{10})
	if err != nil {
		t.Fatal(err)
	}
	finished, err := contact.Finish(now.Add(time.Second), [4]uint8{})
	if err != nil || finished.Facts().Outcomes[0] != 9 || finished.Facts().Attempts[0] != 3 || finished.Facts().Failures != 1 {
		t.Fatalf("lost result was treated as success: %+v (%v)", finished.Facts(), err)
	}
	recovered, err := RestoreAcquisition(finished.Facts())
	if err != nil {
		t.Fatal(err)
	}
	again, err := recovered.Finish(now.Add(2*time.Second), [4]uint8{})
	if err != nil || again.Facts().Outcomes[0] != 9 {
		t.Fatal("missing new observation erased consumed result")
	}
}

func TestAcquisitionHistoryOwnsCopiesAndExposureCap(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	facts := AcquisitionFacts{Exposures: [][32]byte{{10}, {11}}}
	history, err := RestoreAcquisition(facts)
	if err != nil {
		t.Fatal(err)
	}
	facts.Exposures[0] = [32]byte{99}
	history.Facts().Exposures[1] = [32]byte{99}
	if err := history.CheckExposures([2][32]byte{{10}, {11}}); err != nil {
		t.Fatal("caller mutated owned history")
	}
	if err := history.CheckExposures([2][32]byte{{10}, {12}}); err == nil {
		t.Fatal("changed plan exceeded exposure cap")
	}
	started, _, _, err := history.Start(now, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := started.BeginLatest(0, [32]byte{12}); err == nil {
		t.Fatal("attempt bypassed exposure cap")
	}
	for _, index := range []int{-1, 2} {
		if _, _, err := started.BeginLatest(index, [32]byte{10}); err == nil {
			t.Fatal("invalid Source index admitted")
		}
		if _, err := started.BeginDigest(index, [32]byte{20}, [32]byte{10}); err == nil {
			t.Fatal("invalid digest Source index admitted")
		}
		if _, err := started.CompleteDigest(index, true); err == nil {
			t.Fatal("invalid completion Source index admitted")
		}
	}
}

func TestAcquisitionBackoffAndRecoveryPreserveBounds(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	history, err := RestoreAcquisition(AcquisitionFacts{})
	if err != nil {
		t.Fatal(err)
	}
	for iteration, delay := range []int64{30, 60, 120, 240, 480, 900, 900} {
		history, err = history.Finish(now, [4]uint8{2})
		if err != nil || history.Facts().NextAttempt != now.Unix()+delay || history.Facts().Failures != uint64(iteration+1) {
			t.Fatalf("backoff %d = %+v (%v)", iteration, history.Facts(), err)
		}
	}
	finished, err := history.Finish(now, [4]uint8{1, 1})
	if err != nil || finished.Facts().Failures != 0 || finished.Facts().Backoff != 0 || finished.Facts().NextAttempt != 0 {
		t.Fatalf("successful wave did not clear backoff: %+v (%v)", finished.Facts(), err)
	}
	facts := history.Facts()
	facts.Failures = math.MaxInt64
	capped, err := RestoreAcquisition(facts)
	if err != nil {
		t.Fatal(err)
	}
	capped, err = capped.Finish(now, [4]uint8{2})
	if err != nil || capped.Facts().Failures != math.MaxInt64 || capped.Facts().Backoff != 5 {
		t.Fatal("failure count overflowed")
	}
	if _, _, _, err := capped.Start(time.Time{}, [32]byte{}); err == nil {
		t.Fatal("unknown time started a cycle")
	}
	if _, err := capped.Finish(now, [4]uint8{14}); err == nil {
		t.Fatal("unknown outcome became durable history")
	}
	facts = AcquisitionFacts{Attempts: [4]uint8{0, 0, 2}}
	if _, err := RestoreAcquisition(facts); err == nil {
		t.Fatal("digest attempt without selector restored")
	}
}
