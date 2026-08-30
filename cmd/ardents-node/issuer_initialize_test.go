package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestIssuerInitializeCommandPublishesOnlyStablePublicProfile(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	nodePublic, nodePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(nodePrivate)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	identityPath := filepath.Join(directory, "node-identity.pem")
	if err := os.WriteFile(identityPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	network, nodeID, initiatorID := [32]byte{81}, [32]byte{82}, [32]byte{83}
	planPath := filepath.Join(directory, "issuer-initialize.json")
	plan := map[string]any{
		"schema":               "ardents-transit-issuer-initialize-v1",
		"root":                 filepath.Join(directory, "issuer-root"),
		"network_id":           hex.EncodeToString(network[:]),
		"node_id":              hex.EncodeToString(nodeID[:]),
		"identity_key":         identityPath,
		"initiator_node_id":    hex.EncodeToString(initiatorID[:]),
		"initiator_public_key": hex.EncodeToString(initiatorPublic),
		"assignment_not_after": now.Add(time.Hour).Format(time.RFC3339),
		"budget":               4,
	}
	raw, err := json.Marshal(plan)
	if err != nil || os.WriteFile(planPath, raw, 0o600) != nil {
		t.Fatal("write issuer initialization plan")
	}
	initialize := func() []byte {
		t.Helper()
		var output bytes.Buffer
		if err := run(context.Background(), []string{"issuer", "initialize", "--config", planPath}, &output); err != nil {
			t.Fatal(err)
		}
		return output.Bytes()
	}
	first := initialize()
	second := initialize()
	if !bytes.Equal(first, second) {
		t.Fatal("issuer initialize retry changed its public receipt")
	}
	var receipt struct {
		Schema        string `json:"schema"`
		Profile       []byte `json:"profile"`
		ProfileSHA256 string `json:"profile_sha256"`
	}
	if err := json.Unmarshal(first, &receipt); err != nil {
		t.Fatal(err)
	}
	profile, err := credential.DecodeProfile(receipt.Profile)
	if err != nil || receipt.Schema != "ardents-transit-issuer-profile-v1" || profile.NetworkID != network ||
		profile.NodeID != nodeID || profile.InitiatorNodeID != initiatorID || profile.InitiatorPublicKey != [32]byte(initiatorPublic) {
		t.Fatalf("issuer initialization receipt/profile = %+v, %+v, %v", receipt, profile, err)
	}
	if err := credential.VerifyProfile(profile, network, nodeID, [32]byte(nodePublic), now, now.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestIssuerServeRejectsAnyOtherLocalDutyReservation(t *testing.T) {
	issuer := node.TransitIssuerProfile{Root: filepath.Join(t.TempDir(), "issuer-root")}
	if err := validateIssuerRuntime(nodeRuntime{node: node.Config{TransitIssuer: issuer}}); err != nil {
		t.Fatalf("issuer-only runtime rejected: %v", err)
	}
	withInitiator := nodeRuntime{node: node.Config{TransitIssuer: issuer}}
	withInitiator.node.Initiator.Certificate.PrivateKey = ed25519.PrivateKey{1}
	if err := validateIssuerRuntime(withInitiator); err == nil {
		t.Fatal("issuer serve accepted an Initiator reservation")
	}
}
