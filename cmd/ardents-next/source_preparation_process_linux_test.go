//go:build linux && node_process_native

package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func writeSourceFixtureKeys(t *testing.T, p map[string]any, credentials []tls.Certificate) {
	t.Helper()
	members := p["sources"].([]map[string]any)
	for i, path := range []string{members[0]["private_key"].(string), members[1]["private_key"].(string), p["client_private_key"].(string)} {
		der, err := x509.MarshalPKCS8PrivateKey(credentials[i].PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
		raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		clear(raw)
		clear(der)
	}
}

func TestNativeSourcePreparedCohort(t *testing.T) {
	binary := nodeNativeCommand(t)
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	ancestors, err := openPreparationParent(parent)
	if err != nil {
		t.Fatal("invalid environment: original public workspace ancestors unavailable", err)
	}
	if err = errors.Join(signingAncestors(ancestors), ancestors.close()); err != nil {
		t.Fatal("invalid environment: root-controlled direct ancestors required", err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: original workspace is not native ext4", err)
	}
	f := newNetworkAdmissionFixture(t)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := joinProcessClock(t)
	defer closeClock()
	var networks [2]networkAuthorityPlan
	for i := range networks {
		networks[i] = seedNativeNodeState(t, f, bundle, clock)
	}
	authority := networks[0]
	headless := installedHeadless{Schema: "ardents-headless-runtime-v2", NetworkStateRoot: filepath.Join(parent, "client-state"), LocalRoleStateRoot: filepath.Join(parent, "client-roles"), TimeConfidenceFile: clock, NetworkID: hex.EncodeToString(authority.NetworkID[:]), NetworkAuthorities: []string{hex.EncodeToString(authority.Authorities[0][:])}, NetworkThreshold: 1, ClosedProfileAuthority: hex.EncodeToString(authority.ProfileAuthority[:])}
	p, credentials := sourcePreparationFixture(t, parent, headless)
	writeSourceFixtureKeys(t, p, credentials)
	members := p["sources"].([]map[string]any)
	var held [2]net.Listener
	for i := range held {
		held[i], err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = held[i].Close() })
		members[i]["address"] = held[i].Addr().String()
		members[i]["state_root"] = networks[i].Root
	}
	plan, output := filepath.Join(parent, "preparation.json"), filepath.Join(parent, "prepared")
	if err = os.WriteFile(plan, sourcePreparationJSON(t, p), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), binary, "source", "prepare-trust", plan, output)
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatal("actual preparation", err, string(raw))
	}
	var processes [2]*nativeForwardingProcess
	for i := range processes {
		if err = held[i].Close(); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			// A real second-process partial startup must fail and join while the
			// first Source retains its own accepting lifetime. This is a causal
			// refusal followed by separate new configuration, never a retry verdict.
			keyPath := members[i]["private_key"].(string)
			keyBytes, e := os.ReadFile(keyPath)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(keyPath, []byte("invalid external fixture credential"), 0600); e != nil {
				t.Fatal(e)
			}
			failed := launchNativeSource(t, binary, filepath.Join(output, "source-2-serve.json"))
			var refused map[string]string
			if e = json.NewDecoder(failed.stdout).Decode(&refused); e == nil {
				t.Fatal("partial startup announced readiness")
			}
			e = failed.command.Wait()
			failed.joined = true
			_ = failed.stdout.Close()
			var exited *exec.ExitError
			if !errors.As(e, &exited) || exited.ExitCode() != 1 {
				t.Fatal("partial startup physical outcome", e, failed.diagnostic.String())
			}
			if e = os.WriteFile(keyPath, keyBytes, 0600); e != nil {
				t.Fatal(e)
			}
			clear(keyBytes)
		}
		processes[i] = launchNativeSource(t, binary, filepath.Join(output, []string{"source-1-serve.json", "source-2-serve.json"}[i]))
		var ready map[string]string
		if err = json.NewDecoder(processes[i].stdout).Decode(&ready); err != nil || ready["phase"] != "listening" {
			t.Fatal("real Source startup", i, ready, err)
		}
	}
	read := func(name string) []byte {
		t.Helper()
		raw, e := os.ReadFile(filepath.Join(output, name))
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	client, declared, err := installedDeclarations(read("client-headless.json"), read("client-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := installedNetworkConfig(client, declared, func() error { return nil })
	if err != nil {
		t.Fatal("real installed-network projection", err)
	}
	// This test composition opens genuine State; it grants no Installation or
	// installed startup permission. Every TLS handshake uses State's clock.
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := owner.Close(); e != nil {
			t.Error(e)
		}
	})
	snapshot, err := owner.Refresh(t.Context())
	if err != nil || snapshot.Digest != bundle.Epoch.Digest || snapshot.ObservedDigests[0] != bundle.Epoch.Digest || snapshot.ObservedDigests[1] != bundle.Epoch.Digest {
		t.Fatal("both actual Sources acquired", snapshot, err)
	}
	if _, e := owner.CurrentRuntime(); e == nil {
		t.Fatal("TLS supplied missing signed profile authority")
	}
	if _, e := owner.AcceptClosedProfile(bundle.Profile); e != nil {
		t.Fatal(e)
	}
	if _, e := owner.CurrentRuntime(); e != nil {
		t.Fatal("genuine signed State/profile", e)
	}
	altered := bytes.Clone(bundle.Epoch.Raw)
	altered[len(altered)-1] ^= 1
	if _, e := owner.Accept(t.Context(), altered, bundle.Epoch.Inputs, bundle.Epoch.Materials); e == nil {
		t.Fatal("TLS replaced signed State verification")
	}
	if _, e := owner.CurrentRuntime(); e != nil {
		t.Fatal("invalid signed input poisoned retained authority", e)
	}
	// Direct request coverage uses the same supplied acquisition credentials;
	// State acceptance above independently verifies the AREP-v3 contents.
	transport := config.Source
	transport.VerificationClock = config.Clock
	planOwner, _, err := source.New(transport, config.Authorities)
	if err != nil {
		t.Fatal(err)
	}
	for i := range processes {
		for _, operation := range []string{"latest", "by-digest"} {
			request := source.Message{Operation: operation, NetworkDigest: source.NetworkDigest(config.NetworkID)}
			if operation == "by-digest" {
				request.ObjectDigest = bundle.Epoch.Digest
			}
			response, e := planOwner.Fetch(t.Context(), i, request)
			decoded, decodeErr := source.DecodeBundle(response.Payload)
			if e != nil || decodeErr != nil || response.ObjectDigest != bundle.Epoch.Digest || !bytes.Equal(decoded.Epoch, bundle.Epoch.Raw) {
				t.Fatal("actual Source bytes", i, operation, e, decodeErr)
			}
		}
	}
	// New observations of wrong CA/name/pin and an unauthorized client cannot
	// acquire accepted bytes. Healthy exchanges bracket these causal controls.
	for _, fault := range []string{"ca", "name", "pin", "client-pin", "client-ca", "expired"} {
		t.Run(fault, func(t *testing.T) {
			bad := transport
			switch fault {
			case "ca":
				_, _, _, bad.Sources[0].RootPEM, _, _ = sourceNativeTLS(t)
			case "name":
				bad.Sources[0].ServerName = "wrong.test"
			case "pin":
				bad.Sources[0].LeafKeyDigest[0] ^= 1
			case "client-pin":
				bad.ClientCertificate = credentials[3]
			case "client-ca":
				_, bad.ClientCertificate, _, _, _, _ = sourceNativeTLS(t)
			case "expired":
				bad.VerificationClock = func() time.Time { return time.Now().Add(2 * time.Hour) }
			}
			attempt, _, e := source.New(bad, config.Authorities)
			if e == nil {
				response, e := attempt.Fetch(t.Context(), 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(config.NetworkID)})
				if e == nil && response.Status == "ok" {
					t.Fatal("handshake refusal missing", fault)
				}
			}
		})
	}
	// Alter a copied public PEM after startup: original live servers retain
	// their credentials, while a fresh runtime projection refuses the new bytes.
	certificatePath := filepath.Join(output, "source-1-certificate.pem")
	original := read("source-1-certificate.pem")
	if err := os.WriteFile(certificatePath, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if response, e := planOwner.Fetch(t.Context(), 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(config.NetworkID)}); e != nil || response.Status != "ok" {
		t.Fatal("running credential reloaded", e)
	}
	servingRaw := read("source-1-serve.json")
	serving, err := decodeSourceServePlan(servingRaw)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := networkStateConfig(serving.Network)
	if err != nil {
		t.Fatal(err)
	}
	if sourceServingInputs(&fresh, serving, func() error { return nil }) == nil {
		t.Fatal("new configuration accepted altered credential")
	}
	if err := os.WriteFile(certificatePath, original, 0600); err != nil {
		t.Fatal(err)
	}
	if e := owner.Close(); e != nil {
		t.Fatal(e)
	}
	for i, p := range processes {
		if e := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGTERM", p.unit).Run(); e != nil {
			t.Fatal(e)
		}
		err := p.command.Wait()
		p.joined = true
		// Cmd.Wait owns closing its StdoutPipe after original process join.
		// A second Close observes that completed ownership, not a runtime error.
		if e := p.stdout.Close(); e != nil && !errors.Is(e, os.ErrClosed) {
			t.Fatal(e)
		}
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 130 {
			t.Fatal("original physical Wait", i, err, p.diagnostic.String())
		}
		reopened, e := networkStateConfig(&networks[i])
		if e != nil {
			t.Fatal(e)
		}
		next, e := state.Open(reopened)
		if e != nil {
			t.Fatal("original State lease retained after join", e)
		}
		if e = next.Close(); e != nil {
			t.Fatal(e)
		}
	}
	// The exact generated local plans remain unchanged by runtime consumers.
	for _, name := range []string{"source-1-serve.json", "source-2-serve.json", "client-source.json", "client-headless.json"} {
		t.Logf("consumed public plan %s sha256=%x", name, sha256.Sum256(read(name)))
	}
}
