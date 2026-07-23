package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPersistedSignedGrantRejectsObsoleteOrArbitraryOwnerAfterReopen(t *testing.T) {
	now := time.Date(2035, 1, 2, 3, 4, 5, 0, time.UTC)
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x91}, ed25519.SeedSize))
	subjectKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x92}, ed25519.SeedSize))
	node, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	subject, err := identityprincipal.FromEd25519PublicKey(subjectKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	audience := Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}
	owner, err := PrincipalOwner(subject)
	require.NoError(t, err)
	resource, err := NewResourceRef(node.String(), owner, "owned-content", "content-reference")
	require.NoError(t, err)

	for _, invalidOwner := range []string{strings.Replace(subject.String(), "p1_", "p_", 1), "workload_1"} {
		t.Run(invalidOwner, func(t *testing.T) {
			payload := &identityprotocol.AccessGrantPayload{
				Version: 1, Issuer: node.String(), Subject: subject.String(),
				Audience: &identityprotocol.Audience{Node: audience.Node, Interface: audience.Interface, ProtocolMajor: audience.ProtocolMajor},
				Actions:  []string{"application.content.get"},
				Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{
					PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: invalidOwner},
				}},
				NotBefore: timestamppb.New(now.Add(-time.Minute)), NotAfter: timestamppb.New(now.Add(time.Hour)),
			}
			payloadRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
			require.NoError(t, err)
			signed := append(append([]byte(nil), grantDomain...), payloadRaw...)
			id := artifactID("ag1_", signed)
			envelope := &identityprotocol.AccessGrant{Id: id, Payload: payload, Signature: ed25519.Sign(nodeKey, signed)}
			raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
			require.NoError(t, err)
			_, err = ParseAndVerifyAccessGrant(raw, nodeKey.Public().(ed25519.PublicKey), now)
			require.Error(t, err)

			ctx := context.Background()
			dir := t.TempDir()
			database, err := storage.OpenIdentityAccess(ctx, dir, StorageSchema())
			require.NoError(t, err)
			index, err := grantIndexKey(payload)
			require.NoError(t, err)
			sum := sha256.Sum256(raw)
			record := append(append([]byte(nil), nodeKey.Public().(ed25519.PublicKey)...), raw...)
			require.NoError(t, database.Update(ctx, func(tx storage.WriteTransaction) error {
				if err := tx.Put(grantsBucket, []byte(id), record); err != nil {
					return err
				}
				return tx.Put(grantIndexBucket, index, sum[:])
			}))
			require.NoError(t, database.Close(ctx))

			reopened, err := storage.OpenIdentityAccess(ctx, dir, StorageSchema())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reopened.Close(context.Background())) })
			matched, err := (grantRepository{database: reopened}).matches(ctx, now, subject.String(), audience, "application.content.get", resource)
			require.False(t, matched)
			require.EqualError(t, err, "Access Grant record is corrupt")
			require.NotContains(t, err.Error(), invalidOwner)
		})
	}
}
