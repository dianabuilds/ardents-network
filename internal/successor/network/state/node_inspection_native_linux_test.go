//go:build linux && network_signing_native

package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Genuine independently encoded signed Record/Epoch input goes through the
// public Epoch verifier and State intake. The public profile preparer plus an
// isolated test State signer then supplies ordinary closed-profile intake.
// No fake authenticated Header, mutable runtime owner or installed role appears.
func nodeInspectionNativeFixture(t *testing.T) (state.Config, state.NodeHistory) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root Node inspection driver required")
	}
	root, roles := t.TempDir(), t.TempDir()
	var fs syscall.Statfs_t
	if err := syscall.Statfs(root, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: private root-controlled ext4 TMPDIR required", err)
	}
	now := time.Unix(fixtureNow, 0).UTC()
	network := sha256.Sum256([]byte("native Node inspection network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	public := authority.Public().(ed25519.PublicKey)
	inputs := make([][]byte, 16)
	records := make([]fixtureRecord, 16)
	for i := range records {
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(i + 1)}, ed25519.SeedSize))
		id := sha256.Sum256(private.Public().(ed25519.PublicKey))
		records[i] = buildTestRecord(t, network, id, private, fmt.Sprintf("inspection-family-%02d", i), fmt.Sprintf("127.0.0.1:%d", 4200+i), 1, now.Truncate(time.Hour).Add(-time.Minute), now.Truncate(time.Hour).Add(2*time.Hour))
		inputs[i] = records[i].bytes
	}
	built := buildTestEpoch(t, testEpochSpec{networkID: network, number: 1, validFrom: now.Truncate(time.Hour), validUntil: now.Truncate(time.Hour).Add(2 * time.Hour),
		inputs: inputs, accepted: records, assignmentSeed: sha256.Sum256([]byte("native Node inspection assignments")), domains: []string{"initiator", "introduction", "rendezvous", "responder"}, authorities: []ed25519.PrivateKey{authority}})
	policy := epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1, Profile: "ardents-route-v3", Now: now}
	decision, err := epoch.Verify(policy, built.Raw, inputs, built.Materials, true)
	if err != nil {
		t.Fatal(err)
	}
	var issuer, forwarding epoch.Candidate
	for _, candidate := range decision.Candidates {
		if candidate.Domain == "rendezvous" && issuer.NodeID == [32]byte{} {
			issuer = candidate
		}
		if candidate.Domain == "initiator" && forwarding.NodeID == [32]byte{} {
			forwarding = candidate
		}
	}
	if issuer.NodeID == [32]byte{} || forwarding.NodeID == [32]byte{} {
		t.Fatal("fixture lacks distinct actual signed issuer and forwarding assignments")
	}
	encodedGeneration, err := hex.DecodeString(decision.Snapshot.Generation)
	if err != nil || len(encodedGeneration) != 32 {
		t.Fatal("verified generation", err)
	}
	var generation [32]byte
	copy(generation[:], encodedGeneration)
	nodes := []closedprofile.NodeInput{
		{NodeID: issuer.NodeID, RecordDigest: issuer.RecordDigest, RoleDomain: 2, Subrole: 6, DutyGeneration: issuer.RecordGeneration},
		{NodeID: forwarding.NodeID, RecordDigest: forwarding.RecordDigest, RoleDomain: 1, Subrole: 1, DutyGeneration: forwarding.RecordGeneration},
	}
	sort.Slice(nodes, func(i, j int) bool { return bytes.Compare(nodes[i].NodeID[:], nodes[j].NodeID[:]) < 0 })
	body, err := closedprofile.Prepare(closedprofile.Input{NetworkID: network, StateGeneration: generation, EpochDigest: decision.Header.Digest, Epoch: decision.Header.Number,
		IssuerNodeID: issuer.NodeID, IssuanceAuthorityKey: issuer.PublicKey, NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour), Nodes: nodes,
		TokenKeys: []closedprofile.TokenKeyInput{{WindowStart: now.Truncate(time.Hour), Class: 1, SPKI: nodeInspectionNativeSPKI(t)}}})
	if err != nil {
		t.Fatal(err)
	}
	// Independent fixed-domain signature bytes, not the signing helper's output.
	profile := append(bytes.Clone(body), ed25519.Sign(authority, append([]byte("ardents-closed-profile-v3\x00"), body...))...)
	clock := filepath.Join(t.TempDir(), "clock")
	if err := os.WriteFile(clock, []byte("independent native clock observation"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(clock, now, now); err != nil {
		t.Fatal(err)
	}
	cfg := state.Config{Root: root, NetworkID: network, Authorities: policy.Authorities, Threshold: 1, ClosedProfileAuthority: public, AcceptedProfile: policy.Profile,
		Now: now, ClockObservationFile: clock, LocalRoleStateRoot: roles}
	owner, err := state.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Accept(t.Context(), built.Raw, inputs, built.Materials); err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if _, err := owner.AcceptClosedProfile(profile); err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	participation, err := duty.Open(duty.Config{Root: roles, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := participation.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, roles} {
		if err := filepath.WalkDir(path, func(path string, _ os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return os.Chown(path, 61100, 61100)
		}); err != nil {
			t.Fatal(err)
		}
	}
	return cfg, state.NodeHistory{NetworkID: network, ProfileDigest: sha256.Sum256(profile), NodeID: forwarding.NodeID, PublicKey: forwarding.PublicKey, DutyGeneration: forwarding.RecordGeneration}
}

// This independent test encoder preserves the closed RSA-PSS/SHA384 grammar.
// The generated private RSA key is never used to supply accepting token rights.
func nodeInspectionNativeSPKI(t *testing.T) []byte {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	type algorithm struct {
		Algorithm  asn1.ObjectIdentifier
		Parameters asn1.RawValue
	}
	hash := algorithm{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}, asn1.RawValue{FullBytes: []byte{5, 0}}}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := asn1.Marshal(struct {
		Hash algorithm `asn1:"explicit,tag:0"`
		Mask algorithm `asn1:"explicit,tag:1"`
		Salt int       `asn1:"explicit,tag:2"`
	}{hash, algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}, asn1.RawValue{FullBytes: hashDER}}, 48})
	if err != nil {
		t.Fatal(err)
	}
	public, err := asn1.Marshal(private.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := asn1.Marshal(struct {
		Algorithm algorithm
		Key       asn1.BitString
	}{algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}, asn1.RawValue{FullBytes: parameters}}, asn1.BitString{Bytes: public, BitLength: len(public) * 8}})
	if err != nil || !closedprofile.ValidateTokenSPKI(spki) {
		t.Fatal("independent test SPKI", err)
	}
	return spki
}

func TestNodeInspectionNativeSignedOriginalLifetime(t *testing.T) {
	cfg, expected := nodeInspectionNativeFixture(t)
	before := sourceInspectionNativeImages(t, cfg) // physical image helper only
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	inspection, err := state.OpenNodeInspection(ctx, cfg, expected.NodeID, 61100, 61100)
	if err != nil {
		t.Fatal(err)
	}
	defer inspection.Close()
	copy := inspection
	for range 3 {
		actual, err := copy.Check()
		if err != nil || actual != expected {
			t.Fatal("signed original binding differs", actual, expected, err)
		}
	}
	foreign, err := state.OpenNodeInspection(ctx, cfg, expected.NodeID, 61100, 61100)
	if err == nil {
		_ = foreign.Close()
		t.Fatal("inspection lent original State/participation leases")
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}
	if facts, err := copy.Check(); err == nil || facts != (state.NodeHistory{}) {
		t.Fatal("closed copy published binding facts", facts, err)
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	after := sourceInspectionNativeImages(t, cfg)
	if len(before) != len(after) {
		t.Fatal("read-only inspection changed signed history inventory")
	}
	for path, original := range before {
		actual, found := after[path]
		if !found || original.digest != actual.digest || !os.SameFile(original.info, actual.info) || original.info.Mode() != actual.info.Mode() || original.info.Sys().(*syscall.Stat_t).Ctim != actual.info.Sys().(*syscall.Stat_t).Ctim {
			t.Fatal("read-only inspection mutated original history", path)
		}
	}
	next, err := state.OpenNodeInspection(ctx, cfg, expected.NodeID, 61100, 61100)
	if err != nil {
		t.Fatal("actual joined Close retained original leases", err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeInspectionNativeSignedAndMissingHistoryRefuse(t *testing.T) {
	for _, fault := range []string{"State-marker", "participation-pointer", "profile-authority", "expired", "principal", "foreign-Node"} {
		t.Run(fault, func(t *testing.T) {
			cfg, expected := nodeInspectionNativeFixture(t)
			uid := uint32(61100)
			switch fault {
			case "State-marker":
				if err := os.Remove(filepath.Join(cfg.Root, ".ardents-network-state-v1")); err != nil {
					t.Fatal(err)
				}
			case "participation-pointer":
				if err := os.Remove(filepath.Join(cfg.LocalRoleStateRoot, "current")); err != nil {
					t.Fatal(err)
				}
			case "profile-authority":
				foreign := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xe1}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
				cfg.Authorities[sha256.Sum256(foreign)] = foreign
				cfg.ClosedProfileAuthority = foreign
			case "expired":
				cfg.Now = cfg.Now.Add(48 * time.Hour)
				if err := os.Chtimes(cfg.ClockObservationFile, cfg.Now, cfg.Now); err != nil {
					t.Fatal(err)
				}
			case "principal":
				uid++
			case "foreign-Node":
				expected.NodeID[0] ^= 1
			}
			before := sourceInspectionNativeImages(t, cfg)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			for range 2 {
				inspection, err := state.OpenNodeInspection(ctx, cfg, expected.NodeID, uid, 61100)
				if err == nil {
					_ = inspection.Close()
					t.Fatal("unavailable signed/original history admitted", fault)
				}
				if facts, err := inspection.Check(); err == nil || facts != (state.NodeHistory{}) {
					t.Fatal("failed constructor exposed binding facts", facts, err)
				}
			}
			after := sourceInspectionNativeImages(t, cfg)
			if len(before) != len(after) {
				t.Fatal("refusal created missing history")
			}
			for path, original := range before {
				actual, found := after[path]
				if !found || original.digest != actual.digest || !os.SameFile(original.info, actual.info) {
					t.Fatal("refusal repaired or reset original history", path)
				}
			}
		})
	}
}

func TestNodeInspectionNativeCancellationRetainsOriginalClockReader(t *testing.T) {
	cfg, expected := nodeInspectionNativeFixture(t)
	now := cfg.Now
	cfg.Now = time.Time{}
	entered, resume := make(chan struct{}), make(chan struct{})
	var hold atomic.Bool
	var release sync.Once
	defer release.Do(func() { close(resume) })
	cfg.Clock = func() time.Time {
		if hold.CompareAndSwap(true, false) {
			close(entered)
			<-resume
		}
		return now
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	inspection, err := state.OpenNodeInspection(ctx, cfg, expected.NodeID, 61100, 61100)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inspection.Close() })
	hold.Store(true)
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() {
		facts, err := inspection.Check()
		if facts != (state.NodeHistory{}) {
			checked <- errors.New("held canceled observation published binding facts")
			return
		}
		checked <- err
	}()
	select {
	case <-entered:
	case err := <-checked:
		t.Fatal("signed check did not reach original clock read", err)
	case <-ctx.Done():
		t.Fatal("original clock boundary not reached", ctx.Err())
	}
	cancel()
	if facts, err := inspection.Check(); !errors.Is(err, context.Canceled) || facts != (state.NodeHistory{}) {
		t.Fatal("cancellation did not seal fresh observation", facts, err)
	}
	go func() { closed <- inspection.Close() }()
	fresh, finish := context.WithTimeout(t.Context(), 5*time.Second)
	defer finish()
	foreign, err := state.OpenNodeInspection(fresh, cfg, expected.NodeID, 61100, 61100)
	if err == nil {
		_ = foreign.Close()
		t.Fatal("held original reader returned State lease before join")
	}
	select {
	case err := <-closed:
		t.Fatal("Close returned before original clock callback joined", err)
	default:
	}
	release.Do(func() { close(resume) })
	if err := <-checked; !errors.Is(err, context.Canceled) {
		t.Fatal("late observation lost original cancellation", err)
	}
	terminal := <-closed
	if !errors.Is(terminal, context.Canceled) {
		t.Fatal("Close lost original cancellation", terminal)
	}
	if err := inspection.Close(); err != terminal {
		t.Fatal("copied Close changed original terminal result", err, terminal)
	}
	next, err := state.OpenNodeInspection(fresh, cfg, expected.NodeID, 61100, 61100)
	if err != nil {
		t.Fatal("actual joined readers did not return original leases", err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeInspectionNativeCopiedChecksJoinBeforeClose(t *testing.T) {
	cfg, expected := nodeInspectionNativeFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	inspection, err := state.OpenNodeInspection(ctx, cfg, expected.NodeID, 61100, 61100)
	if err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	for range 4 {
		readers.Add(1)
		go func(copy state.NodeInspection) {
			defer readers.Done()
			for range 3 {
				_, _ = copy.Check()
			}
		}(inspection)
	}
	if err := inspection.Close(); err != nil {
		t.Fatal("local Close invented operational failure", err)
	}
	readers.Wait()
	next, err := state.OpenNodeInspection(ctx, cfg, expected.NodeID, 61100, 61100)
	if err != nil {
		t.Fatal("original readers did not actually join", err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
}
