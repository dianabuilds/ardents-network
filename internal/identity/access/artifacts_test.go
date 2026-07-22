package access

import (
	"bytes"
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var artifactTestNow = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

func TestKeyCredentialCanonicalRoundTrip(t *testing.T) {
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, ed25519.SeedSize))
	subject, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)

	payload := &identityprotocol.KeyCredentialPayload{
		Version: 1, Subject: subject.String(), RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(artifactTestNow.Add(-time.Minute)), NotAfter: timestamppb.New(artifactTestNow.Add(24 * time.Hour)),
	}
	artifact, err := SignKeyCredential(payload, root)
	require.NoError(t, err)
	require.NotContains(t, artifact.String(), "signature")

	raw, err := artifact.MarshalBinary()
	require.NoError(t, err)
	parsed, err := ParseAndVerifyKeyCredential(raw, artifactTestNow)
	require.NoError(t, err)
	require.Equal(t, artifact.ID(), parsed.ID())
	wire := new(identityprotocol.KeyCredential)
	require.NoError(t, proto.Unmarshal(raw, wire))
	payloadRaw, _ := proto.MarshalOptions{Deterministic: true}.Marshal(wire.Payload)
	wire.Signature = ed25519.Sign(root, append(append([]byte(nil), grantDomain...), payloadRaw...))
	wrongDomain, _ := proto.MarshalOptions{Deterministic: true}.Marshal(wire)
	_, err = ParseAndVerifyKeyCredential(wrongDomain, artifactTestNow)
	require.Error(t, err, "cross-artifact signature domain must fail")

	raw[len(raw)-1] ^= 1
	_, err = ParseAndVerifyKeyCredential(raw, artifactTestNow)
	require.Error(t, err)
}

func TestKeyCredentialRejectsRootAsDeviceKey(t *testing.T) {
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x19}, ed25519.SeedSize))
	subject, _ := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	deviceID, _ := identityprincipal.DeviceFromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	_, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{Version: 1, Subject: subject.String(), RootPublicKey: root.Public().(ed25519.PublicKey), DeviceId: deviceID.String(), DevicePublicKey: root.Public().(ed25519.PublicKey), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(artifactTestNow), NotAfter: timestamppb.New(artifactTestNow.Add(time.Hour))}, root)
	require.Error(t, err)
}

func TestAccessGrantNormalizesActionsAndRejectsUnknownAction(t *testing.T) {
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x31}, ed25519.SeedSize))
	subjectKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x32}, ed25519.SeedSize))
	node, _ := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	subject, _ := identityprincipal.FromEd25519PublicKey(subjectKey.Public().(ed25519.PublicKey))
	payload := &identityprotocol.AccessGrantPayload{
		Version: 1, Issuer: node.String(), Subject: subject.String(),
		Audience:  &identityprotocol.Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1},
		Actions:   []string{"node.status", "node.start", "node.status"},
		Scope:     &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}},
		NotBefore: timestamppb.New(artifactTestNow), NotAfter: timestamppb.New(artifactTestNow.Add(time.Hour)),
	}
	artifact, err := SignAccessGrant(payload, nodeKey)
	require.NoError(t, err)
	raw, _ := artifact.MarshalBinary()
	parsed, err := ParseAndVerifyAccessGrant(raw, nodeKey.Public().(ed25519.PublicKey), artifactTestNow)
	require.NoError(t, err)
	require.Equal(t, []string{"node.start", "node.status"}, parsed.AccessGrantPayload().GetActions())
	maxLifetime := proto.Clone(payload).(*identityprotocol.AccessGrantPayload)
	maxLifetime.NotAfter = timestamppb.New(maxLifetime.NotBefore.AsTime().Add(365 * 24 * time.Hour))
	_, err = SignAccessGrant(maxLifetime, nodeKey)
	require.NoError(t, err)
	maxLifetime.NotAfter = timestamppb.New(maxLifetime.NotBefore.AsTime().Add(365*24*time.Hour + time.Second))
	_, err = SignAccessGrant(maxLifetime, nodeKey)
	require.Error(t, err)

	payload.Actions = []string{"node.*"}
	_, err = SignAccessGrant(payload, nodeKey)
	require.Error(t, err)

	payload.Actions = []string{"data.get_blob"}
	payload.Scope = &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Exact{Exact: &identityprotocol.ExactScope{Resource: &identityprotocol.ResourceRef{Node: subject.String(), Kind: "content-blob", CanonicalId: "bafy-test"}}}}
	_, err = SignAccessGrant(payload, nodeKey)
	require.Error(t, err, "exact scope cannot cross Node audience")
	payload.Scope.GetExact().Resource.Node = node.String()
	payload.Scope.GetExact().Resource.Kind = "unknown-resource"
	_, err = SignAccessGrant(payload, nodeKey)
	require.Error(t, err, "unknown resource kind must fail closed")
	payload.Scope.GetExact().Resource.Kind = "content-blob"
	payload.Scope.GetExact().Resource.Owner = subject.String()
	payload.Scope.GetExact().Resource.CanonicalId = strings.Repeat("x", 512)
	_, err = SignAccessGrant(payload, nodeKey)
	require.NoError(t, err)
	payload.Scope.GetExact().Resource.CanonicalId += "x"
	_, err = SignAccessGrant(payload, nodeKey)
	require.Error(t, err, "canonical resource ID max+1 must fail")
}

func TestDelegationRequiresApplicationAudienceAndDeviceCredential(t *testing.T) {
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	appKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x43}, ed25519.SeedSize))
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x44}, ed25519.SeedSize))
	delegator, _ := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	delegatee, _ := identityprincipal.FromEd25519PublicKey(appKey.Public().(ed25519.PublicKey))
	node, _ := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	deviceID, _ := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	credential, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: 1, Subject: delegator.String(), RootPublicKey: root.Public().(ed25519.PublicKey), DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(artifactTestNow.Add(-time.Hour)), NotAfter: timestamppb.New(artifactTestNow.Add(24 * time.Hour)),
	}, root)
	require.NoError(t, err)
	credentialWire := new(identityprotocol.KeyCredential)
	credentialRaw, _ := credential.MarshalBinary()
	require.NoError(t, proto.Unmarshal(credentialRaw, credentialWire))
	payload := &identityprotocol.DelegationPayload{
		Version: 1, Delegator: delegator.String(), Delegatee: delegatee.String(),
		Audience: &identityprotocol.Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1},
		Actions:  []string{"application.content.get"}, Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: delegator.String()}}},
		NotBefore: timestamppb.New(artifactTestNow), NotAfter: timestamppb.New(artifactTestNow.Add(15 * time.Minute)), Credential: credentialWire,
	}
	artifact, err := SignDelegation(payload, device, artifactTestNow)
	require.NoError(t, err)
	raw, _ := artifact.MarshalBinary()
	_, err = ParseAndVerifyDelegation(raw, artifactTestNow)
	require.NoError(t, err)

	payload.Audience.Interface = identityprotocol.Interface_INTERFACE_OPERATOR
	_, err = SignDelegation(payload, device, artifactTestNow)
	require.Error(t, err)
	payload.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
	revocation := &identityprotocol.DelegationRevocationPayload{Version: 1, TargetId: artifact.ID(), Issuer: delegator.String(), Audience: payload.Audience, RevokedAt: timestamppb.New(artifactTestNow), Delegator: delegator.String(), Delegatee: delegatee.String(), Credential: credentialWire}
	revoked, err := SignDelegationRevocation(revocation, device, artifactTestNow)
	require.NoError(t, err)
	revokedRaw, _ := revoked.MarshalBinary()
	_, err = ParseAndVerifyDelegationRevocation(revokedRaw, artifactTestNow.Add(30*24*time.Hour))
	require.NoError(t, err, "permanent revocation remains verifiable after Credential expiry")
	wrong := proto.Clone(revocation).(*identityprotocol.DelegationRevocationPayload)
	wrong.Audience.Interface = identityprotocol.Interface_INTERFACE_OPERATOR
	_, err = SignDelegationRevocation(wrong, device, artifactTestNow)
	require.Error(t, err)
	wrong = proto.Clone(revocation).(*identityprotocol.DelegationRevocationPayload)
	wrong.Issuer = delegatee.String()
	_, err = SignDelegationRevocation(wrong, device, artifactTestNow)
	require.Error(t, err)
	wrong = proto.Clone(revocation).(*identityprotocol.DelegationRevocationPayload)
	wrong.TargetId = artifactID("ag1_", []byte("wrong target type"))
	_, err = SignDelegationRevocation(wrong, device, artifactTestNow)
	require.Error(t, err)
}

func TestDeviceRevocationIsNodeLocalAndTargetsDeviceID(t *testing.T) {
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize))
	rootKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x52}, ed25519.SeedSize))
	deviceKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x53}, ed25519.SeedSize))
	node, _ := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	subject, _ := identityprincipal.FromEd25519PublicKey(rootKey.Public().(ed25519.PublicKey))
	device, _ := identityprincipal.DeviceFromEd25519PublicKey(deviceKey.Public().(ed25519.PublicKey))
	payload := &identityprotocol.DeviceRevocationPayload{Version: 1, TargetId: device.String(), TargetDeviceId: device.String(), Subject: subject.String(), Issuer: node.String(), Audience: &identityprotocol.Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, RevokedAt: timestamppb.New(artifactTestNow)}
	artifact, err := SignDeviceRevocation(payload, nodeKey, artifactTestNow)
	require.NoError(t, err)
	raw, _ := artifact.MarshalBinary()
	replay, err := SignDeviceRevocation(payload, nodeKey, artifactTestNow)
	require.NoError(t, err)
	replayRaw, _ := replay.MarshalBinary()
	require.Equal(t, artifact.ID(), replay.ID())
	require.Equal(t, raw, replayRaw, "same canonical revocation must be idempotent")
	_, err = ParseAndVerifyDeviceRevocation(raw, nodeKey.Public().(ed25519.PublicKey), artifactTestNow)
	require.NoError(t, err)

	payload.TargetId = "ag1_wrong"
	_, err = SignDeviceRevocation(payload, nodeKey, artifactTestNow)
	require.Error(t, err)
	payload.TargetId = device.String()
	payload.RevokedAt = timestamppb.New(artifactTestNow.Add(120 * time.Second))
	_, err = SignDeviceRevocation(payload, nodeKey, artifactTestNow)
	require.NoError(t, err)
	payload.RevokedAt = timestamppb.New(artifactTestNow.Add(120*time.Second + time.Second))
	_, err = SignDeviceRevocation(payload, nodeKey, artifactTestNow)
	require.Error(t, err)
}

func TestStrictParserRejectsUnknownAndNonCanonicalGrantActions(t *testing.T) {
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x61}, ed25519.SeedSize))
	subjectKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x62}, ed25519.SeedSize))
	node, _ := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	subject, _ := identityprincipal.FromEd25519PublicKey(subjectKey.Public().(ed25519.PublicKey))
	payload := &identityprotocol.AccessGrantPayload{Version: 1, Issuer: node.String(), Subject: subject.String(), Audience: &identityprotocol.Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, Actions: []string{"node.start", "node.status"}, Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}}, NotBefore: timestamppb.New(artifactTestNow), NotAfter: timestamppb.New(artifactTestNow.Add(time.Hour))}
	artifact, err := SignAccessGrant(payload, nodeKey)
	require.NoError(t, err)
	raw, _ := artifact.MarshalBinary()
	_, err = ParseAndVerifyAccessGrant(append(raw, 0x98, 0x06, 0x01), nodeKey.Public().(ed25519.PublicKey), artifactTestNow)
	require.Error(t, err)

	envelope := new(identityprotocol.AccessGrant)
	require.NoError(t, proto.Unmarshal(raw, envelope))
	envelope.Payload.Actions = []string{"node.status", "node.start"}
	payloadRaw, _ := proto.MarshalOptions{Deterministic: true}.Marshal(envelope.Payload)
	signed := append(append([]byte(nil), grantDomain...), payloadRaw...)
	envelope.Id = artifactID("ag1_", signed)
	envelope.Signature = ed25519.Sign(nodeKey, signed)
	noncanonical, _ := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
	_, err = ParseAndVerifyAccessGrant(noncanonical, nodeKey.Public().(ed25519.PublicKey), artifactTestNow)
	require.Error(t, err)
}

func TestPortableArtifactClockSkewIsExactlyHalfOpen(t *testing.T) {
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x72}, ed25519.SeedSize))
	subject, _ := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	deviceID, _ := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	start := artifactTestNow
	end := start.Add(365 * 24 * time.Hour)
	artifact, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{Version: 1, Subject: subject.String(), RootPublicKey: root.Public().(ed25519.PublicKey), DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(start), NotAfter: timestamppb.New(end)}, root)
	require.NoError(t, err)
	raw, _ := artifact.MarshalBinary()
	_, err = ParseAndVerifyKeyCredential(raw, start.Add(-120*time.Second))
	require.NoError(t, err)
	_, err = ParseAndVerifyKeyCredential(raw, start.Add(-120*time.Second-time.Nanosecond))
	require.Error(t, err)
	_, err = ParseAndVerifyKeyCredential(raw, end.Add(120*time.Second-time.Nanosecond))
	require.NoError(t, err)
	_, err = ParseAndVerifyKeyCredential(raw, end.Add(120*time.Second))
	require.Error(t, err)

	overlong := &identityprotocol.KeyCredentialPayload{Version: 1, Subject: subject.String(), RootPublicKey: root.Public().(ed25519.PublicKey), DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE, identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(start), NotAfter: timestamppb.New(end.Add(time.Second))}
	_, err = SignKeyCredential(overlong, root)
	require.Error(t, err)
	overlong.NotAfter = timestamppb.New(end)
	artifact, err = SignKeyCredential(overlong, root)
	require.NoError(t, err, "constructor must deduplicate set-like purposes")

	nonCanonicalTime := proto.Clone(overlong).(*identityprotocol.KeyCredentialPayload)
	nonCanonicalTime.NotBefore.Nanos = 1
	_, err = SignKeyCredential(nonCanonicalTime, root)
	require.Error(t, err)

	lower := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	overlong.NotBefore, overlong.NotAfter = timestamppb.New(lower), timestamppb.New(lower.Add(time.Hour))
	_, err = SignKeyCredential(overlong, root)
	require.NoError(t, err)
	upper := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	overlong.NotBefore, overlong.NotAfter = timestamppb.New(upper.Add(-time.Hour)), timestamppb.New(upper.Add(-time.Second))
	_, err = SignKeyCredential(overlong, root)
	require.NoError(t, err)
	overlong.NotAfter = timestamppb.New(upper)
	_, err = SignKeyCredential(overlong, root)
	require.Error(t, err, "2100 upper bound is exclusive")
}

func TestAccessGrantRevocationRequiresKnownTarget(t *testing.T) {
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x81}, ed25519.SeedSize))
	subjectKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x82}, ed25519.SeedSize))
	node, _ := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	subject, _ := identityprincipal.FromEd25519PublicKey(subjectKey.Public().(ed25519.PublicKey))
	audience := &identityprotocol.Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}
	grant, err := SignAccessGrant(&identityprotocol.AccessGrantPayload{Version: 1, Issuer: node.String(), Subject: subject.String(), Audience: audience, Actions: []string{"node.status"}, Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}}, NotBefore: timestamppb.New(artifactTestNow), NotAfter: timestamppb.New(artifactTestNow.Add(time.Hour))}, nodeKey)
	require.NoError(t, err)
	payload := &identityprotocol.AccessGrantRevocationPayload{Version: 1, TargetId: grant.ID(), Issuer: node.String(), Audience: audience, RevokedAt: timestamppb.New(artifactTestNow)}
	artifact, err := SignAccessGrantRevocation(payload, nodeKey, artifactTestNow, grant)
	require.NoError(t, err)
	raw, _ := artifact.MarshalBinary()
	_, err = ParseAndVerifyAccessGrantRevocation(raw, nodeKey.Public().(ed25519.PublicKey), artifactTestNow, nil)
	require.Error(t, err)
	wrongGrant, err := SignAccessGrant(&identityprotocol.AccessGrantPayload{Version: 1, Issuer: node.String(), Subject: subject.String(), Audience: audience, Actions: []string{"node.start"}, Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}}, NotBefore: timestamppb.New(artifactTestNow), NotAfter: timestamppb.New(artifactTestNow.Add(time.Hour))}, nodeKey)
	require.NoError(t, err)
	_, err = ParseAndVerifyAccessGrantRevocation(raw, nodeKey.Public().(ed25519.PublicKey), artifactTestNow, wrongGrant)
	require.Error(t, err)
	_, err = ParseAndVerifyAccessGrantRevocation(raw, nodeKey.Public().(ed25519.PublicKey), artifactTestNow, grant)
	require.NoError(t, err)
	wrongAudience := proto.Clone(payload).(*identityprotocol.AccessGrantRevocationPayload)
	wrongAudience.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
	_, err = SignAccessGrantRevocation(wrongAudience, nodeKey, artifactTestNow, grant)
	require.Error(t, err, "revocation audience must equal target Grant audience")
	otherKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x83}, ed25519.SeedSize))
	otherNode, _ := identityprincipal.FromEd25519PublicKey(otherKey.Public().(ed25519.PublicKey))
	wrongIssuer := proto.Clone(payload).(*identityprotocol.AccessGrantRevocationPayload)
	wrongIssuer.Issuer, wrongIssuer.Audience.Node = otherNode.String(), otherNode.String()
	_, err = SignAccessGrantRevocation(wrongIssuer, otherKey, artifactTestNow, grant)
	require.Error(t, err, "revocation signer must be original Grant issuer")
}
