//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// constructionReferenceState builds one independently signed accepted State
// and returns the pin configuration that authenticates it. acceptProfile
// controls whether its current closed profile is durably accepted.
func constructionReferenceState(t *testing.T, acceptProfile bool) (state.Config, source.Bundle, []byte) {
	t.Helper()
	base := t.TempDir()
	authority := ed25519.NewKeyFromSeed(make([]byte, 32))
	nodeSeed := [32]byte{1}
	nodeKey := ed25519.NewKeyFromSeed(nodeSeed[:])
	facts, _ := participantStockFactsAt(t, time.Now().UTC(), 2)
	networkID := facts.NetworkID
	spec := networkfixture.ClosedSpec{NetworkID: networkID, Seed: [32]byte{7}, IssuanceAuthority: facts.IssuanceAuthorityKey,
		Number: 1, NotBefore: facts.NotBefore, NotAfter: facts.NotAfter, Authority: authority,
		Nodes: []networkfixture.ClosedNode{{RecordSpec: networkfixture.RecordSpec{NodeID: facts.IssuerNodeID, Generation: 1,
			ValidFrom: facts.NotBefore, ValidUntil: facts.NotAfter, Endpoint: "127.0.0.1:4401", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 3, PrivateKey: nodeKey}, RoleDomain: 2, Subrole: 6}}}
	for _, key := range facts.TokenKeys[:facts.TokenKeyCount] {
		spec.Keys = append(spec.Keys, networkfixture.ClosedTokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI[:]})
	}
	signed, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	public := authority.Public().(ed25519.PublicKey)
	root := filepath.Join(base, "reference-state")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	clock := filepath.Join(base, "reference-clock")
	if err := os.WriteFile(clock, nil, 0644); err != nil {
		t.Fatal(err)
	}
	config := state.Config{Root: root, NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		ClosedProfileAuthority: public, AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: clock}
	owner, err := state.OpenFresh(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.Accept(context.Background(), signed.Epoch.Raw, signed.Epoch.Inputs, signed.Epoch.Materials)
	if err == nil && acceptProfile {
		_, err = owner.AcceptClosedProfile(signed.Profile)
	}
	if err := errors.Join(err, owner.Close()); err != nil {
		t.Fatal("independent signed reference State", err)
	}
	return config, source.Bundle{Epoch: signed.Epoch.Raw, Inputs: signed.Epoch.Inputs, Materials: signed.Epoch.Materials}, signed.Profile
}

// constructionSourcePlan writes genuine credential files and returns the
// distributed Source declaration artifact grammar.
func constructionSourcePlan(t *testing.T, base string) SourcePlan {
	t.Helper()
	credentials := filepath.Join(base, "credentials")
	if err := os.Mkdir(credentials, 0700); err != nil {
		t.Fatal(err)
	}
	issue := func(name string, serial int64, usage x509.ExtKeyUsage) (string, string, string) {
		public, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{name},
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
			KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		der, err := x509.CreateCertificate(rand.Reader, template, template, public, key)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
		clear(encoded)
		certificatePath := filepath.Join(credentials, name+".pem")
		keyPath := filepath.Join(credentials, name+".key")
		if err := os.WriteFile(certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
			t.Fatal(err)
		}
		pin := sha256.Sum256(append([]byte("ardents-h3-source-transport-key-v1\x00"), public...))
		return certificatePath, keyPath, hex.EncodeToString(pin[:])
	}
	clientCertificate, clientKey, _ := issue("client.construction.test", 2, x509.ExtKeyUsageClientAuth)
	rootOne, _, pinOne := issue("source-one.construction.test", 3, x509.ExtKeyUsageServerAuth)
	rootTwo, _, pinTwo := issue("source-two.construction.test", 4, x509.ExtKeyUsageServerAuth)
	seed := [32]byte{5}
	identity := func(n byte) string { value := [32]byte{n}; return hex.EncodeToString(value[:]) }
	plan := SourcePlan{Schema: "ardents-source-plan-v1", ClockObservedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		OrderSeed: hex.EncodeToString(seed[:]), MaterializationIndex: 3, RefreshIntervalMS: 30000,
		ClientCertificate: clientCertificate, ClientKey: clientKey}
	plan.Sources = append(plan.Sources,
		constructionSourceMember("127.0.0.1:9401", "source-one.construction.test", identity(71), "source-one", rootOne, pinOne),
		constructionSourceMember("127.0.0.1:9402", "source-two.construction.test", identity(72), "source-two", rootTwo, pinTwo))
	return plan
}

func constructionSourceMember(address, serverName, identity, family, root, pin string) struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
} {
	return struct {
		Address        string `json:"address"`
		ServerName     string `json:"server_name"`
		Identity       string `json:"identity"`
		Family         string `json:"family"`
		EndpointHandle string `json:"endpoint_handle"`
		RootCA         string `json:"root_ca"`
		LeafKeyDigest  string `json:"leaf_key_digest"`
	}{Address: address, ServerName: serverName, Identity: identity, Family: family,
		EndpointHandle: family, RootCA: root, LeafKeyDigest: pin}
}

func constructionDeclaration(base string) PlanDeclaration {
	encoded := func(n byte) string { key := [32]byte{n}; return hex.EncodeToString(key[:]) }
	return PlanDeclaration{NetworkStateRoot: filepath.Join(base, "state"), LocalRoleStateRoot: filepath.Join(base, "participation"),
		TextTokenRoot: filepath.Join(base, "stock"), EntryStateRoot: filepath.Join(base, "route"),
		TimeConfidenceFile: filepath.Join(base, "clock"), ApplicationSocket: filepath.Join(base, "application.sock"),
		AdministrationSocket: filepath.Join(base, "administration.sock"), PublicationRoot: filepath.Join(base, "publication"),
		ServiceInstanceRoot: filepath.Join(base, "instance"),
		BrokerID:            encoded(11), ConnectionPrincipal: encoded(12), AdministrationPrincipal: encoded(13),
		ReaderPermission:    PermissionPlan{RequestPath: filepath.Join(base, "reader-request"), ResponsePath: filepath.Join(base, "reader-response"), Maxima: [3]uint32{1, 1, 1}},
		PublisherPermission: PermissionPlan{RequestPath: filepath.Join(base, "publisher-request"), ResponsePath: filepath.Join(base, "publisher-response"), Maxima: [3]uint32{1, 1, 1}}}
}

// The constructed files must carry exactly the authenticated reference-State
// pins, render canonically, and remain consumable by the genuine first
// preparation this constructor serves.
func TestConstructPlansRendersAuthenticatedPreparation(t *testing.T) {
	config, bundle, profile := constructionReferenceState(t, true)
	base := t.TempDir()
	declaration := constructionDeclaration(base)
	declared := constructionSourcePlan(t, base)
	plan, headlessJSON, sourceJSON, err := ConstructPlans(context.Background(), config, declaration, declared)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Schema != "ardents-headless-runtime-v2" || plan.NetworkID != hex.EncodeToString(config.NetworkID[:]) ||
		plan.ClosedProfileAuthority != hex.EncodeToString(config.ClosedProfileAuthority) || plan.NetworkThreshold != config.Threshold ||
		!reflect.DeepEqual(plan.NetworkAuthorities, []string{hex.EncodeToString(config.ClosedProfileAuthority)}) {
		t.Fatalf("constructed plan does not carry the authenticated pins: %+v", plan)
	}
	if headlessJSON[len(headlessJSON)-1] != '\n' {
		t.Fatal("constructed headless plan is not canonical")
	}
	var decoded HeadlessPlan
	if err := json.Unmarshal(headlessJSON, &decoded); err != nil || !reflect.DeepEqual(decoded, plan) {
		t.Fatalf("constructed headless bytes do not decode to the rendered plan: %v %+v", err, decoded)
	}
	var decodedSource SourcePlan
	if err := json.Unmarshal(sourceJSON, &decodedSource); err != nil || len(decodedSource.Sources) != 2 || decodedSource.Schema != "ardents-source-plan-v1" {
		t.Fatalf("constructed source bytes do not decode to the declared plan: %v", err)
	}
	// The rendered plan feeds the genuine first preparation end to end.
	for _, path := range []string{plan.NetworkStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot, plan.EntryStateRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(plan.TimeConfidenceFile, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := PrepareHistories(context.Background(), decoded, bundle, profile); err != nil {
		t.Fatal("constructed plan did not prepare histories", err)
	}
}

func TestConstructPlansRendersReaderRole(t *testing.T) {
	config, _, _ := constructionReferenceState(t, true)
	base := t.TempDir()
	declaration := constructionDeclaration(base)
	declaration.Role = "reader"
	declaration.PublicationRoot = ""
	declaration.ServiceInstanceRoot = ""
	declaration.AdministrationSocket = ""
	declaration.AdministrationPrincipal = ""
	declaration.PublisherPermission = PermissionPlan{}
	if _, _, _, err := ConstructPlans(context.Background(), config, declaration, constructionSourcePlan(t, base)); err != nil {
		t.Fatal(err)
	}
}

// Every refusal is causal and happens before any byte is rendered.
func TestConstructPlansRefusesUnauthenticatedOrInvalidInput(t *testing.T) {
	config, _, _ := constructionReferenceState(t, true)
	base := t.TempDir()
	declaration := constructionDeclaration(base)
	declared := constructionSourcePlan(t, base)
	wrongPrivate := ed25519.NewKeyFromSeed(append([]byte{1}, make([]byte, 31)...))
	wrongPublic := wrongPrivate.Public().(ed25519.PublicKey)
	substitutedKeyPath := filepath.Join(base, "substituted.key")
	encoded, err := x509.MarshalPKCS8PrivateKey(wrongPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(substitutedKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600); err != nil {
		t.Fatal(err)
	}
	clear(encoded)
	withoutProfile, _, _ := constructionReferenceState(t, false)
	refusals := map[string]func(network *state.Config, declaration *PlanDeclaration, sourcePlan *SourcePlan){
		"wrong network identity": func(network *state.Config, _ *PlanDeclaration, _ *SourcePlan) {
			network.NetworkID[0] ^= 0xff
		},
		"wrong authority set": func(network *state.Config, _ *PlanDeclaration, _ *SourcePlan) {
			network.Authorities = map[[32]byte]ed25519.PublicKey{sha256.Sum256(wrongPublic): wrongPublic}
		},
		"wrong threshold": func(network *state.Config, _ *PlanDeclaration, _ *SourcePlan) {
			network.Threshold = 2
		},
		"unpinned profile authority": func(network *state.Config, _ *PlanDeclaration, _ *SourcePlan) {
			network.ClosedProfileAuthority = wrongPublic
		},
		"state without closed profile": func(network *state.Config, _ *PlanDeclaration, _ *SourcePlan) {
			*network = withoutProfile
		},
		"relative state root": func(_ *state.Config, declaration *PlanDeclaration, _ *SourcePlan) {
			declaration.NetworkStateRoot = "relative-state"
		},
		"reader with publisher inputs": func(_ *state.Config, declaration *PlanDeclaration, _ *SourcePlan) {
			declaration.Role = "reader"
		},
		"empty permission allocation": func(_ *state.Config, declaration *PlanDeclaration, _ *SourcePlan) {
			declaration.ReaderPermission.Maxima = [3]uint32{}
		},
		"invalid broker principal": func(_ *state.Config, declaration *PlanDeclaration, _ *SourcePlan) {
			declaration.BrokerID = "zz"
		},
		"overlapping preparation roots": func(_ *state.Config, declaration *PlanDeclaration, _ *SourcePlan) {
			declaration.EntryStateRoot = filepath.Join(declaration.NetworkStateRoot, "entry")
		},
		"reference State overlap": func(_ *state.Config, declaration *PlanDeclaration, _ *SourcePlan) {
			declaration.NetworkStateRoot = config.Root
		},
		"substituted client key": func(_ *state.Config, _ *PlanDeclaration, sourcePlan *SourcePlan) {
			sourcePlan.ClientKey = substitutedKeyPath
		},
		"missing source root certificate": func(_ *state.Config, _ *PlanDeclaration, sourcePlan *SourcePlan) {
			sourcePlan.Sources[0].RootCA = filepath.Join(base, "absent.pem")
		},
		"zero order seed": func(_ *state.Config, _ *PlanDeclaration, sourcePlan *SourcePlan) {
			sourcePlan.OrderSeed = hex.EncodeToString(make([]byte, 32))
		},
		"unbounded refresh interval": func(_ *state.Config, _ *PlanDeclaration, sourcePlan *SourcePlan) {
			sourcePlan.RefreshIntervalMS = 0
		},
	}
	for name, refuse := range refusals {
		t.Run(name, func(t *testing.T) {
			network, localDeclaration, sourcePlan := config, declaration, declared
			refuse(&network, &localDeclaration, &sourcePlan)
			if _, _, rendered, err := ConstructPlans(context.Background(), network, localDeclaration, sourcePlan); err == nil || rendered != nil {
				t.Fatalf("unauthenticated input rendered plans: %v", err)
			}
		})
	}
}
