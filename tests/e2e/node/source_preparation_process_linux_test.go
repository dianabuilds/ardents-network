//go:build linux && node_process_native

package node_test

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
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/endpoint"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"

	sourceowner "github.com/dianabuilds/ardents-network/internal/domain/network/source/serving"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
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
	binary := processfixture.PinnedCommand(t)
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	ancestors, err := artifacts.OpenDirectoryChain(t.Context(), parent, nil)
	if err != nil {
		t.Fatal("invalid environment: original public workspace ancestors unavailable", err)
	}
	if err = errors.Join(ancestors.CheckRootAccess(), ancestors.Close()); err != nil {
		t.Fatal("invalid environment: root-controlled direct ancestors required", err)
	}
	mountType, err := exec.Command("findmnt", "--kernel", "--noheadings", "--output", "FSTYPE", "--target", parent).Output()
	if err != nil || strings.TrimSpace(string(mountType)) != "ext4" {
		t.Fatal("invalid environment: original workspace is not native ext4", err)
	}
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := networkinputs.StartObservedClock(t)
	defer closeClock()
	var networks [2]state.AuthorityDeclaration
	for i := range networks {
		networks[i] = networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
	}
	authority := networks[0]
	headless := endpoint.HeadlessPlan{Schema: "ardents-headless-runtime-v2", NetworkStateRoot: filepath.Join(parent, "client-state"), LocalRoleStateRoot: filepath.Join(parent, "client-roles"), TimeConfidenceFile: clock, NetworkID: hex.EncodeToString(authority.NetworkID[:]), NetworkAuthorities: []string{hex.EncodeToString(authority.Authorities[0][:])}, NetworkThreshold: 1, ClosedProfileAuthority: hex.EncodeToString(authority.ProfileAuthority[:])}
	p, credentials := sourcefixture.TrustPreparationFixture(t, parent, headless)
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
	rawPlan, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(plan, rawPlan, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), binary, "source", "prepare-trust", plan, output)
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatal("actual preparation", err, string(raw))
	}
	var processes [2]*processfixture.NativeProcess
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
			if e = json.NewDecoder(failed.Stdout).Decode(&refused); e == nil {
				t.Fatal("partial startup announced readiness")
			}
			e = failed.Command.Wait()
			failed.Joined = true
			_ = failed.Stdout.Close()
			var exited *exec.ExitError
			if !errors.As(e, &exited) || exited.ExitCode() != 1 {
				t.Fatal("partial startup physical outcome", e, failed.Diagnostic.String())
			}
			if e = os.WriteFile(keyPath, keyBytes, 0600); e != nil {
				t.Fatal(e)
			}
			clear(keyBytes)
		}
		processes[i] = launchNativeSource(t, binary, filepath.Join(output, []string{"source-1-serve.json", "source-2-serve.json"}[i]))
		var ready map[string]string
		if err = json.NewDecoder(processes[i].Stdout).Decode(&ready); err != nil || ready["phase"] != "listening" {
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
	client, declared, err := endpoint.Declarations(read("client-headless.json"), read("client-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := endpoint.NetworkConfig(client, declared, func() error { return nil })
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
	if _, e := owner.AcceptEpoch(t.Context(), altered, bundle.Epoch.Inputs, bundle.Epoch.Materials); e == nil {
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
				_, _, _, bad.Sources[0].RootPEM, _, _ = sourcefixture.NativeTLS(t)
			case "name":
				bad.Sources[0].ServerName = "wrong.test"
			case "pin":
				bad.Sources[0].LeafKeyDigest[0] ^= 1
			case "client-pin":
				bad.ClientCertificate = credentials[3]
			case "client-ca":
				_, bad.ClientCertificate, _, _, _, _ = sourcefixture.NativeTLS(t)
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
	// Changing the original PEM retires its lifetime; it never reloads or
	// replaces the original authority. A separate new invocation is readmitted.
	certificatePath := filepath.Join(output, "source-1-certificate.pem")
	original := read("source-1-certificate.pem")
	if err := os.WriteFile(certificatePath, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	err = processes[0].Command.Wait()
	processes[0].Joined = true
	var retired *exec.ExitError
	if !errors.As(err, &retired) || retired.ExitCode() != 1 {
		t.Fatal("original changed-input retirement", err, processes[0].Diagnostic.String())
	}
	if response, e := planOwner.Fetch(t.Context(), 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(config.NetworkID)}); e == nil && response.Status == "ok" {
		t.Fatal("changed original input retained an accepting path")
	}
	servingRaw := read("source-1-serve.json")
	serving, err := servingdeclaration.DecodeServingDeclaration(servingRaw)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := (*state.AuthorityDeclaration)(serving.Network).Config()
	if err != nil {
		t.Fatal(err)
	}
	inputs, _, err := sourceowner.AcquireInputs(t.Context(), filepath.Join(output, "source-1-serve.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sourceowner.ConfigureInputs(&fresh, serving, inputs, inputs.Check) == nil {
		t.Fatal("new configuration accepted altered credential")
	}
	if err := inputs.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certificatePath, original, 0600); err != nil {
		t.Fatal(err)
	}
	if response, e := planOwner.Fetch(t.Context(), 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(config.NetworkID)}); e == nil && response.Status == "ok" {
		t.Fatal("restored path revived the retired lifetime")
	}
	processes[0] = launchNativeSource(t, binary, filepath.Join(output, "source-1-serve.json"))
	// A failed original completion retains the serving producer. Restored input
	// bytes cannot revive the retired lifetime, and a separate new invocation
	// refuses before listening; only bounded recovery removes that producer.
	refusal := processes[0].Command.Wait()
	processes[0].Joined = true
	if e := processes[0].Stdout.Close(); e != nil && !errors.Is(e, os.ErrClosed) {
		t.Fatal(e)
	}
	var refusedExit *exec.ExitError
	if !errors.As(refusal, &refusedExit) || refusedExit.ExitCode() != 1 {
		t.Fatal("separate new Source adopted a retained producer", refusal, processes[0].Diagnostic.String())
	}
	var refusalTerminal struct {
		Phase, Outcome string
		First          string `json:"first_owner"`
	}
	if raw := bytes.TrimSpace([]byte(processes[0].Diagnostic.String())); json.Unmarshal(raw, &refusalTerminal) != nil || refusalTerminal.Phase != "refused" || refusalTerminal.First != "state" {
		t.Fatal("retained producer refusal cause", processes[0].Diagnostic.String())
	}
	if response, e := planOwner.Fetch(t.Context(), 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(config.NetworkID)}); e == nil && response.Status == "ok" {
		t.Fatal("refused invocation left an accepting path")
	}
	if e := owner.Close(); e != nil {
		t.Fatal(e)
	}
	second := processes[1]
	if e := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGTERM", second.Unit).Run(); e != nil {
		t.Fatal(e)
	}
	waitErr := second.Command.Wait()
	second.Joined = true
	// Cmd.Wait owns closing its StdoutPipe after original process join.
	// A second Close observes that completed ownership, not a runtime error.
	if e := second.Stdout.Close(); e != nil && !errors.Is(e, os.ErrClosed) {
		t.Fatal(e)
	}
	var exited *exec.ExitError
	if !errors.As(waitErr, &exited) || exited.ExitCode() != 130 {
		t.Fatal("original physical Wait", waitErr, second.Diagnostic.String())
	}
	for i := range networks {
		reopened, e := networks[i].Config()
		if e != nil {
			t.Fatal(e)
		}
		next, e := state.Open(reopened)
		if e != nil {
			t.Fatal("original State lease retained after join", i, e)
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
