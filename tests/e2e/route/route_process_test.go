package route_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestAuthenticatedRouteUsesSeparateRoleProcesses(t *testing.T) {
	fixture := newProcessFixture(t)
	binary := buildRouteBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	publisherPlan := writeProcessPlan(t, map[string]any{
		"Role": "publisher", "ManifestDigest": hex32(fixture.manifestDigest),
		"NetworkID": hex32(fixture.network), "EpochDigest": hex32(fixture.epochDigest),
		"NodeID": hex32(fixture.publisherID), "Listen": fixture.addresses[4],
		"Certificate": fixture.identities[4].cert, "Key": fixture.identities[4].key,
		"UpstreamPin":        hex32(fixture.identities[3].public),
		"ServiceCertificate": fixture.identities[4].cert, "ServiceKey": fixture.identities[4].key, "Deadline": "5s",
	})
	processes := []*routeProcess{startRouteProcess(t, ctx, binary, publisherPlan)}
	for index, position := range fixture.plan.Positions {
		upstream := fixture.identities[5].public
		if index > 0 {
			upstream = fixture.identities[index-1].public
		}
		nextID, nextAddress, nextPin := fixture.publisherID, fixture.addresses[4], fixture.identities[4].public
		if index < 3 {
			nextID, nextAddress, nextPin = fixture.plan.Positions[index+1].NodeID, fixture.addresses[index+1], fixture.identities[index+1].public
		}
		plan := writeProcessPlan(t, map[string]any{
			"Role": position.Role, "ManifestDigest": hex32(fixture.manifestDigest),
			"NetworkID": hex32(fixture.network), "EpochDigest": hex32(fixture.epochDigest),
			"NodeID": hex32(position.NodeID), "Listen": fixture.addresses[index],
			"Certificate": fixture.identities[index].cert, "Key": fixture.identities[index].key,
			"UpstreamPin": hex32(upstream), "NextNodeID": hex32(nextID), "Next": nextAddress,
			"NextPin": hex32(nextPin), "Deadline": "5s",
		})
		processes = append(processes, startRouteProcess(t, ctx, binary, plan))
	}
	authority := fixture.authority.Public().(ed25519.PublicKey)
	clientPlan := writeProcessPlan(t, map[string]any{
		"Role": "client", "ManifestDigest": hex32(fixture.manifestDigest),
		"StateRoot": fixture.stateRoot, "NetworkID": hex32(fixture.network),
		"LocalRoleStateRoot": filepath.Join(filepath.Dir(fixture.stateRoot), "local-roles"),
		"Authorities":        []string{hex.EncodeToString(authority)}, "Threshold": 1,
		"At": fixture.now.Format(time.RFC3339), "Seed": hex32(fixture.selectionSeed),
		"Certificate": fixture.identities[5].cert, "Key": fixture.identities[5].key,
		"PublisherPin": hex32(fixture.identities[4].public), "Deadline": "5s",
	})
	client := exec.CommandContext(ctx, binary, "run", clientPlan)
	clientOutput, err := client.CombinedOutput()
	if err != nil {
		t.Fatalf("Client process failed: %v\n%s", err, clientOutput)
	}
	var clientEvidence route.Evidence
	if err := json.Unmarshal(bytes.TrimSpace(clientOutput), &clientEvidence); err != nil {
		t.Fatalf("decode Client evidence: %v\n%s", err, clientOutput)
	}
	if clientEvidence.Role != "client" || clientEvidence.PID == 0 || len(clientEvidence.Positions) != 4 ||
		clientEvidence.Generation == "" || clientEvidence.CanaryLength != 32 || len(clientEvidence.Canary) != 32 {
		t.Fatalf("Client evidence is incomplete: %+v", clientEvidence)
	}
	completed := make([]route.Evidence, 0, 5)
	for _, process := range processes {
		completed = append(completed, process.finish(t))
	}
	pids := map[int]bool{clientEvidence.PID: true}
	seenRoles := map[string]bool{}
	for _, evidence := range completed {
		if evidence.PID == 0 || pids[evidence.PID] || evidence.NetworkID != fixture.network || evidence.EpochDigest != fixture.epochDigest {
			t.Fatalf("process identity/state evidence collapsed: %+v", evidence)
		}
		pids[evidence.PID], seenRoles[evidence.Role] = true, true
		if evidence.Role != "publisher" && (evidence.OpaqueBytes == 0 || evidence.CanaryDigest == clientEvidence.CanaryDigest) {
			t.Fatalf("Route Node did not retain an opaque-only observation: %+v", evidence)
		}
	}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		if !seenRoles[role] {
			t.Fatalf("missing process role %q", role)
		}
	}
	for _, evidence := range completed {
		if evidence.Role == "publisher" && (evidence.CanaryLength != 32 || evidence.CanaryDigest != clientEvidence.CanaryDigest) {
			t.Fatalf("Publisher did not receive the exact Client canary: %+v", evidence)
		}
	}
	for _, address := range fixture.addresses {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("owned listener remained after cleanup at %s: %v", address, err)
		}
		listener.Close()
	}
}

func TestEachRouteProcessCarriesFourTimesReferenceAttachments(t *testing.T) {
	fixture := newProcessFixture(t)
	binary := buildRouteBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	capacity := uint16(16)
	publisherPlan := writeProcessPlan(t, map[string]any{
		"Role": "publisher", "ManifestDigest": hex32(fixture.manifestDigest),
		"NetworkID": hex32(fixture.network), "EpochDigest": hex32(fixture.epochDigest),
		"NodeID": hex32(fixture.publisherID), "Listen": fixture.addresses[4],
		"Certificate": fixture.identities[4].cert, "Key": fixture.identities[4].key,
		"UpstreamPin": hex32(fixture.identities[3].public), "MaximumAttachments": capacity,
		"AttachmentTarget":   capacity,
		"ServiceCertificate": fixture.identities[4].cert, "ServiceKey": fixture.identities[4].key, "Deadline": "5s",
	})
	processes := []*routeProcess{startRouteProcess(t, ctx, binary, publisherPlan)}
	for index, position := range fixture.plan.Positions {
		upstream := fixture.identities[5].public
		if index > 0 {
			upstream = fixture.identities[index-1].public
		}
		nextID, nextAddress, nextPin := fixture.publisherID, fixture.addresses[4], fixture.identities[4].public
		if index < 3 {
			nextID, nextAddress, nextPin = fixture.plan.Positions[index+1].NodeID, fixture.addresses[index+1], fixture.identities[index+1].public
		}
		plan := writeProcessPlan(t, map[string]any{
			"Role": position.Role, "ManifestDigest": hex32(fixture.manifestDigest),
			"NetworkID": hex32(fixture.network), "EpochDigest": hex32(fixture.epochDigest),
			"NodeID": hex32(position.NodeID), "Listen": fixture.addresses[index],
			"Certificate": fixture.identities[index].cert, "Key": fixture.identities[index].key,
			"UpstreamPin": hex32(upstream), "NextNodeID": hex32(nextID), "Next": nextAddress,
			"NextPin": hex32(nextPin), "MaximumAttachments": capacity, "AttachmentTarget": capacity,
			"Deadline": "5s",
		})
		processes = append(processes, startRouteProcess(t, ctx, binary, plan))
	}
	for _, address := range fixture.addresses {
		connection, dialErr := net.Dial("tcp", address)
		if dialErr != nil {
			t.Fatal(dialErr)
		}
		if _, writeErr := connection.Write([]byte("not-a-TLS-setup")); writeErr != nil {
			t.Fatal(writeErr)
		}
		_ = connection.Close()
	}
	time.Sleep(100 * time.Millisecond)
	authority := fixture.authority.Public().(ed25519.PublicKey)
	type clientOutcome struct {
		output []byte
		err    error
	}
	clients := make(chan clientOutcome, capacity+1)
	stateRoots := []string{fixture.stateRoot}
	for index := uint16(1); index < capacity+1; index++ {
		stateRoots = append(stateRoots, copyProcessState(t, fixture.stateRoot))
	}
	for index := range capacity + 1 {
		clientPlan := writeProcessPlan(t, map[string]any{
			"Role": "client", "ManifestDigest": hex32(fixture.manifestDigest), "StateRoot": stateRoots[index],
			"LocalRoleStateRoot": stateRoots[index] + "-local-roles",
			"NetworkID":          hex32(fixture.network), "Authorities": []string{hex.EncodeToString(authority)}, "Threshold": 1,
			"At": fixture.now.Format(time.RFC3339), "Seed": hex32(fixture.selectionSeed),
			"Certificate": fixture.identities[5].cert, "Key": fixture.identities[5].key,
			"PublisherPin": hex32(fixture.identities[4].public), "Deadline": "5s",
		})
		go func(plan string) {
			output, err := exec.CommandContext(ctx, binary, "run", plan).CombinedOutput()
			clients <- clientOutcome{output: output, err: err}
		}(clientPlan)
	}
	succeeded, refused := 0, 0
	for range capacity + 1 {
		client := <-clients
		var evidence route.Evidence
		if client.err == nil && json.Unmarshal(bytes.TrimSpace(client.output), &evidence) == nil && evidence.CanaryLength == 32 {
			succeeded++
		} else {
			refused++
		}
	}
	if succeeded != int(capacity) || refused != 1 {
		t.Fatalf("authenticated overload result: succeeded=%d refused=%d", succeeded, refused)
	}
	for _, process := range processes {
		evidence := process.finish(t)
		if evidence.AttachmentsCompleted != capacity || evidence.AttachmentsAbandoned != 1 || !evidence.PeerAuthenticated {
			t.Fatalf("role process capacity was not real: %+v", evidence)
		}
		if evidence.Role == "publisher" && evidence.CanaryLength != uint32(capacity)*32 {
			t.Fatalf("publisher capacity omitted useful-work evidence: %+v", evidence)
		}
		if evidence.Role == "initiator" && evidence.AttachmentsRefused == 0 {
			t.Fatalf("initiator omitted authenticated overload evidence: %+v", evidence)
		}
	}
}

func copyProcessState(t *testing.T, source string) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "state")
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	return destination
}

func TestClientProcessRejectsExcludingItsOnlyCandidate(t *testing.T) {
	fixture := newProcessFixture(t)
	binary := buildRouteBinary(t)
	authority := fixture.authority.Public().(ed25519.PublicKey)
	base := map[string]any{
		"Role": "client", "ManifestDigest": hex32(fixture.manifestDigest),
		"StateRoot": fixture.stateRoot, "NetworkID": hex32(fixture.network),
		"LocalRoleStateRoot": filepath.Join(filepath.Dir(fixture.stateRoot), "local-roles-exclusion"),
		"Authorities":        []string{hex.EncodeToString(authority)}, "Threshold": 1,
		"At": fixture.now.Format(time.RFC3339), "Seed": hex32(fixture.selectionSeed),
		"Certificate": fixture.identities[5].cert, "Key": fixture.identities[5].key,
		"PublisherPin": hex32(fixture.identities[4].public), "Deadline": "5s",
	}
	tests := []struct {
		name  string
		field string
		value string
	}{
		{"identity", "ExcludedIdentities", hex32(fixture.snapshot.Candidates[0].NodeID)},
		{"family", "ExcludedFamilies", fixture.snapshot.Candidates[0].Family},
		{"domain", "ExcludedDomains", fixture.snapshot.Candidates[0].Domain},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := make(map[string]any, len(base)+1)
			for key, value := range base {
				plan[key] = value
			}
			plan[test.field] = []string{test.value}
			output, err := exec.Command(binary, "run", writeProcessPlan(t, plan)).CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte("cannot fill every Route position")) {
				t.Fatalf("excluded %s did not fail Route selection: err=%v\n%s", test.name, err, output)
			}
		})
	}
}

type routeProcess struct {
	command *exec.Cmd
	decoder *json.Decoder
	stderr  bytes.Buffer
}

func startRouteProcess(t *testing.T, ctx context.Context, binary, plan string) *routeProcess {
	t.Helper()
	process := &routeProcess{command: exec.CommandContext(ctx, binary, "run", plan)}
	stdout, err := process.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process.command.Stderr = &process.stderr
	process.decoder = json.NewDecoder(stdout)
	if err := process.command.Start(); err != nil {
		t.Fatal(err)
	}
	var ready route.Evidence
	if err := process.decoder.Decode(&ready); err != nil || ready.Kind != "ready" {
		process.command.Process.Kill()
		t.Fatalf("role did not become ready: %+v err=%v stderr=%s", ready, err, process.stderr.String())
	}
	return process
}

func (process *routeProcess) finish(t *testing.T) route.Evidence {
	t.Helper()
	var evidence route.Evidence
	if err := process.decoder.Decode(&evidence); err != nil {
		process.command.Process.Kill()
		t.Fatalf("decode terminal role evidence: %v stderr=%s", err, process.stderr.String())
	}
	if err := process.command.Wait(); err != nil {
		t.Fatalf("role process failed: %v evidence=%+v stderr=%s", err, evidence, process.stderr.String())
	}
	if evidence.Kind != "complete" || evidence.Error != "" {
		t.Fatalf("role process returned incomplete evidence: %+v", evidence)
	}
	return evidence
}

func writeProcessPlan(t *testing.T, value any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "role.json")
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildRouteBinary(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	name := "ardents-route"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-trimpath", "-o", path, "./cmd/ardents-route")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build Route command: %v\n%s", err, output)
	}
	return path
}
