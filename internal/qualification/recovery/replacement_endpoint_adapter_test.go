package recovery

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestVerifyUsesCommonS42EndpointProcessEvidence(t *testing.T) {
	value := validS42Evidence(t)
	extension := decodeReplacementTest(t, value.S42)
	for index, cell := range extension.Cells {
		if len(cell.HostProcesses) != 4 {
			t.Fatalf("cell %d host process count = %d", index, len(cell.HostProcesses))
		}
	}
	if result := Verify(value); result.Verdict != "pass" {
		t.Fatalf("common Endpoint/Application process evidence was rejected: %+v", result)
	}
}

func endpointProcessTestSet(t *testing.T, scope hostScopeEvidence, imageID string, observedAt int64,
	identities map[string]string) map[string]processObservationEvidence {
	t.Helper()
	result := make(map[string]processObservationEvidence, len(identities))
	serial := uint32(200)
	for service, identity := range identities {
		serial++
		path := "/usr/local/bin/ardents-stream-app"
		if service == "client-endpoint" || service == "publisher-endpoint" {
			path = "/usr/local/bin/ardents-service"
		}
		projection := dockerProcessPublicProjection{Image: imageID, Path: path,
			Project: scope.AdapterProjection, Service: service}
		raw, err := json.Marshal(projection)
		if err != nil {
			t.Fatal(err)
		}
		ref := processEvidenceRef{Adapter: scope.Adapter, Scope: scope.Commitment,
			Executable: dockerProcessProjection("ardents-qualification-executable-v1\x00", imageID, path),
			Tree: dockerProcessProjection("ardents-qualification-process-tree-v1\x00",
				scope.AdapterProjection, service, "", identity),
			Identity: identity, Incarnation: identity + "@2026-01-01T00:00:00Z"}
		ref.Commitment = processRefCommitment(ref)
		observation := max(int64(1), observedAt+int64(serial))
		result[service] = processObservationEvidence{Host: ref, PID: serial,
			ObservedAtNanos: observation, AdapterProjection: string(raw),
			HostObservation: processObservationCommitment(ref, raw, serial, true, observation)}
	}
	if len(result) != 4 || scope.Image != sha256.Sum256([]byte(imageID)) {
		t.Fatal(fmt.Errorf("endpoint process fixture is inconsistent"))
	}
	return result
}
