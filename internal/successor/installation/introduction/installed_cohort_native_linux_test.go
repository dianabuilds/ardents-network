//go:build linux && introduction_installed_native

package introduction_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// Fixed eighteen-Record single-controller cohort. No seed/family search and no
// independent-family claim. All successful signed inputs are canonical output.
func prepareIntroductionNativeCohort(t *testing.T, command, parent string, permission [32]byte) (bundle, profile []byte, authority [32]byte, registrationPeer, submissionPeer [32]byte) {
	t.Helper()
	if permission == [32]byte{} {
		t.Fatal("explicit public Permission authority absent")
	}
	preparation := filepath.Join(parent, "canonical-preparation")
	if err := os.Mkdir(preparation, 0700); err != nil {
		t.Fatal(err)
	}
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"introduction"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("actual canonical State preparation input", args[0], err, string(raw))
		}
		return raw
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	network, seed := [32]byte{1}, [32]byte{13}
	issuerID, introductionID := [32]byte{2}, [32]byte{3}
	issuerText, introductionText := hex.EncodeToString(issuerID[:]), hex.EncodeToString(introductionID[:])
	from := time.Now().UTC().Truncate(time.Hour)
	until := from.Add(2 * time.Hour)
	stateRoot := filepath.Join(preparation, "state-key")
	var state struct {
		Public string `json:"state_public_key"`
	}
	if json.Unmarshal(call("prepare-state-key", stateRoot), &state) != nil {
		t.Fatal("canonical State-key receipt")
	}
	pub, err := hex.DecodeString(state.Public)
	if err != nil || len(pub) != 32 {
		t.Fatal("canonical State public key unavailable", err)
	}
	copy(authority[:], pub)
	epochArgs := []string{"prepare-epoch", "--network-id", hex.EncodeToString(network[:]), "--authority-key", state.Public, "--assignment-seed", hex.EncodeToString(seed[:]), "--node-id", issuerText, "--valid-from", from.Format(time.RFC3339), "--valid-until", until.Format(time.RFC3339), "--domain", "initiator", "--domain", "introduction", "--domain", "rendezvous", "--domain", "responder"}
	var issuerPublic string
	nodes := []struct{ id, family, endpoint string }{{issuerText, "closed-0-0", "127.0.0.1:4401"}, {introductionText, "closed-0-0", "127.0.0.1:4402"}}
	// Sign every member of the unchanged fixed cohort. Its selected Introduction
	// candidate is ID16; actual assignment is verified below, never redrawn.
	for i := 0; i < 16; i++ {
		id := [32]byte{byte(16 + i)}
		nodes = append(nodes, struct{ id, family, endpoint string }{hex.EncodeToString(id[:]), fmt.Sprintf("closed-descriptor-peer-%02d", i), fmt.Sprintf("127.0.0.1:%d", 4500+i)})
	}
	for index, node := range nodes {
		keyRoot := filepath.Join(preparation, "node-key-"+node.id[:2])
		var key struct {
			Public string `json:"node_public_key"`
		}
		if json.Unmarshal(call("prepare-node-key", keyRoot), &key) != nil || len(key.Public) != 64 {
			t.Fatal("canonical Node-key receipt")
		}
		if index == 0 {
			issuerPublic = key.Public
		}
		planRoot := filepath.Join(preparation, "record-plan-"+node.id[:2])
		signedRoot := filepath.Join(preparation, "record-"+node.id[:2])
		call("prepare-node-record", "--network-id", hex.EncodeToString(network[:]), "--node-id", node.id, "--node-key", key.Public, "--family", node.family, "--endpoint", node.endpoint, "--valid-from", from.Format(time.RFC3339), "--valid-until", until.Format(time.RFC3339), "--capacity", "3", "--output", planRoot)
		message := filepath.Join(planRoot, "node-record.signing-input")
		call("sign-node-record", filepath.Join(planRoot, "node-record-plan.json"), message, digest(read(message)), filepath.Join(keyRoot, "node-key.pem"), signedRoot)
		epochArgs = append(epochArgs, "--record-plan", filepath.Join(planRoot, "node-record-plan.json"), "--record", filepath.Join(signedRoot, "node-record.bin"))
	}
	epochPlan := filepath.Join(preparation, "epoch-plan")
	call(append(epochArgs, "--output", epochPlan)...)
	message := filepath.Join(epochPlan, "epoch.unsigned")
	epochRoot := filepath.Join(preparation, "epoch")
	call("sign-epoch", filepath.Join(epochPlan, "epoch-plan.json"), message, digest(read(message)), filepath.Join(stateRoot, "state-key.pem"), epochRoot)
	tokenRoot := filepath.Join(preparation, "token-material")
	call("prepare-token-keys", "--network-id", hex.EncodeToString(network[:]), "--issuer-node-id", issuerText, "--issuer-node-key", issuerPublic, "--start", from.Format(time.RFC3339), "--end", until.Format(time.RFC3339), "--output", tokenRoot)
	profilePlan := filepath.Join(preparation, "profile-plan")
	bundlePath := filepath.Join(epochRoot, "state-bundle.bin")
	profileArgs := []string{"prepare-profile", "--network-id", hex.EncodeToString(network[:]), "--authority-key", state.Public, "--issuance-authority-key", hex.EncodeToString(permission[:]), "--bundle", bundlePath, "--token-inventory", filepath.Join(tokenRoot, "public", "token-inventory.json"), "--duty", issuerText + ":2:6", "--duty", hex.EncodeToString([]byte{16}) + strings.Repeat("00", 31) + ":4:3"}
	signedBundle, err := source.DecodeBundle(read(bundlePath))
	if err != nil {
		t.Fatal(err)
	}
	decision, err := epoch.Verify(epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority[:]): authority[:]}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC()}, signedBundle.Epoch, signedBundle.Inputs, signedBundle.Materials, true)
	if err != nil || len(decision.Candidates) != 18 {
		t.Fatal("complete fixed signed cohort", err)
	}
	// The fixed original cohort, seed and families stay unchanged. ID16 is
	// checked as Introduction Domain by the genuine signed decision; there is no redraw.
	matched := false
	for _, candidate := range decision.Candidates {
		if candidate.NodeID == ([32]byte{16}) {
			matched = candidate.Domain == "introduction"
		}
	}
	if !matched {
		t.Fatal("fixed Introduction candidate was not assigned Introduction Domain")
	}
	// Select adjacent peers only from this one verified cohort, without redraw.
	for _, candidate := range decision.Candidates {
		if candidate.NodeID != ([32]byte{16}) && candidate.Domain == "introduction" && registrationPeer == ([32]byte{}) {
			registrationPeer = candidate.NodeID
		}
		if candidate.Domain == "initiator" && submissionPeer == ([32]byte{}) {
			submissionPeer = candidate.NodeID
		}
	}
	if registrationPeer == ([32]byte{}) || submissionPeer == ([32]byte{}) {
		t.Fatal("fixed cohort lacks distinct registration/submission peers")
	}

	// A syntactically valid JOIN duty cannot override this candidate's signed
	// Introduction Domain assignment; refusal precedes public output creation.
	foreign := append([]string(nil), profileArgs...)
	foreign[len(foreign)-1] = strings.TrimSuffix(foreign[len(foreign)-1], ":4:3") + ":3:4"
	foreignOutput := filepath.Join(preparation, "foreign-profile")
	refused, refusal := exec.CommandContext(t.Context(), command, append([]string{"introduction"}, append(foreign, "--output", foreignOutput)...)...).CombinedOutput()
	if refusal == nil {
		t.Fatal("foreign duty contradicted signed Domain", string(refused))
	}
	if _, err := os.Lstat(foreignOutput); !os.IsNotExist(err) {
		t.Fatal("foreign duty refusal created output", err)
	}
	duties := []string{issuerText + ":2:6", hex.EncodeToString([]byte{16}) + strings.Repeat("00", 31) + ":4:3", hex.EncodeToString(registrationPeer[:]) + ":4:2", hex.EncodeToString(submissionPeer[:]) + ":1:2"}
	sort.Strings(duties)
	profileArgs = profileArgs[:len(profileArgs)-4]
	for _, duty := range duties {
		profileArgs = append(profileArgs, "--duty", duty)
	}
	call(append(profileArgs, "--output", profilePlan)...)
	profileMessage := filepath.Join(profilePlan, "profile.unsigned")
	profileRoot := filepath.Join(preparation, "profile")
	call("sign-profile", filepath.Join(profilePlan, "profile-plan.json"), profileMessage, digest(read(profileMessage)), filepath.Join(stateRoot, "state-key.pem"), profileRoot)
	return read(bundlePath), read(filepath.Join(profileRoot, "profile.bin")), authority, registrationPeer, submissionPeer
}
