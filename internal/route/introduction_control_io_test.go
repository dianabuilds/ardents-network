package route

import (
	"bytes"
	"testing"
	"time"
)

func TestIntroductionControlRecordPreservesSealedCanonicalBytes(t *testing.T) {
	input := introductionFixture()
	input.Enc = bytes.Repeat([]byte{1}, encapsulationLength)
	input.Ciphertext = bytes.Repeat([]byte{2}, minimumCiphertext)
	want, err := EncodeSealedIntroduction(input)
	if err != nil {
		t.Fatal(err)
	}
	record, err := ReadIntroductionControlRecord(bytes.NewReader(want))
	if err != nil || record.Sealed == nil || record.Registration != nil || !bytes.Equal(record.Raw, want) || !equalIntroduction(*record.Sealed, input) {
		t.Fatalf("ReadIntroductionControlRecord = %+v, %v", record, err)
	}
}

func TestIntroductionControlRecordRecognizesOnlyClosedForms(t *testing.T) {
	var wire bytes.Buffer
	registration := IntroductionSlotRegistration{Reachability: identifier(81), JoinHandle: identifier(82),
		NotAfter: time.Unix(1_750_000_000, 0).UTC()}
	if err := WriteIntroductionSlotRegistration(&wire, registration); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIntroductionControlRecord(&wire)
	if err != nil || record.Registration == nil || *record.Registration != registration || record.Sealed != nil {
		t.Fatalf("registration record = %+v, %v", record, err)
	}
	if _, err := ReadIntroductionControlRecord(bytes.NewReader([]byte("not a route record"))); err == nil {
		t.Fatal("malformed control record was accepted")
	}
}
