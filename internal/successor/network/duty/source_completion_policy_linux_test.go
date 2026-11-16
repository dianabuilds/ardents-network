package duty

import "testing"

func TestSourceCompletionInspectionPolicyRefusesUnjoinedProducer(t *testing.T) {
	for _, phase := range []string{"live", "prepared", "quarantined"} {
		// The original deadline is deliberately long expired. These detached
		// records isolate inspection policy; they grant no physical custody.
		if err := sourceInspectionRecords([]dutyRecord{{Class: "direct-source", State: phase, NotAfter: 1}}); err == nil {
			t.Fatal("unjoined producer supplied completion", phase)
		}
	}
	for _, records := range [][]dutyRecord{nil, {{Class: "direct-source", State: "exposed", NotAfter: 1}}} {
		if err := sourceInspectionRecords(records); err != nil {
			t.Fatal("retained exposure policy refused", err)
		}
	}
}
