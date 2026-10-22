package state_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// This controlled fixture serves already genuinely accepted legacy State to
// legacy client/Node regression consumers. It runs no canonical Source command,
// imports no successor runtime, and grants no installed Source qualification.
// The separately admitted canonical Source is tested in its own native profile.
func startSourceOwner(t *testing.T, path string) func() {
	t.Helper()
	var plan struct {
		StateRoot             string   `json:"state_root"`
		LocalRoleStateRoot    string   `json:"local_role_state_root"`
		NetworkID             string   `json:"network_id"`
		AuthorityPublic       []string `json:"authority_public"`
		Threshold             int      `json:"threshold"`
		At                    string   `json:"at"`
		Listen                string   `json:"listen"`
		ServerCertificate     string   `json:"server_certificate"`
		ServerKey             string   `json:"server_key"`
		ClientRoot            string   `json:"client_root"`
		ClientKeyDigests      []string `json:"client_key_digests"`
		MaterializationIndex  uint32   `json:"materialization_index"`
		StateProfile          string   `json:"state_profile"`
		StateProfileAuthority string   `json:"state_profile_authority"`
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &plan); err != nil {
		t.Fatalf("controlled Source fixture declaration: %v", err)
	}
	decode := func(text string) []byte {
		raw, err := hex.DecodeString(text)
		if err != nil || len(raw) != 32 {
			t.Fatalf("controlled Source fixture public pin: %v", err)
		}
		return raw
	}
	at, err := time.Parse(time.RFC3339, plan.At)
	if err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	config := state.Config{Root: plan.StateRoot, LocalRoleStateRoot: plan.LocalRoleStateRoot, Threshold: plan.Threshold,
		Authorities: map[[32]byte]ed25519.PublicKey{}, Clock: func() time.Time { return at.Add(time.Since(began)).UTC() },
		AcceptedProfile: plan.StateProfile, Source: source.Config{ServeAddress: plan.Listen, MaterialIndex: plan.MaterializationIndex}}
	copy(config.NetworkID[:], decode(plan.NetworkID))
	for _, text := range plan.AuthorityPublic {
		public := decode(text)
		config.Authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	if plan.StateProfileAuthority != "" {
		config.ClosedProfileAuthority = ed25519.PublicKey(decode(plan.StateProfileAuthority))
	}
	config.Source.ServeCertificate, err = tls.LoadX509KeyPair(plan.ServerCertificate, plan.ServerKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if private, ok := config.Source.ServeCertificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(private)
		}
	})
	config.Source.ServeClientRootPEM, err = os.ReadFile(plan.ClientRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range plan.ClientKeyDigests {
		var pin [32]byte
		copy(pin[:], decode(text))
		config.Source.ServeClientKeyDigests = append(config.Source.ServeClientKeyDigests, pin)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal("genuine Source fixture owner", err)
	}
	var once sync.Once
	var closeErr error
	stop := func() {
		once.Do(func() {
			closeErr = owner.Close()
			if private, ok := config.Source.ServeCertificate.PrivateKey.(ed25519.PrivateKey); ok {
				clear(private)
			}
		})
		if closeErr != nil {
			t.Error("genuine Source fixture physical close", closeErr)
		}
	}
	t.Cleanup(stop)
	if _, err := owner.Current(); err != nil {
		stop()
		t.Fatal("genuine Source fixture accepted State", err)
	}
	return stop
}
