package custody

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func TestMeasuredEnvelopeOperationsRetainOnlyNumericAggregates(t *testing.T) {
	capture, err := measurement.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Stop()
	password := []byte("PRIVATE-password-for-envelope")
	defer clear(password)
	envelope, err := sealEnvelope(PurposeVault, []byte("PRIVATE-authority-material"), password)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(envelope)
	if _, plain, _, err := openEnvelope(envelope, []byte("a different long enough password")); err == nil {
		clear(plain)
		t.Fatal("wrong password accepted")
	}
	capture.Stop()
	point := capture.Latest()
	body, err := json.Marshal(point)
	if err != nil || bytes.Contains(body, []byte("PRIVATE")) || len(point.Values) != 2 {
		t.Fatal("private data or missing stages", err, string(body))
	}
	for _, v := range point.Values {
		spec, _ := runtimeevent.LookupInstrument(v.Name).Spec()
		if spec.Owner != "custody" || v.Count != 1 || v.Current != 0 || !v.Available || v.Seconds <= 0 {
			t.Fatal(v)
		}
	}
}
