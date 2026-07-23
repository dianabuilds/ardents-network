package identity

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"
	"time"

	identityv1 "ardents/sdk/go/protocol/identityv1"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var testNow = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

type fixture struct {
	root, device, node, app ed25519.PrivateKey
	credential              *Artifact
	credentialWire          *identityv1.KeyCredential
	delegation              *Artifact
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{
		root:   ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)),
		device: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, ed25519.SeedSize)),
		node:   ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x33}, ed25519.SeedSize)),
		app:    ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x44}, ed25519.SeedSize)),
	}
	var err error
	f.credential, err = SignKeyCredential(KeyCredentialSpec{
		Subject: principalID(f.root.Public().(ed25519.PublicKey)), RootPublicKey: f.root.Public().(ed25519.PublicKey),
		DeviceID: deviceID(f.device.Public().(ed25519.PublicKey)), DevicePublicKey: f.device.Public().(ed25519.PublicKey),
		Purposes: []CredentialPurpose{PurposeAuthenticate}, NotBefore: testNow.Add(-time.Hour), NotAfter: testNow.Add(24 * time.Hour),
	}, f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.credentialWire = new(identityv1.KeyCredential)
	raw, _ := f.credential.MarshalBinary()
	if err := proto.Unmarshal(raw, f.credentialWire); err != nil {
		t.Fatal(err)
	}
	f.delegation, err = SignDelegation(DelegationSpec{
		Delegator: principalID(f.root.Public().(ed25519.PublicKey)), Delegatee: principalID(f.app.Public().(ed25519.PublicKey)),
		Audience:  Audience{Node: principalID(f.node.Public().(ed25519.PublicKey)), Interface: InterfaceApplication, ProtocolMajor: 1},
		Actions:   []string{"application.content.put", "application.content.get", "application.content.put"},
		Scope:     ResourceScope{Kind: ScopePrincipalOwned, Owner: mustSDKResourceOwner(t, principalID(f.root.Public().(ed25519.PublicKey)))},
		NotBefore: testNow, NotAfter: testNow.Add(15 * time.Minute), Credential: f.credential,
	}, f.device, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func delegationSpec(f fixture) DelegationSpec {
	view := f.delegation.Delegation()
	return DelegationSpec{Delegator: view.Delegator, Delegatee: view.Delegatee, Audience: view.Audience, Actions: append([]string(nil), view.Actions...), Scope: view.Scope, NotBefore: view.NotBefore, NotAfter: view.NotAfter, Credential: f.credential}
}

func TestCredentialAndDelegationRoundTripAreImmutableAndRedacted(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct {
		a     *Artifact
		parse func([]byte) (*Artifact, error)
	}{
		{f.credential, func(raw []byte) (*Artifact, error) { return ParseKeyCredential(raw, testNow) }},
		{f.delegation, func(raw []byte) (*Artifact, error) { return ParseDelegation(raw, testNow) }},
	} {
		raw, _ := tc.a.MarshalBinary()
		parsed, err := tc.parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.ID() != tc.a.ID() {
			t.Fatalf("id mismatch")
		}
		raw[0] ^= 0xff
		again, _ := parsed.MarshalBinary()
		if bytes.Equal(raw, again) {
			t.Fatal("wire bytes alias caller")
		}
		text := parsed.String()
		encoded, _ := json.Marshal(parsed)
		if strings.Contains(text, "signature") || bytes.Contains(encoded, []byte("signature")) || bytes.Contains(encoded, f.device.Public().(ed25519.PublicKey)) {
			t.Fatalf("secret projection: %s %s", text, encoded)
		}
	}
	p := f.delegation.Delegation()
	p.Delegator = "mutated"
	if f.delegation.Delegation().Delegator == "mutated" {
		t.Fatal("payload accessor aliases state")
	}
}

func TestParsesAllSixCanonicalEnvelopes(t *testing.T) {
	f := newFixture(t)
	nodeID := principalID(f.node.Public().(ed25519.PublicKey))
	subject := principalID(f.root.Public().(ed25519.PublicKey))
	aud := &identityv1.Audience{Node: nodeID, Interface: identityv1.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}
	grantPayload := &identityv1.AccessGrantPayload{Version: 1, Issuer: nodeID, Subject: subject, Audience: aud, Actions: []string{"node.status"}, Scope: &identityv1.ResourceScope{Scope: &identityv1.ResourceScope_Node{Node: &identityv1.NodeScope{}}}, NotBefore: timestamppb.New(testNow), NotAfter: timestamppb.New(testNow.Add(time.Hour))}
	grant := testEnvelope(t, grantPayload, grantDomain, "ag1_", f.node, func(id string, sig []byte) proto.Message {
		return &identityv1.AccessGrant{Id: id, Payload: grantPayload, Signature: sig}
	})
	grantArtifact, err := ParseAccessGrant(grant, f.node.Public().(ed25519.PublicKey), testNow)
	if err != nil {
		t.Fatal(err)
	}
	devicePayload := &identityv1.DeviceRevocationPayload{Version: 1, TargetId: deviceID(f.device.Public().(ed25519.PublicKey)), Issuer: nodeID, Audience: aud, RevokedAt: timestamppb.New(testNow), TargetDeviceId: deviceID(f.device.Public().(ed25519.PublicKey)), Subject: subject}
	device := testEnvelope(t, devicePayload, deviceRevocationDomain, "dv1_", f.node, func(id string, sig []byte) proto.Message {
		return &identityv1.DeviceRevocation{Id: id, Payload: devicePayload, Signature: sig}
	})
	if _, err := ParseDeviceRevocation(device, f.node.Public().(ed25519.PublicKey), testNow); err != nil {
		t.Fatal(err)
	}
	grantRevPayload := &identityv1.AccessGrantRevocationPayload{Version: 1, TargetId: grantArtifact.ID(), Issuer: nodeID, Audience: aud, RevokedAt: timestamppb.New(testNow)}
	grantRev := testEnvelope(t, grantRevPayload, grantRevocationDomain, "ar1_", f.node, func(id string, sig []byte) proto.Message {
		return &identityv1.AccessGrantRevocation{Id: id, Payload: grantRevPayload, Signature: sig}
	})
	if _, err := ParseAccessGrantRevocation(grantRev, f.node.Public().(ed25519.PublicKey), testNow, grantArtifact); err != nil {
		t.Fatal(err)
	}
	delRaw, _ := f.delegation.MarshalBinary()
	delWire := new(identityv1.Delegation)
	_ = proto.Unmarshal(delRaw, delWire)
	delRevPayload := &identityv1.DelegationRevocationPayload{Version: 1, TargetId: delWire.Id, Issuer: subject, Audience: delWire.Payload.Audience, RevokedAt: timestamppb.New(testNow), Delegator: subject, Delegatee: delWire.Payload.Delegatee, Credential: f.credentialWire}
	delRev := testEnvelope(t, delRevPayload, delegationRevocationDomain, "dr1_", f.device, func(id string, sig []byte) proto.Message {
		return &identityv1.DelegationRevocation{Id: id, Payload: delRevPayload, Signature: sig}
	})
	if _, err := ParseDelegationRevocation(delRev, testNow); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseDelegationRevocation(delRev, testNow.Add(365*24*time.Hour)); err != nil {
		t.Fatalf("permanent revocation became unverifiable after credential expiry: %v", err)
	}
}

func TestRejectsNoncanonicalUnknownWrongKeyAndBoundaries(t *testing.T) {
	f := newFixture(t)
	raw, _ := f.credential.MarshalBinary()
	unknown := append(bytes.Clone(raw), protowire.AppendTag(nil, 99, protowire.VarintType)...)
	unknown = protowire.AppendVarint(unknown, 1)
	if _, err := ParseKeyCredential(unknown, testNow); err == nil {
		t.Fatal("accepted unknown field")
	}
	wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x55}, ed25519.SeedSize))
	wire := new(identityv1.KeyCredential)
	_ = proto.Unmarshal(raw, wire)
	wire.Payload.RootPublicKey = wrong.Public().(ed25519.PublicKey)
	mutated, _ := proto.MarshalOptions{Deterministic: true}.Marshal(wire)
	if _, err := ParseKeyCredential(mutated, testNow); err == nil {
		t.Fatal("accepted wrong root binding")
	}
	if _, err := ParseKeyCredential(raw, wire.Payload.NotAfter.AsTime().Add(PortableClockSkew)); err == nil {
		t.Fatal("accepted half-open skew end")
	}
	dp := delegationSpec(f)
	dp.Actions = []string{"application.content.*"}
	if _, err := SignDelegation(dp, f.device, testNow); err == nil {
		t.Fatal("accepted wildcard action")
	}
	dp = delegationSpec(f)
	dp.Actions = make([]string, MaxActions+1)
	for i := range dp.Actions {
		dp.Actions[i] = "application.content." + strings.Repeat("a", i+1)
	}
	if validateActions(dp.Actions, identityv1.Interface_INTERFACE_APPLICATION) == nil {
		t.Fatal("accepted action overflow")
	}
	dp = delegationSpec(f)
	dp.NotAfter = dp.NotBefore.Add(MaxDelegationLifetime + time.Second)
	if _, err := SignDelegation(dp, f.device, testNow); err == nil {
		t.Fatal("accepted lifetime overflow")
	}
	dp = delegationSpec(f)
	dp.Scope = ResourceScope{Kind: ScopeExact, Resource: ResourceRef{Node: dp.Audience.Node, Kind: "future-kind", CanonicalID: "x"}}
	if _, err := SignDelegation(dp, f.device, testNow); err == nil {
		t.Fatal("accepted unknown resource kind")
	}
	dp.Scope = ResourceScope{Kind: ScopeExact, Resource: ResourceRef{Node: dp.Audience.Node, Kind: "owned-content", CanonicalID: "x"}}
	if _, err := SignDelegation(dp, f.device, testNow); err == nil {
		t.Fatal("accepted owner-sensitive resource without owner")
	}
}

func TestRejectsRecursiveUnknownAndCrossArtifactSignature(t *testing.T) {
	f := newFixture(t)
	raw, _ := f.delegation.MarshalBinary()
	wire := new(identityv1.Delegation)
	_ = proto.Unmarshal(raw, wire)
	wire.Payload.Credential.Payload.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 99, protowire.VarintType), 1))
	nested, _ := proto.MarshalOptions{Deterministic: true}.Marshal(wire)
	if _, err := ParseDelegation(nested, testNow); err == nil {
		t.Fatal("accepted nested unknown field")
	}
	credentialRaw, _ := f.credential.MarshalBinary()
	credential := new(identityv1.KeyCredential)
	_ = proto.Unmarshal(credentialRaw, credential)
	credential.Signature = wire.Signature
	cross, _ := proto.MarshalOptions{Deterministic: true}.Marshal(credential)
	if _, err := ParseKeyCredential(cross, testNow); err == nil {
		t.Fatal("accepted cross-artifact signature")
	}
}

func testEnvelope(t *testing.T, payload proto.Message, domain []byte, prefix string, key ed25519.PrivateKey, envelope func(string, []byte) proto.Message) []byte {
	t.Helper()
	p, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	signed := append(bytes.Clone(domain), p...)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope(artifactID(prefix, signed), ed25519.Sign(key, signed)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func testID(prefix string, b byte) string { return artifactID(prefix, []byte{b}) }
