// Package main generates deterministic identity artifact test vectors.
// It does not own runtime identity validation or protocol policy.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type vector struct {
	Kind               string `json:"kind"`
	ID                 string `json:"id"`
	WireBase64         string `json:"wire_base64"`
	WireSHA256         string `json:"wire_sha256"`
	SignerPublicBase64 string `json:"signer_public_key_base64"`
}

type fixture struct {
	Version         uint32           `json:"version"`
	Now             string           `json:"now"`
	Vectors         []vector         `json:"vectors"`
	BoundaryVectors []boundaryVector `json:"boundary_vectors"`
}

type boundaryVector struct {
	Name       string `json:"name"`
	WireBase64 string `json:"wire_base64"`
	Now        string `json:"now"`
	Accepted   bool   `json:"accepted"`
}

func main() {
	out := flag.String("out", "api/ardents/identity/v1/testdata/artifact-vectors.json", "output path")
	check := flag.Bool("check", false, "verify output without changing it")
	flag.Parse()
	raw, err := generate()
	if err != nil {
		panic(err)
	}
	if *check {
		existing, err := os.ReadFile(*out)
		if err != nil || !bytes.Equal(existing, raw) {
			panic("identity artifact vectors are stale")
		}
		return
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		panic(err)
	}
}

func generate() ([]byte, error) {
	now := time.Date(2032, 3, 4, 5, 6, 7, 0, time.UTC)
	root := key(0x11)
	device := key(0x22)
	nodeKey := key(0x33)
	appKey := key(0x44)
	rootID := principal(root)
	deviceID := devicePrincipal(device)
	nodeID := principal(nodeKey)
	appID := principal(appKey)
	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{Version: 1, Subject: rootID, RootPublicKey: pub(root), DeviceId: deviceID, DevicePublicKey: pub(device), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(now.Add(-time.Hour)), NotAfter: timestamppb.New(now.Add(90 * 24 * time.Hour))}, root)
	if err != nil {
		return nil, err
	}
	credentialWire := new(identityprotocol.KeyCredential)
	credentialRaw, err := credential.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(credentialRaw, credentialWire); err != nil {
		return nil, err
	}
	grant, err := identityaccess.SignAccessGrant(&identityprotocol.AccessGrantPayload{Version: 1, Issuer: nodeID, Subject: rootID, Audience: &identityprotocol.Audience{Node: nodeID, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}, Actions: []string{"application.content.put", "application.content.get"}, Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: rootID}}}, NotBefore: timestamppb.New(now), NotAfter: timestamppb.New(now.Add(30 * 24 * time.Hour))}, nodeKey)
	if err != nil {
		return nil, err
	}
	delegation, err := identityaccess.SignDelegation(&identityprotocol.DelegationPayload{Version: 1, Delegator: rootID, Delegatee: appID, Audience: &identityprotocol.Audience{Node: nodeID, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}, Actions: []string{"application.content.get"}, Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: rootID}}}, NotBefore: timestamppb.New(now), NotAfter: timestamppb.New(now.Add(15 * time.Minute)), Credential: credentialWire}, device, now)
	if err != nil {
		return nil, err
	}
	deviceRev, err := identityaccess.SignDeviceRevocation(&identityprotocol.DeviceRevocationPayload{Version: 1, TargetId: deviceID, Issuer: nodeID, Audience: &identityprotocol.Audience{Node: nodeID, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}, RevokedAt: timestamppb.New(now), TargetDeviceId: deviceID, Subject: rootID}, nodeKey, now)
	if err != nil {
		return nil, err
	}
	grantRev, err := identityaccess.SignAccessGrantRevocation(&identityprotocol.AccessGrantRevocationPayload{Version: 1, TargetId: grant.ID(), Issuer: nodeID, Audience: &identityprotocol.Audience{Node: nodeID, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}, RevokedAt: timestamppb.New(now)}, nodeKey, now, grant)
	if err != nil {
		return nil, err
	}
	delegationRev, err := identityaccess.SignDelegationRevocation(&identityprotocol.DelegationRevocationPayload{Version: 1, TargetId: delegation.ID(), Issuer: rootID, Audience: &identityprotocol.Audience{Node: nodeID, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}, RevokedAt: timestamppb.New(now), Delegator: rootID, Delegatee: appID, Credential: credentialWire}, device, now)
	if err != nil {
		return nil, err
	}
	items := []struct {
		kind     string
		artifact *identityaccess.Artifact
		public   ed25519.PublicKey
	}{{"key_credential", credential, pub(root)}, {"access_grant", grant, pub(nodeKey)}, {"delegation", delegation, pub(device)}, {"device_revocation", deviceRev, pub(nodeKey)}, {"access_grant_revocation", grantRev, pub(nodeKey)}, {"delegation_revocation", delegationRev, pub(device)}}
	f := fixture{Version: 1, Now: now.Format(time.RFC3339), Vectors: make([]vector, 0, len(items))}
	for _, item := range items {
		wire, marshalErr := item.artifact.MarshalBinary()
		if marshalErr != nil {
			return nil, marshalErr
		}
		sum := sha256.Sum256(wire)
		f.Vectors = append(f.Vectors, vector{Kind: item.kind, ID: item.artifact.ID(), WireBase64: base64.RawStdEncoding.EncodeToString(wire), WireSHA256: hex.EncodeToString(sum[:]), SignerPublicBase64: base64.RawStdEncoding.EncodeToString(item.public)})
	}
	base := proto.Clone(credentialWire.Payload).(*identityprotocol.KeyCredentialPayload)
	addBoundary := func(name string, p *identityprotocol.KeyCredentialPayload, at time.Time, accepted bool) error {
		wire, wireErr := uncheckedCredential(p, root)
		if wireErr != nil {
			return wireErr
		}
		f.BoundaryVectors = append(f.BoundaryVectors, boundaryVector{Name: name, WireBase64: base64.RawStdEncoding.EncodeToString(wire), Now: at.Format(time.RFC3339Nano), Accepted: accepted})
		return nil
	}
	lower := time.Unix(identitycontract.LowerTimestampUnix, 0).UTC()
	upper := time.Unix(identitycontract.UpperTimestampUnix, 0).UTC()
	cases := []struct {
		name     string
		mutate   func(*identityprotocol.KeyCredentialPayload)
		at       time.Time
		accepted bool
	}{
		{"timestamp_lower_2020", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(lower)
			p.NotAfter = timestamppb.New(lower.Add(time.Hour))
		}, lower, true},
		{"timestamp_upper_last_second", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(upper.Add(-time.Hour))
			p.NotAfter = timestamppb.New(upper.Add(-time.Second))
		}, upper.Add(-time.Second), true},
		{"timestamp_upper_2100", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(upper.Add(-time.Hour))
			p.NotAfter = timestamppb.New(upper)
		}, upper.Add(-time.Second), false},
		{"timestamp_zero", func(p *identityprotocol.KeyCredentialPayload) { p.NotBefore = &timestamppb.Timestamp{} }, now, false},
		{"timestamp_nanos", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(now)
			p.NotBefore.Nanos = 1
		}, now, false},
		{"lifetime_max", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(now)
			p.NotAfter = timestamppb.New(now.Add(identitycontract.MaxCredentialLifetime))
		}, now, true},
		{"lifetime_max_plus_one", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(now)
			p.NotAfter = timestamppb.New(now.Add(identitycontract.MaxCredentialLifetime + time.Second))
		}, now, false},
		{"not_before_minus_120", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(now)
			p.NotAfter = timestamppb.New(now.Add(time.Hour))
		}, now.Add(-identitycontract.PortableClockSkew), true},
		{"not_before_before_skew", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(now)
			p.NotAfter = timestamppb.New(now.Add(time.Hour))
		}, now.Add(-identitycontract.PortableClockSkew - time.Second), false},
		{"expiry_before_plus_120", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(now.Add(-time.Hour))
			p.NotAfter = timestamppb.New(now)
		}, now.Add(identitycontract.PortableClockSkew - time.Second), true},
		{"expiry_at_plus_120", func(p *identityprotocol.KeyCredentialPayload) {
			p.NotBefore = timestamppb.New(now.Add(-time.Hour))
			p.NotAfter = timestamppb.New(now)
		}, now.Add(identitycontract.PortableClockSkew), false},
	}
	for _, tc := range cases {
		p := proto.Clone(base).(*identityprotocol.KeyCredentialPayload)
		tc.mutate(p)
		if err := addBoundary(tc.name, p, tc.at, tc.accepted); err != nil {
			return nil, err
		}
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func uncheckedCredential(payload *identityprotocol.KeyCredentialPayload, key ed25519.PrivateKey) ([]byte, error) {
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
	if err != nil {
		return nil, err
	}
	signed := append([]byte(identitycontract.KeyCredentialDomain), raw...)
	sum := sha256.Sum256(signed)
	id := identitycontract.KeyCredentialPrefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
	return proto.MarshalOptions{Deterministic: true}.Marshal(&identityprotocol.KeyCredential{Id: id, Payload: payload, Signature: ed25519.Sign(key, signed)})
}

func key(b byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{b}, ed25519.SeedSize))
}
func pub(k ed25519.PrivateKey) ed25519.PublicKey { return k.Public().(ed25519.PublicKey) }
func principal(k ed25519.PrivateKey) string {
	id, err := identityprincipal.FromEd25519PublicKey(pub(k))
	if err != nil {
		panic(err)
	}
	return id.String()
}
func devicePrincipal(k ed25519.PrivateKey) string {
	id, err := identityprincipal.DeviceFromEd25519PublicKey(pub(k))
	if err != nil {
		panic(err)
	}
	return id.String()
}
