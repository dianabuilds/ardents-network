package recovery

import (
	"encoding/json"
	"testing"
)

func TestVerifyRejectsS42HostObservationAfterCampaignCompletion(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	cell := &extension.Cells[len(extension.Cells)-1]
	receipt := &cell.Proposals[0].Stopped[0]
	receipt.State.ObservedAtNanos = value.CampaignCompletedAtNanos + 1
	receipt.State.Commitment = processStateCommitment(receipt.State)
	receipt.ObservedAtNanos = receipt.State.ObservedAtNanos - cell.HostStartedAtNanos
	var err error
	value.S42, err = json.Marshal(extension)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict == "pass" {
		t.Fatalf("post-campaign host observation passed: %+v", result)
	}
}
