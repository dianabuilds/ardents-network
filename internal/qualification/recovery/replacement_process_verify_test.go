package recovery

import (
	"crypto/sha256"
	"encoding/json"
	"testing"
)

func testHostScope(sourceCommit, imageID, manifestDigest string) hostScopeEvidence {
	result := hostScopeEvidence{Adapter: "docker-compose-v1", AdapterProjection: "ardents-recovery-test",
		Machine: [32]byte{90}, Source: sha256.Sum256([]byte(sourceCommit)), Image: sha256.Sum256([]byte(imageID))}
	result.Campaign = sha256.Sum256([]byte(result.AdapterProjection + "\x00" + manifestDigest))
	result.Commitment = hostScopeCommitment(result)
	return result
}

func testProcessRef(scope hostScopeEvidence, identity, incarnation string) processEvidenceRef {
	return processEvidenceRef{Adapter: scope.Adapter, Scope: scope.Commitment,
		Identity: identity, Incarnation: incarnation}
}

func testObservedProcess(t *testing.T, value candidateProcess, scope hostScopeEvidence, imageID string) candidateProcess {
	t.Helper()
	projection := dockerProcessPublicProjection{Image: imageID, Path: "/usr/local/bin/ardents-route",
		Project: scope.AdapterProjection, Service: value.Service, Arguments: []string{"--plan", "/run/plan.json"}}
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	value.AdapterProjection = string(raw)
	value.Host.Executable = dockerProcessProjection("ardents-qualification-executable-v1\x00",
		append([]string{projection.Image, projection.Path}, projection.Arguments...)...)
	value.Host.Tree = dockerProcessProjection("ardents-qualification-process-tree-v1\x00",
		projection.Project, projection.Service, projection.PIDMode, value.ContainerID)
	value.Host.Commitment = processRefCommitment(value.Host)
	value.HostObservation = processObservationCommitment(value.Host, []byte(value.AdapterProjection),
		value.PID, true, value.ObservedAtNanos)
	return value
}

func testStoppedReceipt(process candidateProcess, hostStartedAt, at, faultStart int64) failedResourceReceipt {
	state := processStateEvidence{Resource: process.Host, State: "stopped", ObservedAtNanos: hostStartedAt + at}
	state.Commitment = processStateCommitment(state)
	result := failedResourceReceipt{ContainerID: process.ContainerID, ObservedAtNanos: at, State: state}
	if faultStart > 0 {
		result.Fault = processFaultEvidence{Resource: process.Host, Kind: "stop", State: "stopped",
			InvocationStartedNanos:   hostStartedAt + faultStart,
			InvocationCompletedNanos: hostStartedAt + faultStart + 2,
			ObservedAtNanos:          hostStartedAt + faultStart + 1}
		result.Fault.Commitment = processFaultCommitment(result.Fault)
	}
	return result
}
