package recovery

import (
	"encoding/json"
	"testing"
)

func TestVerifyRejectsS42ProcessObservationsAfterRelease(t *testing.T) {
	for name, mutate := range map[string]func(*replacementCell){
		"Endpoint": func(cell *replacementCell) {
			process := cell.HostProcesses["client-endpoint"]
			process.ObservedAtNanos = cell.ActiveStartedAtNanos + 1
			process.HostObservation = processObservationCommitment(process.Host,
				[]byte(process.AdapterProjection), process.PID, true, process.ObservedAtNanos)
			cell.HostProcesses["client-endpoint"] = process
		},
		"Route": func(cell *replacementCell) {
			process := cell.Routes[0].Processes["initiator"]
			moveProcessObservationAfterRelease(cell, &process)
			cell.Routes[0].Processes["initiator"] = process
		},
		"proposal": func(cell *replacementCell) {
			process := cell.Proposals[0].Processes[0]
			moveProcessObservationAfterRelease(cell, &process)
			cell.Proposals[0].Processes[0] = process
		},
	} {
		t.Run(name, func(t *testing.T) {
			value := validS42Evidence(t)
			extension := decodeReplacementTest(t, value.S42)
			mutate(&extension.Cells[0])
			var err error
			value.S42, err = json.Marshal(extension)
			if err != nil {
				t.Fatal(err)
			}
			if result := verifyS42Test(value); result.Verdict != "invalid" {
				t.Fatalf("post-release %s observation accepted: %+v", name, result)
			}
		})
	}
}

func moveProcessObservationAfterRelease(cell *replacementCell, process *candidateProcess) {
	process.ObservedAtNanos = cell.ActiveStartedAtNanos + 1
	process.HostObservation = processObservationCommitment(process.Host, []byte(process.AdapterProjection),
		process.PID, true, process.ObservedAtNanos)
}
