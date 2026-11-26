package releasefixture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/domain/release"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// InfrastructureReference is the independent verification instant for the signed fixture.
func InfrastructureReference() time.Time {
	return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
}

// InfrastructureDistribution writes the original signed infrastructure test bundle.
func InfrastructureDistribution(t testing.TB, parent string, binary []byte, authority *ReleaseAuthority, version int64, mutate func(map[string]*metadata.TargetFiles), invocation string, unitOverride ...[]byte) (string, string, map[string][]byte) {
	t.Helper()
	if invocation != "source" && invocation != "forwarding" && invocation != "issuer" && invocation != "descriptor" && invocation != "introduction" && invocation != "join" {
		t.Fatal("unsupported test artifact invocation")
	}
	unitName, descriptorName := "ardents-"+invocation+".service", "infrastructure-"+invocation+".json"
	targetName := "ardents/linux-amd64/infrastructure/" + invocation
	expires := InfrastructureReference().Add(365 * 24 * time.Hour)
	if authority == nil {
		authority = NewReleaseAuthority(t, expires)
	}
	unit := []byte("[Unit]\nDescription=Source public template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n")
	if len(unitOverride) == 1 {
		unit = unitOverride[0]
	}
	descriptor := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"infrastructure-fixture\",\"release_version\":%d,\"invocation\":\"source\",\"files\":{\"ardents-node-linux-amd64\":\"%x\",\"ardents-source.service\":\"%x\"}}\n", version, sha256.Sum256(binary), sha256.Sum256(unit)))
	if invocation == "forwarding" {
		descriptor = []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"infrastructure-fixture\",\"release_version\":%d,\"invocation\":\"forwarding\",\"files\":{\"ardents-forwarding.service\":\"%x\",\"ardents-node-linux-amd64\":\"%x\"}}\n", version, sha256.Sum256(unit), sha256.Sum256(binary)))
	}
	if invocation != "source" && invocation != "forwarding" {
		names := []string{unitName, "ardents-node-linux-amd64"}
		sort.Strings(names)
		values := map[string][]byte{unitName: unit, "ardents-node-linux-amd64": binary}
		pairs := []string{}
		for _, name := range names {
			pairs = append(pairs, fmt.Sprintf("%q:%q", name, fmt.Sprintf("%x", sha256.Sum256(values[name]))))
		}
		descriptor = []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"infrastructure-fixture\",\"release_version\":%d,\"invocation\":%q,\"files\":{%s}}\n", version, invocation, strings.Join(pairs, ",")))
	}
	targets := metadata.Targets(expires)
	targets.Signed.Version = version
	for target, body := range map[string][]byte{"ardents/linux-amd64/node": binary, targetName: descriptor} {
		sum := sha256.Sum256(body)
		attestations := []map[string]string{}
		for _, builder := range []string{"fixture-builder-one", "fixture-builder-two"} {
			attestations = append(attestations, map[string]string{"builder_identity": builder, "build_identity": "fixture-build", "source_revision": "fixture-source", "build_input_commitment": "fixture-inputs", "target_sha256": hex.EncodeToString(sum[:])})
		}
		fields := map[string]any{"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": "linux-amd64", "architecture": "amd64", "environment": "h3-test", "network": "new-release-test", "release_identity": "infrastructure-fixture", "release_version": version, "source_revision": "fixture-source", "build_input_commitment": "fixture-inputs", "build_identity": "fixture-build", "dependency_identity": "fixture-dependencies", "sbom_identity": "fixture-sbom", "attestation_policy": "two-builder", "qualification": "qualified", "build_state": "current", "protocol_phase": "announced", "build_safety_no_new_work_after": InfrastructureReference().Add(30 * 24 * time.Hour), "build_safety_terminate_after": InfrastructureReference().Add(180 * 24 * time.Hour), "builder_attestations": attestations}
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		custom := json.RawMessage(raw)
		targets.Signed.Targets[target] = &metadata.TargetFiles{Path: target, Length: int64(len(body)), Hashes: metadata.Hashes{"sha256": sum[:]}, Custom: &custom}
	}
	if mutate != nil {
		mutate(targets.Signed.Targets)
	}
	authority.Sign(t, targets)
	tb, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	td := sha256.Sum256(tb)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Version = version
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(tb)), Hashes: metadata.Hashes{"sha256": td[:]}}
	authority.Sign(t, snapshot)
	sb, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	sd := sha256.Sum256(sb)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Version = version
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(sb)), Hashes: metadata.Hashes{"sha256": sd[:]}}
	authority.Sign(t, timestamp)
	ts, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"ardents-node-linux-amd64": binary, unitName: unit, descriptorName: descriptor, "root.json": authority.RootBytes(), "timestamp.json": ts, "snapshot.json": sb, "targets.json": tb}
	directory := filepath.Join(parent, "distribution")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(directory, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(directory, "ardents-node-linux-amd64"), 0700); err != nil {
		t.Fatal(err)
	}
	pin := WriteInfrastructureOracleInvocationInventory(t, directory, files, version, invocation)
	return directory, pin, files
}

// WriteInfrastructureOracleInvocationInventory independently records fixture artifact digests.
func WriteInfrastructureOracleInvocationInventory(t testing.TB, dir string, files map[string][]byte, version int64, invocation string) string {
	t.Helper()
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := []string{}
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	raw := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"infrastructure-fixture\",\"release_version\":%d,\"invocation\":%q,\"files\":{%s}}\n", version, invocation, strings.Join(pairs, ",")))
	if err := os.WriteFile(filepath.Join(dir, "infrastructure-enrollment.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// InfrastructureCandidateInputs supplies detached bytes to the real Release verifier.
func InfrastructureCandidateInputs(t testing.TB, c enrollment.InfrastructureCandidate) release.Inputs {
	t.Helper()
	root, _ := c.File("root.json")
	ts, _ := c.File("timestamp.json")
	sn, _ := c.File("snapshot.json")
	targets, _ := c.File("targets.json")
	program, _ := c.File("ardents-node-linux-amd64")
	files, err := release.StaticMetadataFiles(root, ts, sn, targets)
	if err != nil {
		t.Fatal(err)
	}
	return release.Inputs{RootBytes: root, Files: files, Artifact: program, TargetPath: "ardents/linux-amd64/node", Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "h3-test", Network: "new-release-test", RefTime: InfrastructureReference()}}
}
