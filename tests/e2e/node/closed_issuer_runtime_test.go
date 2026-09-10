package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// All State and issuer material below is accepted through product commands;
// the Node's own runtime opens State and obtains its current duty projection.
func runClosedIssuerProcess(t *testing.T, node, endpoint string, acceptArguments []string, signedProfile, issuerRoot string,
	network, issuer [32]byte, authority, identity ed25519.PrivateKey, now time.Time, nodeCount int, exchange func(bool), participant func(string, map[string]any)) {
	t.Helper()
	public := authority.Public().(ed25519.PublicKey)
	clientAuthority := makeAuthority(t, "issuer-command-source-client")
	client := makeLeaf(t, clientAuthority, "issuer-command-source-client.test", false)
	sources := make([]map[string]any, 2)
	for index := range sources {
		root, roleRoot := t.TempDir(), t.TempDir()
		if err := os.Chmod(roleRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		args := append([]string(nil), acceptArguments...)
		args[2], args[16] = root, acceptArguments[16]+".1"
		runProvisioningCommand(t, endpoint, args...)
		name := fmt.Sprintf("issuer-command-source-%d.test", index)
		server := makeLeaf(t, makeAuthority(t, name), name, true)
		address := freeAddress(t)
		plan := nativeDutySourcePlan(network, public, time.Now().UTC(), root, roleRoot, address, server, clientAuthority.root, client.sourcePin)
		delete(plan, "native_rendezvous_profile")
		plan["state_profile"], plan["state_profile_authority"], plan["materialization_index"] = "ardents-route-v3", hex.EncodeToString(public), 1
		stop := startSource(t, node, writeJSON(t, fmt.Sprintf("issuer-source-%d.json", index), plan))
		t.Cleanup(stop)
		digest := sha256.Sum256([]byte(name))
		sources[index] = map[string]any{"address": address, "server_name": name, "identity": hex.EncodeToString(digest[:]),
			"family": fmt.Sprintf("issuer-source-%d", index), "endpoint_handle": name, "root_ca": server.root,
			"leaf_key_digest": hex.EncodeToString(server.sourcePin[:])}
	}
	root, roleRoot := t.TempDir(), t.TempDir()
	if err := os.Chmod(roleRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	args := append([]string(nil), acceptArguments...)
	args[2], args[16] = root, acceptArguments[16]+".1"
	args = append(args, "--closed-profile", signedProfile)
	runProvisioningCommand(t, endpoint, args...)
	clock := filepath.Join(t.TempDir(), "issuer.clock")
	t.Cleanup(startClockObserver(t, clock))
	certificate, key := closedIssuerListenCredential(t, identity, now)
	order := sha256.Sum256([]byte("issuer-command-source-order"))
	plan := map[string]any{"schema": "ardents-node-plan-v1", "state_root": root, "local_role_state_root": roleRoot,
		"network_id": hex.EncodeToString(network[:]), "authority_public": []string{hex.EncodeToString(public)}, "threshold": 1,
		"closed_profile_authority": hex.EncodeToString(public), "server_certificate": certificate, "server_key": key,
		"node_id": hex.EncodeToString(issuer[:]), "identity_key": key, "clock_observation_file": clock,
		"materialization_index": 1, "order_seed": hex.EncodeToString(order[:]), "sources": sources,
		"source_client_certificate": client.certificate, "source_client_key": client.key,
		"closed_issuer": map[string]any{"root": issuerRoot, "admission_root": t.TempDir(), "connection_limit": 2, "drain_timeout_ms": 1000}}
	path := writeJSON(t, "closed-issuer-node.json", plan)
	first := startNodeCommand(t, node, "issuer", "serve", "--config", path)
	t.Cleanup(func() { stopClosedIssuerProcess(t, first) })
	ready := waitNodeState(t, first, "READY", 10*time.Second)
	if ready.Epoch != 1 || ready.AssignmentDigest == [32]byte{} {
		t.Fatalf("closed issuer has no accepted duty binding: %+v", ready)
	}
	resolutionRoot := ""
	live := []*nodeProcess{first}
	for index, role := range closedTextTopologyRoles(nodeCount) {
		if index == 1 {
			continue
		}
		forwardRoot := t.TempDir()
		forwardArgs := append([]string(nil), acceptArguments...)
		forwardArgs[2] = forwardRoot
		if index != 0 {
			forwardArgs[16] += fmt.Sprintf(".%d", index)
		}
		forwardArgs = append(forwardArgs, "--closed-profile", signedProfile)
		runProvisioningCommand(t, endpoint, forwardArgs...)
		forwardPlan := make(map[string]any, len(plan))
		for name, value := range plan {
			forwardPlan[name] = value
		}
		delete(forwardPlan, "closed_issuer")
		forwardPlan["state_root"] = forwardRoot
		forwardRoles := t.TempDir()
		if err := os.Chmod(forwardRoles, 0o700); err != nil {
			t.Fatal(err)
		}
		forwardPlan["local_role_state_root"] = forwardRoles
		forwardPlan["node_id"], forwardPlan["materialization_index"] = identifierNode(byte(index+1)), index
		forwardKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(6 + index)}, ed25519.SeedSize))
		cert, keyPath := closedIssuerListenCredential(t, forwardKey, now)
		forwardPlan["identity_key"], forwardPlan["server_key"], forwardPlan["server_certificate"] = keyPath, keyPath, cert
		reservation := map[string]any{"connection_limit": 2, "drain_timeout_ms": 1000}
		switch role {
		case [2]uint8{2, 5}:
			reservation["root"], reservation["admission_root"] = t.TempDir(), t.TempDir()
			forwardPlan["closed_resolution"] = reservation
			resolutionRoot = reservation["root"].(string)
		case [2]uint8{4, 3}:
			reservation["admission_root"] = t.TempDir()
			forwardPlan["closed_introduction"] = reservation
		case [2]uint8{2, 4}:
			reservation["admission_root"] = t.TempDir()
			forwardPlan["closed_data_join"] = reservation
		default:
			reservation["root"] = t.TempDir()
			forwardPlan["closed_forwarding"] = reservation
		}
		process := startNodeCommand(t, node, "node", "--config", writeJSON(t, fmt.Sprintf("closed-node-%d.json", index+1), forwardPlan))
		t.Cleanup(func() { stopClosedIssuerProcess(t, process) })
		waitNodeState(t, process, "READY", 10*time.Second)
		live = append(live, process)
	}
	// Each process reached READY and must remain alive after the last Node
	// starts. This does not prove continuous duty readiness or an Endpoint exchange.
	for _, process := range live {
		select {
		case <-process.done:
			t.Fatalf("closed topology Node exited after readiness: %v", process.terminalErr())
		default:
		}
	}
	if nodeCount != 3 {
		participant(resolutionRoot, plan)
		return
	}
	exchange(false)
	terminateLiveClosedIssuer(t, first)
	exchange(true)
	restarted := startNodeCommand(t, node, "issuer", "serve", "--config", path)
	t.Cleanup(func() { stopClosedIssuerProcess(t, restarted) })
	again := waitNodeState(t, restarted, "READY", 10*time.Second)
	if again.Epoch != ready.Epoch || again.AssignmentDigest != ready.AssignmentDigest || again.Assignment != ready.Assignment {
		t.Fatalf("closed issuer restart changed accepted duty: %+v / %+v", ready, again)
	}
	exchange(false)
	terminateLiveClosedIssuer(t, restarted)

}

func closedIssuerListenCredential(t *testing.T, key ed25519.PrivateKey, now time.Time) (string, string) {
	t.Helper()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "closed-issuer.test"},
		DNSNames: []string{"closed-issuer.test"}, NotBefore: now, NotAfter: now.Add(2 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return writePEM(t, "closed-issuer-cert.pem", "CERTIFICATE", raw), writePrivateKey(t, "closed-issuer-key.pem", key)
}

func stopClosedIssuerProcess(t *testing.T, process *nodeProcess) {
	t.Helper()
	stopProcess(process)
	select {
	case <-process.done:
	case <-time.After(5 * time.Second):
		t.Error("closed issuer process did not exit after termination")
	}
}

// A buffered READY event from an already-dead process is not readiness.
func terminateLiveClosedIssuer(t *testing.T, process *nodeProcess) {
	t.Helper()
	select {
	case <-process.done:
		t.Fatalf("issuer exited without test termination: %v", process.terminalErr())
	case <-time.After(time.Second):
	}
	if err := process.command.Process.Kill(); err != nil {
		t.Fatalf("test could not terminate the live issuer: %v", err)
	}
	select {
	case <-process.done:
		if process.terminalErr() == nil {
			t.Fatal("issuer exited successfully despite forced termination")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("issuer did not join after test termination")
	}
}
