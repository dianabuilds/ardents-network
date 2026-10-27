//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/custody"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

// Coherent canonical input preparation only. Installation still independently
// admits native paths, account, Release, births and delivery; a prepared plan
// or matching certificate grants no Startup, Guard or live duty.
func TestForwardingInstallInputsNativeCanonicalNodeCredentialAndPlan(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	prepareForwardingCanonicalInstallInputs(t, parent, fmt.Sprintf("/var/lib/forwarding-input-%d", os.Getpid()), forwardingPreparationOptions{})
}

type forwardingPreparationOptions struct {
	presentations, interior bool
}

type forwardingCanonicalInstallInputs struct {
	request, plan, runtime, clock, presentations, peer string
	bundle, profile                                    []byte
}

func prepareForwardingCanonicalInstallInputs(t *testing.T, parent, runtime string, options forwardingPreparationOptions) forwardingCanonicalInstallInputs {
	t.Helper()
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	password := make([]byte, 64)
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	hex.Encode(password, secret)
	clear(secret)
	defer clear(password)
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(parent, "permission-vault")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority, Binding: custody.AuthorityBinding{Environment: sha256.Sum256([]byte("coherent native installation input preparation")), Network: [32]byte{1}, Root: sha256.Sum256([]byte("separate finite Permission authority")), Kind: custody.AuthorityAdmission}}, forwardingTokenTestSecrets{password})
	if err != nil {
		t.Fatal(err)
	}
	bundleRaw, profileRaw, statePublic, peer := prepareForwardingStateNativeCohort(t, command, parent, created.AdmissionAuthority.Public, options.interior)
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical coherent input preparation", args[0], err, string(raw))
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
	preparation := filepath.Join(parent, "canonical-preparation")
	privatePath := filepath.Join(preparation, "node-key-03", "node-key.pem")
	before, err := os.Lstat(privatePath)
	if err != nil || before.Mode() != 0600 || before.Sys().(*syscall.Stat_t).Uid != 0 || before.Sys().(*syscall.Stat_t).Nlink != 1 {
		t.Fatal("original canonical Node private custody", err)
	}
	recordPath := filepath.Join(preparation, "record-03", "node-record.bin")
	sum := sha256.Sum256(read(recordPath))
	certificateOutput := filepath.Join(parent, "node-certificate")
	call("prepare-node-certificate", filepath.Join(preparation, "record-plan-03", "node-record-plan.json"), recordPath, hex.EncodeToString(sum[:]), privatePath, certificateOutput)
	certificatePath := filepath.Join(certificateOutput, "node-certificate.pem")
	block, rest := pem.Decode(read(certificatePath))
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		t.Fatal("canonical public certificate framing")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := source.DecodeBundle(bundleRaw)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := epoch.Verify(epoch.Policy{NetworkID: [32]byte{1}, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(statePublic[:]): statePublic[:]}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC()}, bundle.Epoch, bundle.Inputs, bundle.Materials, true)
	if err != nil {
		t.Fatal("independent exact signed Epoch", err)
	}
	profile, err := closedprofile.Verify(profileRaw, closedprofile.Context{NetworkID: [32]byte{1}, StateGeneration: decision.Header.Digest, EpochDigest: decision.Header.Digest, Epoch: decision.Header.Number, Authority: statePublic[:], Now: time.Now().UTC()})
	if err != nil {
		t.Fatal("independent exact signed profile", err)
	}
	nodeID := [32]byte{3}
	matched := false
	for _, candidate := range decision.Candidates {
		if candidate.NodeID == nodeID {
			public, ok := certificate.PublicKey.(ed25519.PublicKey)
			if !ok || !bytes.Equal(public, candidate.PublicKey[:]) || !certificate.NotBefore.Equal(candidate.ValidFrom) || !certificate.NotAfter.Equal(candidate.ValidUntil) || certificate.CheckSignature(certificate.SignatureAlgorithm, certificate.RawTBSCertificate, certificate.Signature) != nil {
				t.Fatal("certificate differs from actual signed Node Record")
			}
			for _, duty := range profile.Nodes {
				if duty.NodeID == nodeID {
					matched = duty.RecordDigest == candidate.RecordDigest && duty.DutyGeneration == candidate.RecordGeneration && duty.RoleDomain == 4 && duty.Subrole == 1
				}
			}
		}
	}
	if !matched || profile.AuthorityKey != created.AdmissionAuthority.Public {
		t.Fatal("State, Node and Permission binding differs")
	}
	if peer != "" {
		peerBytes, err := hex.DecodeString(peer)
		if err != nil || len(peerBytes) != 32 {
			t.Fatal("verified Interior ID malformed", err)
		}
		var peerID [32]byte
		copy(peerID[:], peerBytes)
		var member epoch.Candidate
		for _, candidate := range decision.Candidates {
			if candidate.NodeID == peerID {
				member = candidate
			}
		}
		joined := false
		for _, duty := range profile.Nodes {
			if duty.NodeID == peerID {
				joined = duty.RoleDomain == 4 && duty.Subrole == 2 && duty.RecordDigest == member.RecordDigest && duty.DutyGeneration == member.RecordGeneration
			}
		}
		if !joined || member.Domain != "introduction" || member.PublicKey == [32]byte{} {
			t.Fatal("canonical profile did not bind the actual Interior candidate")
		}
		peerRecord := filepath.Join(preparation, "record-"+peer[:2], "node-record.bin")
		peerRecordHash := sha256.Sum256(read(peerRecord))
		peerOutput := filepath.Join(parent, "peer-certificate")
		call("prepare-node-certificate", filepath.Join(preparation, "record-plan-"+peer[:2], "node-record-plan.json"), peerRecord, hex.EncodeToString(peerRecordHash[:]), filepath.Join(preparation, "node-key-"+peer[:2], "node-key.pem"), peerOutput)
		certificateRaw := read(filepath.Join(peerOutput, "node-certificate.pem"))
		block, rest := pem.Decode(certificateRaw)
		if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
			t.Fatal("actual canonical Interior certificate framing")
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		public, ok := certificate.PublicKey.(ed25519.PublicKey)
		if !ok || !bytes.Equal(public, member.PublicKey[:]) || !certificate.NotBefore.Equal(member.ValidFrom) || !certificate.NotAfter.Equal(member.ValidUntil) || certificate.CheckSignature(certificate.SignatureAlgorithm, certificate.RawTBSCertificate, certificate.Signature) != nil {
			t.Fatal("actual Interior certificate differs from authenticated Record")
		}
		writeForwardingPeerPreparation(t, parent, forwardingCanonicalPeer{Schema: "ardents-forwarding-test-peer-v1", NodeID: peerID, PublicKey: member.PublicKey, FamilyID: member.FamilyID, RecordDigest: member.RecordDigest, ProfileDigest: profile.Digest, DutyGeneration: member.RecordGeneration, Endpoint: member.Endpoint, Domain: member.Domain, RoleDomain: 4, Subrole: 2, CertificateSHA256: sha256.Sum256(certificateRaw)})
	}
	clock := filepath.Join(filepath.Dir(runtime), "clock")
	if err := os.WriteFile(clock, nil, 0644); err != nil {
		t.Fatal(err)
	}
	requestOutput, planOutput := filepath.Join(parent, "request"), filepath.Join(parent, "plan")
	// Runtime declarations can have a traversable parent distinct from the
	// original private delivery directory; all owned destinations stay absent.
	call("prepare-request", "--installation-root", filepath.Join(runtime, "node"), "--state-root", filepath.Join(runtime, "state"), "--participation-root", filepath.Join(runtime, "participation"), "--spend-root", filepath.Join(runtime, "spend"), "--hosting-root", filepath.Join(runtime, "hosting"), "--credential-directory", filepath.Join(runtime, "credentials"), "--plan", filepath.Join(planOutput, "forwarding-plan.json"), "--certificate", certificatePath, "--private-key", privatePath, "--clock", clock, "--output", requestOutput)
	request, err := forwarding.DecodeRequest(read(filepath.Join(requestOutput, "forwarding-request.json")))
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{1}
	call("prepare-plan", "--request", filepath.Join(requestOutput, "forwarding-request.json"), "--output", planOutput, "--network-id", hex.EncodeToString(network[:]), "--node-id", hex.EncodeToString(nodeID[:]), "--profile-authority", hex.EncodeToString(statePublic[:]), "--authority", hex.EncodeToString(statePublic[:]), "--threshold", "1", "--work-tx", "134217728", "--work-rx", "134217728", "--termination-tx", "1048576", "--termination-rx", "1048576", "--retirement-wait-ms", "5000")
	plan, err := forwarding.DecodePlan(read(filepath.Join(planOutput, "forwarding-plan.json")))
	if err != nil {
		t.Fatal(err)
	}
	if err := request.MatchPlan(plan); err != nil {
		t.Fatal("actual request/plan mismatch", err)
	}
	if plan.NodeID != nodeID || plan.Network.NetworkID != network || plan.Network.ProfileAuthority != statePublic || plan.Network.Threshold != 1 || len(plan.Network.Authorities) != 1 || plan.Network.Authorities[0] != statePublic || request.Certificate != certificatePath || request.PrivateKey != privatePath || plan.Certificate != filepath.Join(runtime, "credentials", "certificate.pem") || plan.PrivateKey != filepath.Join(runtime, "credentials", "private-key.pem") {
		t.Fatal("canonical runtime plan drifted from exact Node/pins/credential delivery")
	}
	after, err := os.Lstat(privatePath)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("original canonical Node key changed", err)
	}
	if _, err := os.Lstat(runtime); !os.IsNotExist(err) {
		t.Fatal("input preparation created a semantic or installation root", err)
	}
	var presentationPath string
	if options.presentations {
		var peers [][32]byte
		if peer != "" {
			raw, err := hex.DecodeString(peer)
			if err != nil || len(raw) != 32 {
				t.Fatal("authenticated peer identity unavailable", err)
			}
			var identity [32]byte
			copy(identity[:], raw)
			peers = append(peers, identity)
		}
		presentationPath = prepareForwardingChannelPresentations(t, parent, vault, created, forwardingTokenTestSecrets{password}, bundleRaw, profileRaw, statePublic, peers...)
	}
	return forwardingCanonicalInstallInputs{
		request: filepath.Join(requestOutput, "forwarding-request.json"),
		plan:    filepath.Join(planOutput, "forwarding-plan.json"),
		runtime: runtime, clock: clock, presentations: presentationPath, peer: peer, bundle: bundleRaw, profile: profileRaw,
	}
}

// Select from the whole fixed, authenticated cohort. This does not retry keys,
// families or seeds, and conveys no live State/duty or operator independence.
func forwardingCanonicalInteriorCandidate(t *testing.T, raw []byte, public [32]byte) string {
	t.Helper()
	bundle, err := source.DecodeBundle(raw)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := epoch.Verify(epoch.Policy{NetworkID: [32]byte{1}, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public[:]): public[:]}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC()}, bundle.Epoch, bundle.Inputs, bundle.Materials, true)
	if err != nil || len(decision.Candidates) != 18 {
		t.Fatal("actual fixed canonical peer cohort unavailable", err)
	}
	var adjacent epoch.Candidate
	for _, candidate := range decision.Candidates {
		if candidate.NodeID == [32]byte{3} {
			adjacent = candidate
		}
	}
	if adjacent.Domain != "introduction" || adjacent.FamilyID == [32]byte{} {
		t.Fatal("original Forwarding assignment changed")
	}
	for _, candidate := range decision.Candidates {
		if candidate.NodeID[0] >= 16 && candidate.NodeID[0] <= 31 && candidate.Domain == "introduction" && candidate.FamilyID != adjacent.FamilyID && candidate.PublicKey != adjacent.PublicKey {
			return hex.EncodeToString(candidate.NodeID[:])
		}
	}
	t.Fatal("fixed declared cohort has no distinct authenticated Interior candidate")
	return ""
}

type forwardingCanonicalPeer struct {
	Schema            string   `json:"schema"`
	NodeID            [32]byte `json:"node_id"`
	PublicKey         [32]byte `json:"public_key"`
	FamilyID          [32]byte `json:"family_id"`
	RecordDigest      [32]byte `json:"record_digest"`
	ProfileDigest     [32]byte `json:"profile_digest"`
	DutyGeneration    uint64   `json:"duty_generation"`
	Endpoint          string   `json:"endpoint"`
	Domain            string   `json:"domain"`
	RoleDomain        uint8    `json:"role_domain"`
	Subrole           uint8    `json:"subrole"`
	CertificateSHA256 [32]byte `json:"certificate_sha256"`
}

func writeForwardingPeerPreparation(t *testing.T, parent string, prepared forwardingCanonicalPeer) {
	t.Helper()
	raw, err := json.Marshal(prepared)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(parent, "peer-preparation.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(append(raw, '\n'))
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
		t.Fatal(err)
	}
}

// This separate private-manager scenario stops after initial installation and
// offline State/Hosting preparation. It never invokes installed runtime,
// Startup, a duty producer, Guard, Route, root Start, Stop or restart.
func TestForwardingInstallInputsNativeCoherentInitialStoppedPreparation(t *testing.T) {
	prepareForwardingCoherentInitialStopped(t, forwardingPreparationOptions{})
}

func TestForwardingInstallInputsNativePreparedChannelsAndInitialStopped(t *testing.T) {
	prepareForwardingCoherentInitialStopped(t, forwardingPreparationOptions{presentations: true})
}

func TestForwardingInstallInputsNativePreparedPeerAndInitialStopped(t *testing.T) {
	prepareForwardingCoherentInitialStopped(t, forwardingPreparationOptions{presentations: true, interior: true})
}

func prepareForwardingCoherentInitialStopped(t *testing.T, options forwardingPreparationOptions) {
	t.Helper()
	const parent = "/var/lib/forwarding-coherent-initial"
	marker, err := os.ReadFile("/etc/forwarding-initial-component-marker")
	if err != nil || string(marker) != "ardents-forwarding-coherent-preparation-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: exact private coherent preparation stand required", err)
	}
	var pins struct {
		PrivateRoot     [2]uint64            `json:"private_root"`
		OuterNamespaces map[string][2]uint64 `json:"outer_namespaces"`
	}
	rawPins, err := os.ReadFile("/artifacts/pins.json")
	if err != nil || json.Unmarshal(rawPins, &pins) != nil {
		t.Fatal("invalid environment: outside original stand pins required", err)
	}
	native := func(path string) *syscall.Stat_t {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.Sys().(*syscall.Stat_t)
	}
	identity := native("/")
	if pins.PrivateRoot != [2]uint64{uint64(identity.Dev), identity.Ino} || len(pins.OuterNamespaces) != 6 {
		t.Fatal("invalid environment: original private root differs")
	}
	for _, kind := range []string{"mnt", "pid", "net", "uts", "ipc", "cgroup"} {
		observed := native("/proc/self/ns/" + kind)
		if pins.OuterNamespaces[kind] == [2]uint64{uint64(observed.Dev), observed.Ino} {
			t.Fatal("invalid environment: host namespace retained", kind)
		}
	}
	if err := os.Mkdir(parent, 0755); err != nil {
		t.Fatal("fresh original preparation directory required", err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: native ext4 required", err)
	}
	command, controlBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	_, nodeBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_NODE_PROGRAM")
	preparer, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_RELEASE_PREPARER")
	evidence, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_BUILD_EVIDENCE")
	buildRoot := os.Getenv("ARDENTS_FORWARDING_BUILD_ROOT")
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	if !filepath.IsAbs(buildRoot) || digest(read(filepath.Join(buildRoot, "READY"))) != os.Getenv("ARDENTS_FORWARDING_BUILD_READY_SHA256") {
		t.Fatal("invalid environment: actual original build commitment required")
	}
	for _, number := range []string{"build-1", "build-2"} {
		if !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-node")), nodeBytes) || !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-control")), controlBytes) {
			t.Fatal("canonical commands differ from actual controlled build")
		}
	}
	delivery := filepath.Join(parent, "delivery")
	if err := os.Mkdir(delivery, 0700); err != nil {
		t.Fatal(err)
	}
	inputs := prepareForwardingCanonicalInstallInputs(t, delivery, filepath.Join(parent, "runtime"), options)
	if options.interior {
		raw := read(filepath.Join(delivery, "peer-preparation.json"))
		var peer forwardingCanonicalPeer
		if json.Unmarshal(raw, &peer) != nil || peer.Schema != "ardents-forwarding-test-peer-v1" || hex.EncodeToString(peer.NodeID[:]) != inputs.peer || peer.NodeID == [32]byte{3} || peer.PublicKey == [32]byte{} || peer.RoleDomain != 4 || peer.Subrole != 2 || peer.Domain != "introduction" || peer.DutyGeneration != 1 || peer.CertificateSHA256 != sha256.Sum256(read(filepath.Join(delivery, "peer-certificate/node-certificate.pem"))) {
			t.Fatal("actual public Interior preparation differs")
		}
	}
	if options.presentations {
		info, err := os.Lstat(inputs.presentations)
		if err != nil || info.Mode() != 0600 || info.Sys().(*syscall.Stat_t).Uid != 0 || info.Sys().(*syscall.Stat_t).Nlink != 1 {
			t.Fatal("private original channel presentations unavailable", err)
		}
		raw := read(inputs.presentations)
		defer clear(raw)
		var prepared forwardingPreparedChannels
		if json.Unmarshal(raw, &prepared) != nil || prepared.Schema != "ardents-forwarding-test-presentations-v1" || prepared.NodeID != [32]byte{3} || prepared.PublicKey == [32]byte{} || prepared.Endpoint != "127.0.0.1:4402" || !prepared.Deadline.After(time.Now()) || prepared.Channels[0].Nonce == prepared.Channels[1].Nonce {
			t.Fatal("finite public channel binding differs from canonical cohort")
		}
		var prior [][]byte
		for _, channel := range prepared.Channels {
			for _, token := range channel.Tokens {
				if len(token) == 0 {
					t.Fatal("missing genuine pre-presented credential")
				}
				for _, other := range prior {
					if bytes.Equal(token, other) {
						t.Fatal("finite channel input reused a one-use token")
					}
				}
				prior = append(prior, token)
			}
		}
		for _, token := range prior {
			clear(token)
		}
	}
	if err := os.Mkdir(inputs.runtime, 0755); err != nil {
		t.Fatal(err)
	}
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical coherent preparation", args[0], err, string(raw))
		}
		return raw
	}
	templateRoot, descriptorRoot := filepath.Join(delivery, "template"), filepath.Join(delivery, "descriptor")
	call("prepare-template", templateRoot)
	template := filepath.Join(templateRoot, "ardents-forwarding.service")
	call("prepare-descriptor", "coherent-private-forwarding", "1", filepath.Join(buildRoot, "build-1", "ardents-node"), template, descriptorRoot)
	descriptor := filepath.Join(descriptorRoot, "infrastructure-forwarding.json")
	now := time.Now().UTC().Truncate(time.Second)
	publicPlan := filepath.Join(delivery, "public-release-plan")
	args := []string{"-I", "-S", preparer, "--build-root", buildRoot, "--descriptor", descriptor, "--template", template, "--network", "coherent-private-forwarding", "--expires", now.Add(time.Hour).Format(time.RFC3339), "--no-new-work-after", now.Add(30 * time.Minute).Format(time.RFC3339), "--terminate-after", now.Add(45 * time.Minute).Format(time.RFC3339), "--qualification-evidence", evidence, "--qualification-evidence-sha256", os.Getenv("ARDENTS_FORWARDING_BUILD_EVIDENCE_SHA256"), "--output", publicPlan}
	if raw, err := exec.CommandContext(t.Context(), "python3", args...).CombinedOutput(); err != nil {
		t.Fatal("actual controlled-build public plan", err, string(raw))
	}
	keys, metadataRoot := filepath.Join(delivery, "release-keys"), filepath.Join(delivery, "metadata")
	call("prepare-release-keys", keys)
	call("sign-initial-release", filepath.Join(publicPlan, "forwarding-release-plan.json"), keys, metadataRoot, filepath.Join(delivery, "verification"))
	distribution := filepath.Join(delivery, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	publicFiles := map[string][]byte{"ardents-node-linux-amd64": nodeBytes, "ardents-forwarding.service": read(template), "infrastructure-forwarding.json": read(descriptor)}
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		publicFiles[name] = read(filepath.Join(metadataRoot, name))
	}
	for name, raw := range publicFiles {
		mode := os.FileMode(0600)
		if name == "ardents-node-linux-amd64" {
			mode = 0700
		}
		if err := os.WriteFile(filepath.Join(distribution, name), raw, mode); err != nil {
			t.Fatal(err)
		}
	}
	inventoryRoot := filepath.Join(delivery, "inventory")
	call("prepare-inventory", distribution, inventoryRoot)
	inventory := read(filepath.Join(inventoryRoot, "infrastructure-enrollment.json"))
	if err := os.WriteFile(filepath.Join(distribution, "infrastructure-enrollment.json"), inventory, 0600); err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(parent, "release-history")
	node := filepath.Join(distribution, "ardents-node-linux-amd64")
	install := []string{"forwarding", "install-initial-stopped", distribution, digest(inventory), history, "alpha", "coherent-private-forwarding", inputs.request}
	if raw, err := exec.CommandContext(t.Context(), node, install...).CombinedOutput(); err != nil || !bytes.Equal(raw, []byte("{\"outcome\":\"forwarding-initial-stopped-completed\",\"started\":false}\n")) {
		t.Fatal("actual coherent initial stopped installation", err, string(raw))
	}
	request, err := forwarding.DecodeRequest(read(inputs.request))
	if err != nil {
		t.Fatal(err)
	}
	bindingPath := filepath.Join(request.InstallationRoot, "binding.json")
	completed, err := forwarding.ReadCompleted(t.Context(), bindingPath)
	if err != nil {
		t.Fatal("independent completed initial owner", err)
	}
	if err := completed.Close(); err != nil {
		t.Fatal(err)
	}
	var binding struct {
		UID, GID   uint32
		Generation string
	}
	if json.Unmarshal(read(bindingPath), &binding) != nil || binding.UID < 100 || binding.UID > 999 || binding.GID != binding.UID {
		t.Fatal("genuine installed Node principal missing")
	}
	sealedPlan := filepath.Join(request.InstallationRoot, "generations", binding.Generation, "forwarding-plan.json")
	for _, path := range []string{request.StateRoot, request.ParticipationRoot, request.SpendRoot, request.HostingRoot} {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 || native(path).Uid != binding.UID || native(path).Gid != binding.GID {
			t.Fatal("initial semantic root already initialized or foreign", path, err)
		}
	}
	client, verifier := filepath.Join(parent, "ardents-control"), filepath.Join(parent, "state-verifier.test")
	for path, raw := range map[string][]byte{client: controlBytes, verifier: read(os.Args[0])} {
		if err := os.WriteFile(path, raw, 0555); err != nil {
			t.Fatal(err)
		}
	}
	bundlePath, profilePath := filepath.Join(parent, "bundle.bin"), filepath.Join(parent, "profile.bin")
	for path, raw := range map[string][]byte{bundlePath: inputs.bundle, profilePath: inputs.profile} {
		if err := os.WriteFile(path, raw, 0444); err != nil {
			t.Fatal(err)
		}
	}
	principal := &syscall.Credential{Uid: binding.UID, Gid: binding.GID, Groups: []uint32{}}
	if err := os.Chtimes(inputs.clock, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	accept := exec.CommandContext(t.Context(), client, "forwarding", "prepare-state", sealedPlan, bundlePath, profilePath)
	accept.SysProcAttr = &syscall.SysProcAttr{Credential: principal, Pdeathsig: syscall.SIGKILL}
	if raw, err := accept.CombinedOutput(); err != nil {
		t.Fatal("actual installed State-root offline acceptance", err, string(raw))
	}
	verify := exec.CommandContext(t.Context(), verifier, "-test.run", "^TestForwardingStateNativeCanonicalOfflineAcceptanceAndNoReset$", "-test.count=1", "-test.v", "-test.timeout=30s")
	verify.Env = append(os.Environ(), "ARDENTS_FORWARDING_STATE_VERIFY="+sealedPlan)
	if inputs.peer != "" {
		verify.Env = append(verify.Env, "ARDENTS_FORWARDING_STATE_VERIFY_PEER="+inputs.peer)
	}
	verify.SysProcAttr = &syscall.SysProcAttr{Credential: principal, Pdeathsig: syscall.SIGKILL}
	if raw, err := verify.CombinedOutput(); err != nil {
		t.Fatal("fresh actual Node-principal retained membership verification", err, string(raw))
	}
	if inputs.peer != "" {
		// This is a separate remote-leg component input, never another installed
		// Node. Canonical preparation supplies public bytes; actual State under
		// the original Node UID independently accepts and reopens its own root.
		// The provider root is the original host allowance, not a fresh budget.
		peerBase := filepath.Join(parent, "component-peer")
		if err := os.Mkdir(peerBase, 0755); err != nil {
			t.Fatal(err)
		}
		peerState := filepath.Join(peerBase, "state")
		if err := os.Mkdir(peerState, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(peerState, int(binding.UID), int(binding.GID)); err != nil {
			t.Fatal(err)
		}
		peerPreparation := filepath.Join(delivery, "canonical-preparation")
		peerKey := filepath.Join(peerPreparation, "node-key-"+inputs.peer[:2], "node-key.pem")
		peerCertificate := filepath.Join(delivery, "peer-certificate", "node-certificate.pem")
		peerPlanOutput := filepath.Join(delivery, "peer-plan")
		peerRequestOutput := filepath.Join(delivery, "peer-request")
		call("prepare-request", "--installation-root", filepath.Join(parent, "peer-node"), "--state-root", peerState, "--participation-root", filepath.Join(peerBase, "participation"), "--spend-root", filepath.Join(peerBase, "spend"), "--hosting-root", request.HostingRoot, "--credential-directory", filepath.Join(peerBase, "credentials"), "--plan", filepath.Join(peerPlanOutput, "forwarding-plan.json"), "--certificate", peerCertificate, "--private-key", peerKey, "--clock", inputs.clock, "--output", peerRequestOutput)
		originalPlan, err := forwarding.DecodePlan(read(sealedPlan))
		if err != nil {
			t.Fatal(err)
		}
		peerPlanArgs := []string{"prepare-plan", "--request", filepath.Join(peerRequestOutput, "forwarding-request.json"), "--output", peerPlanOutput, "--network-id", hex.EncodeToString(originalPlan.Network.NetworkID[:]), "--node-id", inputs.peer, "--profile-authority", hex.EncodeToString(originalPlan.Network.ProfileAuthority[:]), "--threshold", strconv.Itoa(originalPlan.Network.Threshold), "--work-tx", "134217728", "--work-rx", "134217728", "--termination-tx", "1048576", "--termination-rx", "1048576", "--retirement-wait-ms", "5000"}
		for _, public := range originalPlan.Network.Authorities {
			peerPlanArgs = append(peerPlanArgs, "--authority", hex.EncodeToString(public[:]))
		}
		call(peerPlanArgs...)
		peerPlanBytes := read(filepath.Join(peerPlanOutput, "forwarding-plan.json"))
		peerPlan, err := forwarding.DecodePlan(peerPlanBytes)
		if err != nil || peerPlan.NodeID == originalPlan.NodeID || hex.EncodeToString(peerPlan.NodeID[:]) != inputs.peer || peerPlan.Network.Root != peerState || peerPlan.HostingRoot != originalPlan.HostingRoot || peerPlan.Network.ClockObservationFile != originalPlan.Network.ClockObservationFile {
			t.Fatal("canonical peer plan differs from separate State/shared host budget", err)
		}
		// Copy canonical public bytes verbatim into a root-controlled traversable
		// input. Neither this copy nor the plan grants installed startup rights.
		peerPlanPath := filepath.Join(peerBase, "forwarding-plan.json")
		if err := os.WriteFile(peerPlanPath, peerPlanBytes, 0444); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(inputs.clock, time.Now(), time.Now()); err != nil {
			t.Fatal(err)
		}
		peerAccept := exec.CommandContext(t.Context(), client, "forwarding", "prepare-state", peerPlanPath, bundlePath, profilePath)
		peerAccept.SysProcAttr = &syscall.SysProcAttr{Credential: principal, Pdeathsig: syscall.SIGKILL}
		if raw, err := peerAccept.CombinedOutput(); err != nil {
			t.Fatal("canonical separate peer State acceptance", err, string(raw))
		}
		peerVerify := exec.CommandContext(t.Context(), verifier, "-test.run", "^TestForwardingStateNativeCanonicalOfflineAcceptanceAndNoReset$", "-test.count=1", "-test.v", "-test.timeout=30s")
		peerVerify.Env = append(os.Environ(), "ARDENTS_FORWARDING_STATE_VERIFY="+peerPlanPath, "ARDENTS_FORWARDING_STATE_VERIFY_INTERIOR=1")
		peerVerify.SysProcAttr = &syscall.SysProcAttr{Credential: principal, Pdeathsig: syscall.SIGKILL}
		if raw, err := peerVerify.CombinedOutput(); err != nil {
			t.Fatal("independently reopened Interior State", err, string(raw))
		}
		for _, absent := range []string{peerPlan.ParticipationRoot, peerPlan.SpendRoot, filepath.Join(parent, "peer-node"), filepath.Join(peerBase, "credentials")} {
			if _, err := os.Lstat(absent); !os.IsNotExist(err) {
				t.Fatal("offline peer preparation created another runtime or installation", absent, err)
			}
		}
	}
	call("prepare-hosting", "--binding", bindingPath, "--provider", "coherent-private-provider", "--start", now.Add(-time.Minute).Format(time.RFC3339), "--end", now.Add(2*time.Hour).Format(time.RFC3339), "--unit", "GiB", "--quantity", "1", "--directions", "tx+rx", "--interfaces", "lo", "--initial-used-bytes", "0", "--low-watermark-bytes", "1024")
	for _, path := range []string{request.ParticipationRoot, request.SpendRoot} {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			t.Fatal("offline preparation opened an unselected semantic root", path, err)
		}
	}
	unit, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-forwarding.service", "--property=ActiveState,SubState,ExecMainPID,ExecMainStartTimestampMonotonic,InvocationID").CombinedOutput()
	if err != nil || !bytes.Contains(unit, []byte("ActiveState=inactive\n")) || !bytes.Contains(unit, []byte("ExecMainPID=0\n")) || !bytes.Contains(unit, []byte("ExecMainStartTimestampMonotonic=0\n")) || !bytes.Contains(unit, []byte("InvocationID=\n")) {
		t.Fatal("initial preparation started installed Node", err, string(unit))
	}
	report := []byte(fmt.Sprintf("{\"binding\":%q,\"generation\":%q,\"uid\":%d,\"state_prepared\":true,\"hosting_prepared\":true,\"presentations_prepared\":%t,\"peer_prepared\":%t,\"runtime_started\":false,\"installed_qualification\":false,\"bootstrap_acceptance\":false}\n", bindingPath, binding.Generation, binding.UID, options.presentations, inputs.peer != ""))
	if err := os.WriteFile(filepath.Join(parent, "result.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
