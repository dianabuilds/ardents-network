//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
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
	_ = compiledCommand(t)
	f := newNetworkAdmissionFixture(t, func(f *networkAdmissionFixture) {
		if class == admission.RegistrationClass {
			f.spec.Nodes[1].RoleDomain, f.spec.Nodes[1].Subrole = 4, 3
		}
	})
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clockFile := filepath.Join(t.TempDir(), "clock-observation")
	if err := os.WriteFile(clockFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	// Local clock evidence for this process test only, not external time trust.
	stop, joined := make(chan struct{}), make(chan error, 1)
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				joined <- nil
				return
			case now := <-ticker.C:
				if err := os.Chtimes(clockFile, now, now); err != nil {
					joined <- err
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		if err := <-joined; err != nil {
			t.Error(err)
		}
	})
	command := func(operation string, config any, accepted bool) struct {
		Outcome string                `json:"outcome"`
		Profile *network.ProfileFacts `json:"profile"`
	} {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), "network", operation, "--config", hostingConfig(t, config))
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
	seed := func() networkAuthorityPlan {
		t.Helper()
		var key [32]byte
		copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
		p := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clockFile}
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
		if reply := command("inspect", map[string]any{"network": p}, true); reply.Profile == nil || reply.Profile.Digest != f.profile.Digest || reply.Profile.Generation != f.profile.StateGeneration {
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
	holder, closeHolder := admissionLocalConsole(t, "holder", holderConfig)
	var maxima [3]uint32
	maxima[class-1] = 1
	request := holder(holderCommand{Operation: "request", Maxima: maxima})
	if request.Outcome != "completed" {
		t.Fatal(request)
	}
	prepared, err := allocation.Prepare(request.Request, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journalFile := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(journalFile, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	readback, err := os.ReadFile(journalFile)
	if err != nil || !bytes.Equal(readback, decision.Journal()) {
		t.Fatal(err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	signed, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if reply := holder(holderCommand{Operation: "import", Digest: request.Digest, Payload: signed}); reply.Outcome != "completed" {
		t.Fatal(reply)
	}
	challenge := token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: f.receiver.DutyGeneration, Class: uint8(class), WindowStart: time.Now().UTC().Truncate(time.Hour)}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge}, Selection: stock.ExchangeBinding{ID: [32]byte{21}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter}
	badIntent := intent
	badChallenge := challenge
	badChallenge.ReceiverNodeID = [32]byte{99}
	badIntent.Challenges = []token.ClosedTokenContext{badChallenge}
	if reply := holder(holderCommand{Operation: "begin", Intent: badIntent}); reply.Outcome != "refused" {
		t.Fatal("unknown signed recipient consumed issuance allowance", reply)
	}
	batch := holder(holderCommand{Operation: "begin", Intent: intent})
	if batch.Outcome != "completed" {
		t.Fatal(batch)
	}
	issued := admissionLocalCommand(t, "issue-current", map[string]any{"plan": f.plan, "profile": "", "network": issuerNetwork, "batch": batch.Request, "kind": quota.Bootstrap})
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued)
	}
	if reply := holder(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
		t.Fatal(reply)
	}
	presentation := stock.Presentation{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: f.receiver.NodeID, RecipientDutyGeneration: f.receiver.DutyGeneration, ChannelNonce: [32]byte{22}, Deadline: f.profile.NotAfter}
	wrong := presentation
	wrong.RecipientDutyGeneration++
	if reply := holder(holderCommand{Operation: "take", Presentation: wrong, Class: uint8(class)}); reply.Outcome != "refused" {
		t.Fatal("unaccepted recipient consumed presentation", reply)
	}
	taken := holder(holderCommand{Operation: "take", Presentation: presentation, Class: uint8(class)})
	if taken.Outcome != "completed" {
		t.Fatal(taken)
	}
	closeHolder()
	receiverConfig := map[string]any{"root": t.TempDir(), "profile": "", "network": receiverNetwork, "receiver": f.receiver, "not_after": f.profile.NotAfter}
	receiver, closeReceiver := admissionLocalConsole(t, "receiver", receiverConfig)
	deadline := time.Now().Add(time.Second)
	accept := map[string]any{"operation": "accept", "token": taken.Token, "class": class, "deadline": deadline}
	if reply := receiver(accept); reply.Outcome != "completed" || reply.Bytes != selectedBytes || !reply.Deadline.Equal(deadline) {
		t.Fatal(reply)
	}
	closeReceiver()
	reopened, closeReopened := admissionLocalConsole(t, "receiver", receiverConfig)
	if reply := reopened(accept); reply.Outcome != "refused" {
		t.Fatal("reopened new executable revived durable spend", reply)
	}
	closeReopened()
}
