package recovery

import (
	"encoding/json"
	"testing"
)

func TestCommonS42ProcessRulesAcceptAdapterNeutralEvidence(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	nativeTestReplacementProcesses(&extension)
	byRole, err := verifyReplacementCandidates(extension.Candidates)
	if err != nil {
		t.Fatal(err)
	}
	for cellIndex := range extension.Cells {
		cell := &extension.Cells[cellIndex]
		identities := map[string]bool{}
		for _, process := range cell.HostProcesses {
			identities[process.Host.Identity] = true
		}
		if result := verifyReplacementProposals(*cell, byRole, extension.RouteCase,
			value.Manifest.RouteManifest, extension.HostScope, identities); result.Verdict != "pass" {
			t.Fatalf("cell %d adapter-neutral proposals were rejected: %+v", cellIndex, result)
		}
		if result := verifyReplacementRoutes(*cell, byRole, extension.RouteCase.SelectionSeed,
			extension.HostScope, identities); result.Verdict != "pass" {
			t.Fatalf("cell %d adapter-neutral routes were rejected: %+v", cellIndex, result)
		}
	}
}

func TestVerifyRejectsUnsupportedS42Adapter(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	nativeTestReplacementProcesses(&extension)
	var err error
	value.S42, err = json.Marshal(extension)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict != "invalid" {
		t.Fatalf("unsupported Adapter verdict = %+v, want invalid", result)
	}
}

func TestCommonS42ProcessRulesRejectIdentityProjectionReuse(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	nativeTestReplacementProcesses(&extension)
	cell := &extension.Cells[0]
	source, reused := cell.Proposals[0].Processes[0], &cell.Proposals[0].Processes[1]
	reused.Host.Identity, reused.Host.Incarnation = source.Host.Identity, source.Host.Incarnation
	reused.Host.Executable[0]++
	reused.Host.Tree[0]++
	reused.Host.Commitment = processRefCommitment(reused.Host)
	reused.HostObservation = processObservationCommitment(reused.Host, []byte(reused.AdapterProjection),
		reused.PID, true, reused.ObservedAtNanos)
	cell.Proposals[0].Stopped[1] = nativeTestState(cell.Proposals[0].Stopped[1], *reused)
	byRole, err := verifyReplacementCandidates(extension.Candidates)
	if err != nil {
		t.Fatal(err)
	}
	if result := verifyReplacementProposals(*cell, byRole, extension.RouteCase,
		value.Manifest.RouteManifest, extension.HostScope, map[string]bool{}); result.Verdict == "pass" {
		t.Fatal("one process identity represented multiple projections and Node candidates")
	}
}

func nativeTestReplacementProcesses(extension *replacementEvidence) {
	extension.HostScope.Adapter = "native-host-v1"
	extension.HostScope.AdapterProjection = "native-test-machine"
	extension.HostScope.Commitment = hostScopeCommitment(extension.HostScope)
	for cellIndex := range extension.Cells {
		cell := &extension.Cells[cellIndex]
		for routeIndex := range cell.Routes {
			for role, process := range cell.Routes[routeIndex].Processes {
				cell.Routes[routeIndex].Processes[role] = nativeTestProcess(process, extension.HostScope)
			}
		}
		for proposalIndex := range cell.Proposals {
			proposal := &cell.Proposals[proposalIndex]
			for processIndex, process := range proposal.Processes {
				proposal.Processes[processIndex] = nativeTestProcess(process, extension.HostScope)
				proposal.Stopped[processIndex] = nativeTestState(proposal.Stopped[processIndex],
					proposal.Processes[processIndex])
			}
		}
		for eventIndex := range cell.Events {
			event := &cell.Events[eventIndex]
			event.Failed = nativeTestProcess(event.Failed, extension.HostScope)
			event.Replacement = nativeTestProcess(event.Replacement, extension.HostScope)
			event.RendezvousBefore = nativeTestProcess(event.RendezvousBefore, extension.HostScope)
			event.RendezvousAfter = nativeTestProcess(event.RendezvousAfter, extension.HostScope)
			event.Introduction = nativeTestProcess(event.Introduction, extension.HostScope)
			event.FailedResource = nativeTestState(event.FailedResource, event.Failed)
			event.FailedResource.Fault.Resource = event.Failed.Host
			event.FailedResource.Fault.Commitment = processFaultCommitment(event.FailedResource.Fault)
		}
	}
}

func nativeTestProcess(process candidateProcess, scope hostScopeEvidence) candidateProcess {
	legacyIdentity, legacyIncarnation := process.ContainerID, process.Incarnation
	process.Host.Adapter = scope.Adapter
	process.Host.Scope = scope.Commitment
	process.Host.Identity = "process/" + legacyIdentity
	process.Host.Incarnation = "boot/" + legacyIncarnation
	process.Host.Commitment = processRefCommitment(process.Host)
	process.AdapterProjection = `{"kind":"native-host-v1"}`
	process.HostObservation = processObservationCommitment(process.Host, []byte(process.AdapterProjection),
		process.PID, true, process.ObservedAtNanos)
	process.Service, process.ContainerID, process.Incarnation = "", "", ""
	return process
}

func nativeTestState(receipt failedResourceReceipt, process candidateProcess) failedResourceReceipt {
	receipt.ContainerID = ""
	receipt.State.Resource = process.Host
	receipt.State.Commitment = processStateCommitment(receipt.State)
	return receipt
}
