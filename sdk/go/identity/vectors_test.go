package identity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	identityv1 "ardents/sdk/go/protocol/identityv1"

	"google.golang.org/protobuf/proto"
)

func TestSDKConsumesCanonicalServerVectors(t *testing.T) {
	type vector struct {
		Kind                  string `json:"kind"`
		ID                    string `json:"id"`
		WireBase64            string `json:"wire_base64"`
		WireSHA256            string `json:"wire_sha256"`
		SignerPublicKeyBase64 string `json:"signer_public_key_base64"`
	}
	type boundary struct {
		Name       string `json:"name"`
		WireBase64 string `json:"wire_base64"`
		Now        string `json:"now"`
		Accepted   bool   `json:"accepted"`
	}
	var fixture struct {
		Version         uint32     `json:"version"`
		Now             string     `json:"now"`
		Vectors         []vector   `json:"vectors"`
		BoundaryVectors []boundary `json:"boundary_vectors"`
	}
	raw, err := os.ReadFile("../../../api/ardents/identity/v1/testdata/artifact-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Vectors) != 6 {
		t.Fatalf("invalid vector fixture")
	}
	if len(fixture.BoundaryVectors) != 11 {
		t.Fatal("missing boundary vectors")
	}
	for _, item := range fixture.BoundaryVectors {
		wire, decodeErr := base64.RawStdEncoding.DecodeString(item.WireBase64)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		at, parseErr := time.Parse(time.RFC3339Nano, item.Now)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		_, verifyErr := ParseKeyCredential(wire, at)
		if (verifyErr == nil) != item.Accepted {
			t.Fatalf("boundary %s accepted=%v error=%v", item.Name, item.Accepted, verifyErr)
		}
	}
	now, err := time.Parse(time.RFC3339, fixture.Now)
	if err != nil {
		t.Fatal(err)
	}
	var knownGrant *Artifact
	var knownCredential *Artifact
	for _, item := range fixture.Vectors {
		if item.Kind == "key_credential" {
			wire, decodeErr := base64.RawStdEncoding.DecodeString(item.WireBase64)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			knownCredential, err = ParseKeyCredential(wire, now)
			if err != nil {
				t.Fatal(err)
			}
		}
		if item.Kind != "access_grant" {
			continue
		}
		wire, decodeErr := base64.RawStdEncoding.DecodeString(item.WireBase64)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		public, decodeErr := base64.RawStdEncoding.DecodeString(item.SignerPublicKeyBase64)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		knownGrant, err = ParseAccessGrant(wire, ed25519.PublicKey(public), now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if knownGrant == nil {
		t.Fatal("missing access grant vector")
	}
	if knownCredential == nil {
		t.Fatal("missing Credential vector")
	}
	for _, item := range fixture.Vectors {
		item := item
		t.Run(item.Kind, func(t *testing.T) {
			wire, err := base64.RawStdEncoding.DecodeString(item.WireBase64)
			if err != nil {
				t.Fatal(err)
			}
			public, err := base64.RawStdEncoding.DecodeString(item.SignerPublicKeyBase64)
			if err != nil || len(public) != ed25519.PublicKeySize {
				t.Fatal("invalid public key")
			}
			sum := sha256.Sum256(wire)
			if hex.EncodeToString(sum[:]) != item.WireSHA256 {
				t.Fatal("wire hash mismatch")
			}
			if rebuilt := rebuildVectorWithSDKSigner(t, item.Kind, wire, knownCredential, now); !bytes.Equal(rebuilt, wire) {
				t.Fatal("SDK canonical signer bytes differ from golden vector")
			}
			var artifact *Artifact
			switch item.Kind {
			case "key_credential":
				artifact, err = ParseKeyCredential(wire, now)
			case "access_grant":
				artifact, err = ParseAccessGrant(wire, ed25519.PublicKey(public), now)
			case "delegation":
				artifact, err = ParseDelegation(wire, now)
			case "device_revocation":
				artifact, err = ParseDeviceRevocation(wire, ed25519.PublicKey(public), now)
			case "access_grant_revocation":
				artifact, err = ParseAccessGrantRevocation(wire, ed25519.PublicKey(public), now, knownGrant)
			case "delegation_revocation":
				artifact, err = ParseDelegationRevocation(wire, now)
			default:
				t.Fatalf("unknown vector kind %q", item.Kind)
			}
			if err != nil {
				t.Fatal(err)
			}
			if artifact.ID() != item.ID {
				t.Fatalf("id=%q want %q", artifact.ID(), item.ID)
			}
			mutated := append([]byte(nil), wire...)
			mutated[len(mutated)-1] ^= 1
			var mutationErr error
			switch item.Kind {
			case "key_credential":
				_, mutationErr = ParseKeyCredential(mutated, now)
			case "access_grant":
				_, mutationErr = ParseAccessGrant(mutated, ed25519.PublicKey(public), now)
			case "delegation":
				_, mutationErr = ParseDelegation(mutated, now)
			case "device_revocation":
				_, mutationErr = ParseDeviceRevocation(mutated, ed25519.PublicKey(public), now)
			case "access_grant_revocation":
				_, mutationErr = ParseAccessGrantRevocation(mutated, ed25519.PublicKey(public), now, knownGrant)
			case "delegation_revocation":
				_, mutationErr = ParseDelegationRevocation(mutated, now)
			}
			if mutationErr == nil {
				t.Fatal("bit flip accepted")
			}
			unknown := append(append([]byte(nil), wire...), 0x98, 0x06, 0x01)
			for name, candidate := range map[string][]byte{"truncated": wire[:len(wire)-1], "unknown": unknown} {
				if verifyMalformedSDKVector(item.Kind, candidate, ed25519.PublicKey(public), now, knownGrant) == nil {
					t.Fatalf("%s input accepted", name)
				}
			}
			if item.Kind == "access_grant" || item.Kind == "device_revocation" || item.Kind == "access_grant_revocation" {
				wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x99}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
				if verifyMalformedSDKVector(item.Kind, wire, wrong, now, knownGrant) == nil {
					t.Fatal("wrong signer key accepted")
				}
			}
			jsonProjection, jsonErr := json.Marshal(artifact)
			if jsonErr != nil {
				t.Fatal(jsonErr)
			}
			for _, projection := range []string{artifact.String(), fmt.Sprintf("%#v", artifact), string(jsonProjection)} {
				if strings.Contains(strings.ToLower(projection), "signature") || strings.Contains(projection, item.WireBase64) || strings.Contains(projection, item.SignerPublicKeyBase64) {
					t.Fatalf("unredacted projection: %s", projection)
				}
			}
		})
	}
}

func verifyMalformedSDKVector(kind string, wire []byte, public ed25519.PublicKey, now time.Time, knownGrant *Artifact) error {
	var err error
	switch kind {
	case "key_credential":
		_, err = ParseKeyCredential(wire, now)
	case "access_grant":
		_, err = ParseAccessGrant(wire, public, now)
	case "delegation":
		_, err = ParseDelegation(wire, now)
	case "device_revocation":
		_, err = ParseDeviceRevocation(wire, public, now)
	case "access_grant_revocation":
		_, err = ParseAccessGrantRevocation(wire, public, now, knownGrant)
	case "delegation_revocation":
		_, err = ParseDelegationRevocation(wire, now)
	}
	return err
}

func rebuildVectorWithSDKSigner(t *testing.T, kind string, wire []byte, credential *Artifact, now time.Time) []byte {
	t.Helper()
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, ed25519.SeedSize))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x33}, ed25519.SeedSize))
	switch kind {
	case "key_credential":
		var e identityv1.KeyCredential
		if proto.Unmarshal(wire, &e) != nil {
			t.Fatal("decode")
		}
		artifact, err := SignKeyCredential(credentialFromProto(e.Payload), root)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := artifact.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		return raw
	case "access_grant":
		var e identityv1.AccessGrant
		if proto.Unmarshal(wire, &e) != nil {
			t.Fatal("decode")
		}
		return testEnvelope(t, e.Payload, grantDomain, "ag1_", node, func(id string, sig []byte) proto.Message {
			return &identityv1.AccessGrant{Id: id, Payload: e.Payload, Signature: sig}
		})
	case "delegation":
		var e identityv1.Delegation
		if proto.Unmarshal(wire, &e) != nil {
			t.Fatal("decode")
		}
		artifact, err := SignDelegation(DelegationSpec{Delegator: e.Payload.Delegator, Delegatee: e.Payload.Delegatee, Audience: audienceFromProto(e.Payload.Audience), Actions: append([]string(nil), e.Payload.Actions...), Scope: scopeFromProto(e.Payload.Scope), NotBefore: e.Payload.NotBefore.AsTime(), NotAfter: e.Payload.NotAfter.AsTime(), Credential: credential}, device, now)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := artifact.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		return raw
	case "device_revocation":
		var e identityv1.DeviceRevocation
		if proto.Unmarshal(wire, &e) != nil {
			t.Fatal("decode")
		}
		return testEnvelope(t, e.Payload, deviceRevocationDomain, "dv1_", node, func(id string, sig []byte) proto.Message {
			return &identityv1.DeviceRevocation{Id: id, Payload: e.Payload, Signature: sig}
		})
	case "access_grant_revocation":
		var e identityv1.AccessGrantRevocation
		if proto.Unmarshal(wire, &e) != nil {
			t.Fatal("decode")
		}
		return testEnvelope(t, e.Payload, grantRevocationDomain, "ar1_", node, func(id string, sig []byte) proto.Message {
			return &identityv1.AccessGrantRevocation{Id: id, Payload: e.Payload, Signature: sig}
		})
	case "delegation_revocation":
		var e identityv1.DelegationRevocation
		if proto.Unmarshal(wire, &e) != nil {
			t.Fatal("decode")
		}
		return testEnvelope(t, e.Payload, delegationRevocationDomain, "dr1_", device, func(id string, sig []byte) proto.Message {
			return &identityv1.DelegationRevocation{Id: id, Payload: e.Payload, Signature: sig}
		})
	default:
		t.Fatalf("unknown vector kind %q", kind)
		return nil
	}
}
