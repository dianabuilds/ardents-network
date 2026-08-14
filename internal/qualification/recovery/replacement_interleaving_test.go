package recovery

import (
	"encoding/json"
	"testing"
)

func TestVerifyRejectsS42CrossCategoryCellOverlap(t *testing.T) {
	for name, mutate := range map[string]func(*Evidence, *replacementEvidence){
		"first replacement sequence overlaps base": func(bundle *Evidence, extension *replacementEvidence) {
			extension.Cells[0].HostStartedAtNanos = bundle.Cells[0].HostCompletedAtNanos - 1
		},
		"second base overlaps first replacement sequence": func(bundle *Evidence, extension *replacementEvidence) {
			bundle.Cells[1].HostStartedAtNanos = replacementCellHostEnd(extension.Cells[4],
				extension.Cells[4].HostStartedAtNanos+extension.Cells[4].TerminalNanos) - 1
		},
		"second replacement sequence overlaps base": func(bundle *Evidence, extension *replacementEvidence) {
			extension.Cells[5].HostStartedAtNanos = bundle.Cells[1].HostCompletedAtNanos - 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			bundle := validS42Evidence(t)
			extension := decodeReplacementTest(t, bundle.S42)
			mutate(&bundle, &extension)
			var err error
			bundle.S42, err = json.Marshal(extension)
			if err != nil {
				t.Fatal(err)
			}
			if result := Verify(bundle); result.Verdict == "pass" {
				t.Fatalf("cross-category cell overlap passed: %+v", result)
			}
		})
	}
}
