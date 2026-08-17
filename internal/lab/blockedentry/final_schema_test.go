package blockedentry

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestFinalReconciliationCleanupFieldsStrictRoundTrip(t *testing.T) {
	raw := []byte(`{"batch":4,"allocation_delta":0,"fd_delta":0,"socket_delta":0,` +
		`"goroutine_delta":0,"timer_delta":0,"state_bytes_delta":0,"evidence_records_delta":0,` +
		`"cleanup_sockets":2,"cleanup_descendants":3,"cleanup_state_bytes":5,"residuals":0}`)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value finalReconciliation
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		t.Fatalf("strict decode corrected P4 reconciliation: %v", err)
	}
	if value.CleanupSockets != 2 || value.CleanupDescendants != 3 || value.CleanupStateBytes != 5 {
		t.Fatalf("cleanup fields were lost: %+v", value)
	}
	roundTrip, err := json.Marshal(value)
	if err != nil || !bytes.Contains(roundTrip, []byte(`"cleanup_sockets":2`)) ||
		!bytes.Contains(roundTrip, []byte(`"cleanup_descendants":3`)) ||
		!bytes.Contains(roundTrip, []byte(`"cleanup_state_bytes":5`)) {
		t.Fatalf("cleanup fields did not round trip: %s (%v)", roundTrip, err)
	}
}
