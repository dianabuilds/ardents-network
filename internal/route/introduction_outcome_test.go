package route

import (
	"bytes"
	"testing"
	"time"
)

func TestIntroductionOutcomeV1RoundTripsExactBoundedRecords(t *testing.T) {
	ready := IntroductionSlotReady{Reachability: identifier(101), JoinHandle: identifier(102), NotAfter: time.Unix(1_750_000_000, 0).UTC()}
	result := IntroductionDeliveryResult{AttachmentID: identifier(103), Outcome: IntroductionDelivered}
	var buffer bytes.Buffer
	if err := WriteIntroductionSlotReady(&buffer, ready); err != nil {
		t.Fatal(err)
	}
	decodedReady, err := ReadIntroductionSlotReady(&buffer)
	if err != nil || decodedReady != ready {
		t.Fatalf("slot ready = %+v, %v", decodedReady, err)
	}
	if err := WriteIntroductionDeliveryResult(&buffer, result); err != nil {
		t.Fatal(err)
	}
	decodedResult, err := ReadIntroductionDeliveryResult(&buffer)
	if err != nil || decodedResult != result {
		t.Fatalf("delivery result = %+v, %v", decodedResult, err)
	}
}

func TestIntroductionDeliveryResultV1RefusesUnknownOutcome(t *testing.T) {
	if _, err := EncodeIntroductionDeliveryResult(IntroductionDeliveryResult{AttachmentID: identifier(104), Outcome: 99}); err == nil {
		t.Fatal("unknown Introduction outcome was accepted")
	}
}
