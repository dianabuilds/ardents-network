//go:build linux

package node_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestNetworkCommandsAndAdmissionUseAuthenticatedRoots(t *testing.T) {
	for _, test := range []struct {
		name  string
		class admission.Class
		bytes uint64
	}{
		{"Forward", admission.ForwardClass, 33554432},
		{"Registration", admission.RegistrationClass, 1048576},
	} {
		t.Run(test.name, func(t *testing.T) {
			testNetworkCommandsAdmissionClass(t, test.class, test.bytes)
		})
	}
}

func testNetworkCommandsAdmissionClass(t *testing.T, class admission.Class, selectedBytes uint64) {
	t.Helper()
	binary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, plan, inventory := processfixture.PrepareNetworkGenesis(t, t.TempDir())
	if class == admission.RegistrationClass {
		spec.Nodes[1].RoleDomain, spec.Nodes[1].Subrole = 4, 3
	}
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	profileDigest := sha256.Sum256(bundle.Profile)
	processfixture.PrepareIssuer(t, &plan, inventory, spec, bundle.Nodes[0], profileDigest)
	member := bundle.Nodes[1]
	binding := receiving.Receiver{NetworkID: spec.NetworkID, StateGeneration: bundle.Epoch.Digest, StateDigest: bundle.Epoch.Digest,
		ProfileDigest: profileDigest, NodeID: member.NodeID, DutyGeneration: member.Generation}
	clockFile, closeClock := processfixture.StartObservedClock(t)
	t.Cleanup(closeClock)
	command := func(operation string, config any, accepted bool) struct {
		Outcome string                `json:"outcome"`
		Profile *network.ProfileFacts `json:"profile"`
	} {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), binary, "network", operation, "--config", writeNetworkInput(t, config))
		var output, log bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &log
		err := cmd.Run()
		if (err == nil) != accepted {
			t.Fatalf("Network %s: accepted=%t error=%v output=%s log=%s", operation, accepted, err, &output, &log)
		}
		var reply struct {
			Outcome string                `json:"outcome"`
			Profile *network.ProfileFacts `json:"profile"`
		}
		if err := json.Unmarshal(output.Bytes(), &reply); err != nil {
			t.Fatal(err, &output)
		}
		return reply
	}
	seed := func() state.AuthorityDeclaration {
		t.Helper()
		var key [32]byte
		copy(key[:], spec.Authority[32:])
		p := state.AuthorityDeclaration{Root: filepath.Join(t.TempDir(), "network"), NetworkID: spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clockFile}
		bad := bytes.Clone(bundle.Epoch.Raw)
		bad[len(bad)-1] ^= 1
		if reply := command("accept-epoch", map[string]any{"network": p, "epoch": bad, "inputs": bundle.Epoch.Inputs, "materials": bundle.Epoch.Materials}, false); reply.Outcome != "unavailable" {
			t.Fatal(reply)
		}
		if reply := command("accept-epoch", map[string]any{"network": p, "epoch": bundle.Epoch.Raw, "inputs": bundle.Epoch.Inputs, "materials": bundle.Epoch.Materials}, true); reply.Outcome != "accepted-epoch" {
			t.Fatal(reply)
		}
		if reply := command("inspect", map[string]any{"network": p}, false); reply.Profile != nil {
			t.Fatal("Epoch intake alone published profile authority")
		}
		if reply := command("accept-profile", map[string]any{"network": p, "profile": bundle.Profile}, true); reply.Outcome != "accepted-profile" {
			t.Fatal(reply)
		}
		if reply := command("inspect", map[string]any{"network": p}, true); reply.Profile == nil || reply.Profile.Digest != profileDigest || reply.Profile.Generation != bundle.Epoch.Digest {
			t.Fatal("new executable reconstructed different signed authority", reply)
		}
		return p
	}
	// Each simultaneously opened participant owns one installation-local State
	// lease. They share authenticated public bytes, not a writable private root.
	holderNetwork, issuerNetwork, receiverNetwork := seed(), seed(), seed()
	role := admission.AllocationUser
	if class == admission.RegistrationClass {
		role = admission.AllocationPublisher
	}
	holderConfig := map[string]any{"root": t.TempDir(), "profile": "", "network": holderNetwork, "role": role}
	holder, closeHolder := nodefixture.OpenAdmissionConsole(t, binary, "holder", writeNetworkInput(t, holderConfig))
	var maxima [3]uint32
	maxima[class-1] = 1
	request := holder(map[string]any{"operation": "request", "maxima": maxima})
	if request.Outcome != "completed" {
		t.Fatal(request)
	}
	profile := admission.AuthorityFacts{NetworkID: spec.NetworkID, IssuanceAuthorityKey: spec.IssuanceAuthority}
	signed := processfixture.FixturePermission(t, request.Request, profile, spec.Authority, time.Now())
	if reply := holder(map[string]any{"operation": "import", "digest": request.Digest, "payload": signed}); reply.Outcome != "completed" {
		t.Fatal(reply)
	}
	challenge := token.ClosedTokenContext{NetworkID: spec.NetworkID, ProfileDigest: profileDigest, IssuerNodeID: bundle.Nodes[0].NodeID, ReceiverNodeID: binding.NodeID, ReceiverDutyGeneration: binding.DutyGeneration, Class: uint8(class), WindowStart: time.Now().UTC().Truncate(time.Hour)}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge}, Selection: stock.ExchangeBinding{ID: [32]byte{21}, ProfileDigest: profileDigest}, Bootstrap: true, Deadline: spec.NotAfter}
	badIntent := intent
	badChallenge := challenge
	badChallenge.ReceiverNodeID = [32]byte{99}
	badIntent.Challenges = []token.ClosedTokenContext{badChallenge}
	if reply := holder(map[string]any{"operation": "begin", "intent": badIntent}); reply.Outcome != "refused" {
		t.Fatal("unknown signed recipient consumed issuance allowance", reply)
	}
	batch := holder(map[string]any{"operation": "begin", "intent": intent})
	if batch.Outcome != "completed" {
		t.Fatal(batch)
	}
	issued := invokeNetworkAdmission(t, binary, "issue-current", map[string]any{"plan": plan, "profile": "", "network": issuerNetwork, "batch": batch.Request, "kind": quota.Bootstrap})
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued)
	}
	if reply := holder(map[string]any{"operation": "complete", "payload": issued.Response}); reply.Outcome != "completed" {
		t.Fatal(reply)
	}
	presentation := stock.Presentation{NetworkID: spec.NetworkID, StateGeneration: bundle.Epoch.Digest, StateDigest: bundle.Epoch.Digest, ProfileDigest: profileDigest, RecipientNodeID: binding.NodeID, RecipientDutyGeneration: binding.DutyGeneration, ChannelNonce: [32]byte{22}, Deadline: spec.NotAfter}
	wrong := presentation
	wrong.RecipientDutyGeneration++
	if reply := holder(map[string]any{"operation": "take", "presentation": wrong, "class": uint8(class)}); reply.Outcome != "refused" {
		t.Fatal("unaccepted recipient consumed presentation", reply)
	}
	taken := holder(map[string]any{"operation": "take", "presentation": presentation, "class": uint8(class)})
	if taken.Outcome != "completed" {
		t.Fatal(taken)
	}
	closeHolder()
	receiverConfig := map[string]any{"root": t.TempDir(), "profile": "", "network": receiverNetwork, "receiver": binding, "not_after": spec.NotAfter}
	receiver, closeReceiver := nodefixture.OpenAdmissionConsole(t, binary, "receiver", writeNetworkInput(t, receiverConfig))
	deadline := time.Now().Add(time.Second)
	accept := map[string]any{"operation": "accept", "token": taken.Token, "class": class, "deadline": deadline}
	if reply := receiver(accept); reply.Outcome != "completed" || reply.Bytes != selectedBytes || !reply.Deadline.Equal(deadline) {
		t.Fatal(reply)
	}
	closeReceiver()
	reopened, closeReopened := nodefixture.OpenAdmissionConsole(t, binary, "receiver", writeNetworkInput(t, receiverConfig))
	if reply := reopened(accept); reply.Outcome != "refused" {
		t.Fatal("reopened new executable revived durable spend", reply)
	}
	closeReopened()
}

func writeNetworkInput(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "network-input.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func invokeNetworkAdmission(t *testing.T, binary, operation string, config any) nodefixture.AdmissionReply {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, "admission", operation, "--config", writeNetworkInput(t, config))
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(operation, err)
	}
	var reply nodefixture.AdmissionReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		t.Fatal(err)
	}
	return reply
}
