package access

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

	"github.com/stretchr/testify/require"
)

func TestServerConsumesCanonicalArtifactVectors(t *testing.T) {
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
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.Equal(t, uint32(1), fixture.Version)
	now, err := time.Parse(time.RFC3339, fixture.Now)
	require.NoError(t, err)
	require.Len(t, fixture.Vectors, 6)
	require.Len(t, fixture.BoundaryVectors, 11)
	for _, item := range fixture.BoundaryVectors {
		wire, decodeErr := base64.RawStdEncoding.DecodeString(item.WireBase64)
		require.NoError(t, decodeErr)
		at, parseErr := time.Parse(time.RFC3339Nano, item.Now)
		require.NoError(t, parseErr)
		_, verifyErr := ParseAndVerifyKeyCredential(wire, at)
		if item.Accepted {
			require.NoError(t, verifyErr, item.Name)
		} else {
			require.Error(t, verifyErr, item.Name)
		}
	}
	var knownGrant *Artifact
	for _, item := range fixture.Vectors {
		if item.Kind != "access_grant" {
			continue
		}
		wire, decodeErr := base64.RawStdEncoding.DecodeString(item.WireBase64)
		require.NoError(t, decodeErr)
		public, decodeErr := base64.RawStdEncoding.DecodeString(item.SignerPublicKeyBase64)
		require.NoError(t, decodeErr)
		knownGrant, err = ParseAndVerifyAccessGrant(wire, ed25519.PublicKey(public), now)
		require.NoError(t, err)
	}
	require.NotNil(t, knownGrant)
	for _, item := range fixture.Vectors {
		item := item
		t.Run(item.Kind, func(t *testing.T) {
			wire, err := base64.RawStdEncoding.DecodeString(item.WireBase64)
			require.NoError(t, err)
			public, err := base64.RawStdEncoding.DecodeString(item.SignerPublicKeyBase64)
			require.NoError(t, err)
			require.Len(t, public, ed25519.PublicKeySize)
			sum := sha256.Sum256(wire)
			require.Equal(t, item.WireSHA256, hex.EncodeToString(sum[:]))
			var artifact *Artifact
			switch item.Kind {
			case "key_credential":
				artifact, err = ParseAndVerifyKeyCredential(wire, now)
			case "access_grant":
				artifact, err = ParseAndVerifyAccessGrant(wire, ed25519.PublicKey(public), now)
			case "delegation":
				artifact, err = ParseAndVerifyDelegation(wire, now)
			case "device_revocation":
				artifact, err = ParseAndVerifyDeviceRevocation(wire, ed25519.PublicKey(public), now)
			case "access_grant_revocation":
				artifact, err = ParseAndVerifyAccessGrantRevocation(wire, ed25519.PublicKey(public), now, knownGrant)
			case "delegation_revocation":
				artifact, err = ParseAndVerifyDelegationRevocation(wire, now)
			default:
				t.Fatalf("unknown vector kind %q", item.Kind)
			}
			require.NoError(t, err)
			require.Equal(t, item.ID, artifact.ID())
			mutated := append([]byte(nil), wire...)
			mutated[len(mutated)-1] ^= 1
			var mutationErr error
			switch item.Kind {
			case "key_credential":
				_, mutationErr = ParseAndVerifyKeyCredential(mutated, now)
			case "access_grant":
				_, mutationErr = ParseAndVerifyAccessGrant(mutated, ed25519.PublicKey(public), now)
			case "delegation":
				_, mutationErr = ParseAndVerifyDelegation(mutated, now)
			case "device_revocation":
				_, mutationErr = ParseAndVerifyDeviceRevocation(mutated, ed25519.PublicKey(public), now)
			case "access_grant_revocation":
				_, mutationErr = ParseAndVerifyAccessGrantRevocation(mutated, ed25519.PublicKey(public), now, knownGrant)
			case "delegation_revocation":
				_, mutationErr = ParseAndVerifyDelegationRevocation(mutated, now)
			}
			require.Error(t, mutationErr, "bit flip must fail closed")
			unknown := append(append([]byte(nil), wire...), 0x98, 0x06, 0x01)
			for name, candidate := range map[string][]byte{"truncated": wire[:len(wire)-1], "unknown": unknown} {
				require.Error(t, verifyMalformedVector(item.Kind, candidate, ed25519.PublicKey(public), now, knownGrant), name+" input must fail closed")
			}
			if item.Kind == "access_grant" || item.Kind == "device_revocation" || item.Kind == "access_grant_revocation" {
				wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x99}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
				require.Error(t, verifyMalformedVector(item.Kind, wire, wrong, now, knownGrant), "wrong signer key must fail")
			}
			jsonProjection, jsonErr := json.Marshal(artifact)
			require.NoError(t, jsonErr)
			for _, projection := range []string{artifact.String(), fmt.Sprintf("%#v", artifact), string(jsonProjection)} {
				require.NotContains(t, strings.ToLower(projection), "signature")
				require.NotContains(t, projection, item.WireBase64)
				require.NotContains(t, projection, item.SignerPublicKeyBase64)
			}
		})
	}
}

func verifyMalformedVector(kind string, wire []byte, public ed25519.PublicKey, now time.Time, knownGrant *Artifact) error {
	var err error
	switch kind {
	case "key_credential":
		_, err = ParseAndVerifyKeyCredential(wire, now)
	case "access_grant":
		_, err = ParseAndVerifyAccessGrant(wire, public, now)
	case "delegation":
		_, err = ParseAndVerifyDelegation(wire, now)
	case "device_revocation":
		_, err = ParseAndVerifyDeviceRevocation(wire, public, now)
	case "access_grant_revocation":
		_, err = ParseAndVerifyAccessGrantRevocation(wire, public, now, knownGrant)
	case "delegation_revocation":
		_, err = ParseAndVerifyDelegationRevocation(wire, now)
	}
	return err
}
