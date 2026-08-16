package state_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type processCertificate struct {
	certificatePath string
	keyPath         string
	rootPath        string
	private         ed25519.PrivateKey
	pin             [32]byte
}

func TestFiniteSourceCommandsAsBlackBoxProcesses(t *testing.T) {
	genesis := writeVerifierFixtureAt(t, time.Now().Unix())
	fixture := writeVerifierSuccessor(t, genesis)
	ardents := buildProductCommand(t, "ardents")
	node := buildProductCommand(t, "ardents-node")
	genesisMaterial := filepath.Join(t.TempDir(), "genesis-material.bin")
	if err := os.WriteFile(genesisMaterial, genesis.materializations[0], 0o600); err != nil {
		t.Fatal(err)
	}
	successorMaterial := filepath.Join(t.TempDir(), "successor-material.bin")
	if err := os.WriteFile(successorMaterial, fixture.materializations[0], 0o600); err != nil {
		t.Fatal(err)
	}
	endpointRoot, firstRoot, secondRoot := t.TempDir(), t.TempDir(), t.TempDir()
	acceptFixtureWithCommand(t, ardents, genesis, genesisMaterial, endpointRoot)
	for _, root := range []string{firstRoot, secondRoot} {
		acceptFixtureWithCommand(t, ardents, genesis, genesisMaterial, root)
		acceptFixtureWithCommand(t, ardents, fixture, successorMaterial, root)
	}

	clientAuthority := processAuthority(t, 0x31, "endpoint-client-root")
	client := processLeaf(t, clientAuthority, 0x32, "endpoint.test", false)
	firstAuthority := processAuthority(t, 0x41, "first-source-root")
	secondAuthority := processAuthority(t, 0x42, "second-source-root")
	firstServer := processLeaf(t, firstAuthority, 0x43, "first-source.test", true)
	secondServer := processLeaf(t, secondAuthority, 0x44, "second-source.test", true)
	firstAddress, secondAddress := freeProcessAddress(t), freeProcessAddress(t)
	firstPlan := writeServerPlan(t, fixture, firstRoot, firstAddress, firstServer, clientAuthority.rootPath, client.pin)
	secondPlan := writeServerPlan(t, fixture, secondRoot, secondAddress, secondServer, clientAuthority.rootPath, client.pin)
	stopFirst := startSourceProcess(t, node, firstPlan)
	defer stopFirst()
	stopSecond := startSourceProcess(t, node, secondPlan)
	defer stopSecond()

	plan := map[string]any{
		"schema": "ardents-h3-source-plan-v1", "network_id": hex.EncodeToString(fixture.networkID[:]),
		"local_role_state_root": endpointRoot + "-local-roles",
		"authority_public":      []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1,
		"clock_observed_at":     time.Now().UTC().Format(time.RFC3339),
		"order_seed":            hexDigest(sha256.Sum256([]byte("black-box-source-order"))),
		"materialization_index": 0, "client_certificate": client.certificatePath, "client_key": client.keyPath,
		"sources": []map[string]any{
			processSourcePlan(firstAddress, "first-source.test", "first", firstAuthority.rootPath, firstServer.pin),
			processSourcePlan(secondAddress, "second-source.test", "second", secondAuthority.rootPath, secondServer.pin),
		},
	}
	planPath := writeProcessJSON(t, "endpoint-source-plan.json", plan)
	command := exec.Command(ardents, "refresh-sources", "--state-root", endpointRoot, "--source-plan", planPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("refresh source processes: %v\n%s", err, output)
	}
	var event struct {
		Kind               string    `json:"kind"`
		Generation         string    `json:"generation"`
		SourceAttempts     uint16    `json:"source_attempts"`
		SourceOutcomes     [4]string `json:"source_outcomes"`
		LatestCompleteness string    `json:"latest_completeness"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &event); err != nil || event.Kind != "source-wave-accepted" ||
		event.Generation != fixture.generation || event.SourceAttempts != 2 ||
		event.SourceOutcomes != [4]string{"valid", "valid", "not-attempted", "not-attempted"} ||
		event.LatestCompleteness != "latest completeness unproven" {
		t.Fatalf("unexpected source refresh event: event=%+v err=%v raw=%s", event, err, output)
	}
	authorityID := sha256.Sum256(fixture.authorityPublic)
	opened, err := state.Open(state.Config{Root: endpointRoot, NetworkID: fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{authorityID: fixture.authorityPublic},
		Threshold:   1, Now: time.Unix(fixture.now, 0)})
	if err != nil {
		t.Fatalf("open refreshed state: %v", err)
	}
	defer opened.Close()
	current, err := opened.Current()
	if err != nil || current.Generation != fixture.generation || current.Epoch != fixture.epoch {
		t.Fatalf("current state mismatch: snapshot=%+v err=%v", current, err)
	}
}

func buildProductCommand(t *testing.T, name string) string {
	t.Helper()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	path := filepath.Join(t.TempDir(), name+suffix)
	command := exec.Command("go", "build", "-o", path, "./cmd/"+name)
	command.Dir = filepath.Join("..", "..", "..")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}

func acceptFixtureWithCommand(t *testing.T, binary string, fixture verifierFixture, material, root string) {
	t.Helper()
	generation := filepath.Join(fixture.root, "generations", fixture.generation)
	arguments := []string{
		"accept-offline", "--state-root", root,
		"--network-id", hex.EncodeToString(fixture.networkID[:]),
		"--authorities", hex.EncodeToString(fixture.authorityPublic), "--threshold", "1",
		"--at", time.Unix(fixture.now, 0).UTC().Format(time.RFC3339),
		"--epoch", filepath.Join(generation, "epoch.bin"), "--inputs", filepath.Join(generation, "inputs"),
		"--materialization", material,
	}
	if output, err := exec.Command(binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("install fixture state: %v\n%s", err, output)
	}
}

func startSourceProcess(t *testing.T, binary, plan string) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, binary, "source", "--config", plan)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
			return
		}
		ready <- ""
	}()
	select {
	case line := <-ready:
		if !strings.Contains(line, `"kind":"source-ready"`) {
			cancel()
			_ = command.Wait()
			t.Fatalf("source did not become ready: line=%q stderr=%s", line, stderr.String())
		}
	case <-time.After(5 * time.Second):
		cancel()
		_ = command.Wait()
		t.Fatal("source readiness timed out")
	}
	return func() { cancel(); _ = command.Wait() }
}

func writeServerPlan(t *testing.T, fixture verifierFixture, root, address string, server processCertificate, clientRoot string, clientPin [32]byte) string {
	t.Helper()
	return writeProcessJSON(t, "source-server-plan.json", map[string]any{
		"schema": "ardents-h3-source-server-v1", "state_root": root,
		"local_role_state_root": root + "-local-roles",
		"network_id":            hex.EncodeToString(fixture.networkID[:]),
		"authority_public":      []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1,
		"at": time.Unix(fixture.now, 0).UTC().Format(time.RFC3339), "listen": address,
		"server_certificate": server.certificatePath, "server_key": server.keyPath,
		"client_root": clientRoot, "client_key_digests": []string{hex.EncodeToString(clientPin[:])},
		"materialization_index": 0,
	})
}

func processSourcePlan(address, serverName, label, root string, pin [32]byte) map[string]any {
	identity := sha256.Sum256([]byte(label + "-source-identity"))
	return map[string]any{
		"address": address, "server_name": serverName, "identity": hex.EncodeToString(identity[:]),
		"family": label + "-source-family", "endpoint_handle": label + "-source-handle",
		"root_ca": root, "leaf_key_digest": hex.EncodeToString(pin[:]),
	}
}

func writeProcessJSON(t *testing.T, name string, value any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func freeProcessAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func processAuthority(t *testing.T, marker byte, name string) processCertificate {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	template := &x509.Certificate{SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Unix(1_600_000_000, 0), NotAfter: time.Unix(2_200_000_000, 0),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(nil, template, template, private.Public(), private)
	if err != nil {
		t.Fatal(err)
	}
	rootPath := writeProcessPEM(t, "root.pem", "CERTIFICATE", raw)
	return processCertificate{rootPath: rootPath, private: private}
}

func processLeaf(t *testing.T, authority processCertificate, marker byte, name string, server bool) processCertificate {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	rootPEM, err := os.ReadFile(authority.rootPath)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(rootPEM)
	parent, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	usage := x509.ExtKeyUsageClientAuth
	if server {
		usage = x509.ExtKeyUsageServerAuth
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: time.Unix(1_600_000_000, 0), NotAfter: time.Unix(2_200_000_000, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	raw, err := x509.CreateCertificate(nil, template, parent, private.Public(), authority.private)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte("ardents-h3-source-transport-key-v1\x00")
	return processCertificate{certificatePath: writeProcessPEM(t, "leaf.pem", "CERTIFICATE", raw),
		keyPath: writeProcessPEM(t, "leaf-key.pem", "PRIVATE KEY", key), private: private,
		pin: sha256.Sum256(append(prefix, private.Public().(ed25519.PublicKey)...))}
}

func writeProcessPEM(t *testing.T, name, kind string, raw []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func hexDigest(value [32]byte) string { return fmt.Sprintf("%x", value) }
