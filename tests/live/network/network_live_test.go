//go:build live

package network_test

import (
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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/tests/fixtures/networkfixture"
)

func TestContainersCarryOneAuthenticatedRoute(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	root := repositoryRoot(t)
	fixture := newLiveFixture(t)
	project := fmt.Sprintf("ardents-live-%d", time.Now().UnixNano())
	image := project + ":test"
	environment := append(os.Environ(),
		"ARDENTS_LIVE_IMAGE="+image,
		"ARDENTS_LIVE_ROOT="+filepath.ToSlash(fixture.root),
	)
	compose := func(ctx context.Context, arguments ...string) ([]byte, error) {
		base := []string{"compose", "-p", project, "-f", filepath.Join(root, "tests", "live", "network.compose.yaml")}
		command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
		command.Dir, command.Env = root, environment
		return command.CombinedOutput()
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, _ = compose(ctx, "down", "--volumes", "--remove-orphans", "--rmi", "local", "--timeout", "0")
	}
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if output, err := compose(ctx, "build", "publisher"); err != nil {
		t.Fatalf("build live image: %v\n%s", err, output)
	}
	passive := []string{"publisher", "responder", "rendezvous", "introduction", "initiator"}
	if output, err := compose(ctx, append([]string{"up", "-d", "--no-build"}, passive...)...); err != nil {
		t.Fatalf("start live Route: %v\n%s", err, output)
	}
	for _, service := range passive {
		waitForKind(t, ctx, compose, service, "ready")
	}
	clientOutput, err := compose(ctx, "run", "--rm", "--no-deps", "client")
	if err != nil {
		t.Fatalf("run live Client: %v\n%s", err, clientOutput)
	}
	client := decodeEvidence(t, clientOutput, "complete")
	if client.Role != "client" || client.CanaryLength != 32 || len(client.Canary) != 32 || len(client.Positions) != 4 {
		t.Fatalf("live Client result is incomplete: %+v", client)
	}
	for _, service := range passive {
		evidence := waitForKind(t, ctx, compose, service, "complete")
		if evidence.Role != service || evidence.NetworkID != fixture.network || evidence.EpochDigest != fixture.epochDigest {
			t.Fatalf("%s returned the wrong identity or state: %+v", service, evidence)
		}
		if service == "publisher" {
			if evidence.CanaryLength != 32 || evidence.CanaryDigest != client.CanaryDigest {
				t.Fatalf("Publisher did not receive the Client canary: %+v", evidence)
			}
		} else if evidence.OpaqueBytes == 0 {
			t.Fatalf("%s did not relay opaque Route bytes", service)
		}
	}
	cleanup()
	if output, err := compose(ctx, "ps", "-aq"); err != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatalf("live containers remain after cleanup: err=%v containers=%s", err, output)
	}
}

type composeCall func(context.Context, ...string) ([]byte, error)

func waitForKind(t *testing.T, ctx context.Context, compose composeCall, service, kind string) route.Evidence {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", service)
		if err == nil {
			if value, ok := findEvidence(output, kind); ok {
				return value
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s %s: %v\n%s", service, kind, ctx.Err(), output)
		case <-ticker.C:
		}
	}
}

func decodeEvidence(t *testing.T, output []byte, kind string) route.Evidence {
	t.Helper()
	if value, ok := findEvidence(output, kind); ok {
		return value
	}
	t.Fatalf("missing %s evidence in output:\n%s", kind, output)
	return route.Evidence{}
}

func findEvidence(output []byte, kind string) (route.Evidence, bool) {
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var value route.Evidence
		if json.Unmarshal(bytes.TrimSpace(line), &value) == nil && value.Kind == kind {
			return value, true
		}
	}
	return route.Evidence{}, false
}

type liveIdentity struct {
	private ed25519.PrivateKey
	public  [32]byte
	cert    string
	key     string
}

type liveFixture struct {
	root, stateRoot                string
	network, epochDigest, manifest [32]byte
	now                            time.Time
	authority                      ed25519.PrivateKey
	identities                     []liveIdentity
	addresses                      []string
	plan                           route.Plan
	publisherID                    [32]byte
}

func newLiveFixture(t *testing.T) liveFixture {
	t.Helper()
	root := t.TempDir()
	value := liveFixture{root: root, stateRoot: filepath.Join(root, "state"),
		network: sha256.Sum256([]byte("live-network")), now: time.Now().UTC().Truncate(time.Second),
		authority:   ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa1}, ed25519.SeedSize)),
		addresses:   []string{"172.31.20.11:4601", "172.31.20.12:4602", "172.31.20.13:4603", "172.31.20.14:4604", "172.31.20.16:4605"},
		publisherID: sha256.Sum256([]byte("live-publisher")), manifest: sha256.Sum256([]byte("live-route-manifest"))}
	secrets := filepath.Join(root, "secrets")
	plans := filepath.Join(root, "plans")
	for _, directory := range []string{secrets, plans} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for index := range 6 {
		value.identities = append(value.identities, writeLiveIdentity(t, secrets, byte(index+1), value.now))
	}
	domains := []string{"initiator", "introduction", "rendezvous", "responder"}
	seed := sha256.Sum256([]byte("live-route-assignment"))
	inputs, accepted := make([][]byte, 0, 4), make([]networkfixture.Record, 0, 4)
	for index, domain := range domains {
		family := liveFamily(t, value.network, seed, domain, domains)
		nodeID := sha256.Sum256([]byte("live-node-" + domain))
		record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: value.network, NodeID: nodeID,
			Generation: 1, ValidFrom: value.now.Add(-time.Minute), ValidUntil: value.now.Add(time.Hour),
			Family: family, Endpoint: value.addresses[index], Capability: 2, Capacity: 1,
			PrivateKey: value.identities[index].private})
		if err != nil {
			t.Fatal(err)
		}
		inputs, accepted = append(inputs, record.Raw), append(accepted, record)
	}
	epoch, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{NetworkID: value.network, Number: 1,
		ValidFrom: value.now.Add(-time.Minute), ValidUntil: value.now.Add(time.Hour), Inputs: inputs,
		Accepted: accepted, AssignmentSeed: seed, Profile: "h3-route-tracer-v1", Domains: domains,
		Authorities: []ed25519.PrivateKey{value.authority}})
	if err != nil {
		t.Fatal(err)
	}
	value.epochDigest = epoch.Digest
	public := value.authority.Public().(ed25519.PublicKey)
	opened, err := state.Open(state.Config{Root: value.stateRoot, NetworkID: value.network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		AcceptedProfile: "h3-route-tracer-v1", Now: value.now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.Accept(context.Background(), epoch.Raw, epoch.Inputs, epoch.Materials); err != nil {
		t.Fatal(err)
	}
	snapshot, err := opened.Current()
	if err != nil {
		t.Fatal(err)
	}
	value.plan, err = route.Select(snapshot, route.Selection{Seed: sha256.Sum256([]byte("live-client-selection")), At: value.now})
	if closeErr := opened.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	value.writePlans(t, plans, public)
	return value
}

func (value liveFixture) writePlans(t *testing.T, plans string, authority ed25519.PublicKey) {
	t.Helper()
	writeLivePlan(t, plans, "publisher", map[string]any{
		"Role": "publisher", "ManifestDigest": liveHex(value.manifest), "NetworkID": liveHex(value.network),
		"EpochDigest": liveHex(value.epochDigest), "NodeID": liveHex(value.publisherID), "Listen": value.addresses[4],
		"Certificate": value.identities[4].cert, "Key": value.identities[4].key,
		"UpstreamPin": liveHex(value.identities[3].public), "ServiceCertificate": value.identities[4].cert,
		"ServiceKey": value.identities[4].key, "Deadline": "10s",
	})
	for index, position := range value.plan.Positions {
		upstream := value.identities[5].public
		if index > 0 {
			upstream = value.identities[index-1].public
		}
		nextID, nextAddress, nextPin := value.publisherID, value.addresses[4], value.identities[4].public
		if index < 3 {
			nextID, nextAddress, nextPin = value.plan.Positions[index+1].NodeID, value.addresses[index+1], value.identities[index+1].public
		}
		writeLivePlan(t, plans, position.Role, map[string]any{
			"Role": position.Role, "ManifestDigest": liveHex(value.manifest), "NetworkID": liveHex(value.network),
			"EpochDigest": liveHex(value.epochDigest), "NodeID": liveHex(position.NodeID), "Listen": value.addresses[index],
			"Certificate": value.identities[index].cert, "Key": value.identities[index].key,
			"UpstreamPin": liveHex(upstream), "NextNodeID": liveHex(nextID), "Next": nextAddress,
			"NextPin": liveHex(nextPin), "Deadline": "10s",
		})
	}
	writeLivePlan(t, plans, "client", map[string]any{
		"Role": "client", "ManifestDigest": liveHex(value.manifest), "StateRoot": "/run/ardents/state",
		"NetworkID": liveHex(value.network), "Authorities": []string{hex.EncodeToString(authority)}, "Threshold": 1,
		"At": value.now.Format(time.RFC3339), "Seed": liveHex(value.plan.Seed),
		"Certificate": value.identities[5].cert, "Key": value.identities[5].key,
		"PublisherPin": liveHex(value.identities[4].public), "Deadline": "10s",
	})
}

func writeLiveIdentity(t *testing.T, root string, marker byte, now time.Time) liveIdentity {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: "route.live"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	certificate, err := x509.CreateCertificate(nil, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	certName, keyName := fmt.Sprintf("identity-%d-cert.pem", marker), fmt.Sprintf("identity-%d-key.pem", marker)
	writeLiveFile(t, filepath.Join(root, certName), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}))
	writeLiveFile(t, filepath.Join(root, keyName), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}))
	var fixed [32]byte
	copy(fixed[:], public)
	return liveIdentity{private: private, public: fixed, cert: "/run/ardents/secrets/" + certName, key: "/run/ardents/secrets/" + keyName}
}

func liveFamily(t *testing.T, network, seed [32]byte, wanted string, domains []string) string {
	t.Helper()
	for index := range 10_000 {
		family := fmt.Sprintf("%s-family-%d", wanted, index)
		selected, err := assignment.Select(network, 1, seed, family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if selected == wanted {
			return family
		}
	}
	t.Fatal("could not derive a family for the required Route domain")
	return ""
}

func writeLivePlan(t *testing.T, root, name string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeLiveFile(t, filepath.Join(root, name+".json"), raw)
}

func writeLiveFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func liveHex(value [32]byte) string { return hex.EncodeToString(value[:]) }

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
}
