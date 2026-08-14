package recovery

import (
	"encoding/json"
	"testing"
)

func TestCommonS42ProcessRulesAcceptAdapterNeutralEvidence(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	hostScope := decodeHostScopeTest(t, value.HostScope)
	nativeTestReplacementProcesses(&extension, &hostScope)
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
			value.Manifest.RouteManifest, hostScope, identities); result.Verdict != "pass" {
			t.Fatalf("cell %d adapter-neutral proposals were rejected: %+v", cellIndex, result)
		}
		if result := verifyReplacementRoutes(*cell, byRole, extension.RouteCase.SelectionSeed,
			hostScope, identities); result.Verdict != "pass" {
			t.Fatalf("cell %d adapter-neutral routes were rejected: %+v", cellIndex, result)
		}
	}
}

func TestVerifyRejectsUnsupportedS42Adapter(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	hostScope := decodeHostScopeTest(t, value.HostScope)
	nativeTestReplacementProcesses(&extension, &hostScope)
	value.HostScope = encodeHostScopeTest(t, hostScope)
	var err error
	value.S42, err = json.Marshal(extension)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict != "invalid" {
		t.Fatalf("unsupported Adapter verdict = %+v, want invalid", result)
	}
}

func TestVerifyRejectsChangedS42HostScope(t *testing.T) {
	value := validS42Evidence(t)
	hostScope := decodeHostScopeTest(t, value.HostScope)
	hostScope.Machine[0]++
	hostScope.Commitment = hostScopeCommitment(hostScope)
	value.HostScope = encodeHostScopeTest(t, hostScope)
	if result := Verify(value); result.Verdict == "pass" {
		t.Fatal("changed campaign HostScope passed existing process observations")
	}
}

func TestCommonS42ProcessRulesRejectIdentityProjectionReuse(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	hostScope := decodeHostScopeTest(t, value.HostScope)
	nativeTestReplacementProcesses(&extension, &hostScope)
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
		value.Manifest.RouteManifest, hostScope, map[string]bool{}); result.Verdict == "pass" {
		t.Fatal("one process identity represented multiple projections and Node candidates")
	}
}

func nativeTestReplacementProcesses(extension *replacementEvidence, scope *hostScopeEvidence) {
	scope.Adapter = "native-host-v1"
	scope.AdapterProjection = "native-test-machine"
	scope.Commitment = hostScopeCommitment(*scope)
	for cellIndex := range extension.Cells {
		cell := &extension.Cells[cellIndex]
		for role, process := range cell.HostProcesses {
			process.Host.Adapter, process.Host.Scope = scope.Adapter, scope.Commitment
			process.Host.Identity, process.Host.Incarnation = "process/"+process.Host.Identity, "boot/"+process.Host.Incarnation
			process.Host.Commitment = processRefCommitment(process.Host)
			process.AdapterProjection = `{"kind":"native-host-v1"}`
			process.HostObservation = processObservationCommitment(process.Host, []byte(process.AdapterProjection),
				process.PID, true, process.ObservedAtNanos)
			cell.HostProcesses[role] = process
		}
		for routeIndex := range cell.Routes {
			for role, process := range cell.Routes[routeIndex].Processes {
				cell.Routes[routeIndex].Processes[role] = nativeTestProcess(process, *scope)
			}
		}
		for proposalIndex := range cell.Proposals {
			proposal := &cell.Proposals[proposalIndex]
			for processIndex, process := range proposal.Processes {
				proposal.Processes[processIndex] = nativeTestProcess(process, *scope)
				proposal.Stopped[processIndex] = nativeTestState(proposal.Stopped[processIndex],
					proposal.Processes[processIndex])
			}
			for role, timing := range proposal.PlanTimings {
				process, _ := replacementPlanProcess(*cell, *proposal, role)
				timing.Process = process
				proposal.PlanTimings[role] = timing
			}
		}
		for eventIndex := range cell.Events {
			event := &cell.Events[eventIndex]
			event.Failed = nativeTestProcess(event.Failed, *scope)
			event.Replacement = nativeTestProcess(event.Replacement, *scope)
			event.RendezvousBefore = nativeTestProcess(event.RendezvousBefore, *scope)
			event.RendezvousAfter = nativeTestProcess(event.RendezvousAfter, *scope)
			event.Introduction = nativeTestProcess(event.Introduction, *scope)
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
