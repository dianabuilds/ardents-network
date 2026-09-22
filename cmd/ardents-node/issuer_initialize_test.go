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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestTransitIssuerInitializeRefusesBeforeRootOrIdentity(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	missing := filepath.Join(directory, "missing")
	identityPath := filepath.Join(missing, "node-identity.pem")
	root := filepath.Join(missing, "issuer-root")
	network, nodeID, initiatorID := [32]byte{81}, [32]byte{82}, [32]byte{83}
	planPath := filepath.Join(directory, "issuer-initialize.json")
	plan := map[string]any{
		"schema":               "ardents-transit-issuer-initialize-v1",
		"root":                 root,
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
	var output bytes.Buffer
	err = run(context.Background(), []string{"issuer", "initialize", "--config", planPath}, &output)
	if err == nil || err.Error() != "old Transit issuer start is retired" {
		t.Fatalf("transit issuer initialize error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("retired transit issuer initialize output = %q", output.Bytes())
	}
	for _, path := range []string{root, identityPath} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("retired transit issuer initialize changed %q: %v", path, statErr)
		}
	}
}

func TestIssuerServeRejectsTransitRuntimeAndPlanBeforeEffects(t *testing.T) {
	plan := oldDutyRetirementPlan(t)
	plan.TransitIssuer = &transitIssuerPlan{Root: filepath.Join(filepath.Dir(plan.IdentityKey), "issuer-root")}
	path := writeForwardingNodePlan(t, plan)
	if err := runIssuerNode(t.Context(), path, new(bytes.Buffer)); !errors.Is(err, errOldNodeDutyRetired) {
		t.Fatalf("transit issuer serve error = %v", err)
	}
	for _, effect := range []string{plan.TransitIssuer.Root, plan.IdentityKey, plan.StateRoot, plan.LocalRoleStateRoot} {
		if _, statErr := os.Stat(effect); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("retired transit issuer serve changed %q: %v", effect, statErr)
		}
	}
}

func TestIssuerServeAcceptsOneClosedIssuerReservation(t *testing.T) {
	issuer := node.ClosedIssuerProfile{Root: filepath.Join(t.TempDir(), "closed-issuer-root")}
	if err := validateIssuerRuntime(nodeRuntime{node: node.Config{ClosedIssuer: issuer}}); err != nil {
		t.Fatalf("closed issuer-only runtime rejected: %v", err)
	}
}

func TestClosedIssuerInitializeRejectsTransitFieldsBeforeEffects(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	directory := t.TempDir()
	missing := filepath.Join(directory, "missing")
	root := filepath.Join(missing, "closed-issuer-root")
	identity := filepath.Join(missing, "identity.pem")
	plan := map[string]any{
		"schema": "ardents-closed-issuer-initialize-v1", "root": root,
		"network_id": hex.EncodeToString(make([]byte, 32)), "node_id": hex.EncodeToString(make([]byte, 32)),
		"identity_key": identity, "not_before": now.Format(time.RFC3339), "not_after": now.Add(time.Hour).Format(time.RFC3339),
		"budget": 1,
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "mixed-issuer-initialize.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	err = run(t.Context(), []string{"issuer", "initialize", "--config", path}, new(bytes.Buffer))
	if err == nil || err.Error() != "closed issuer initialization plan is not canonical" {
		t.Fatalf("mixed issuer initialization error = %v", err)
	}
	for _, effect := range []string{root, identity} {
		if _, statErr := os.Stat(effect); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("mixed issuer initialization changed %q: %v", effect, statErr)
		}
	}
}

func TestClosedIssuerInitializeCommandPublishesOnlySPKIProfile(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	nodePublic, nodePrivate, err := ed25519.GenerateKey(rand.Reader)
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
	network, nodeID := [32]byte{91}, [32]byte{92}
	planPath := filepath.Join(directory, "closed-issuer-initialize.json")
	plan := map[string]any{"schema": "ardents-closed-issuer-initialize-v1", "root": filepath.Join(directory, "closed-issuer-root"),
		"network_id": hex.EncodeToString(network[:]), "node_id": hex.EncodeToString(nodeID[:]), "identity_key": identityPath,
		"not_before": now.Format(time.RFC3339), "not_after": now.Add(time.Hour).Format(time.RFC3339)}
	raw, err := json.Marshal(plan)
	if err != nil || os.WriteFile(planPath, raw, 0o600) != nil {
		t.Fatal("write closed issuer initialization plan")
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"issuer", "initialize", "--config", planPath}, &output); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Schema  string `json:"schema"`
		Profile []byte `json:"profile"`
	}
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	profile, err := credential.DecodeClosedIssuerProfile(receipt.Profile, nodePublic)
	if err != nil || receipt.Schema != "ardents-closed-issuer-profile-v1" || profile.NetworkID != network || profile.NodeID != nodeID || len(profile.Keys) != 3 ||
		bytes.Contains(output.Bytes(), privateDER) {
		t.Fatalf("closed issuer initialization receipt/profile = %+v, %+v, %v", receipt, profile, err)
	}
}
