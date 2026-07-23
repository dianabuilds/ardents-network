package network

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCanonicalizeResourceOwnsNetworkProtocolMapping(t *testing.T) {
	record := resourceTargetSignedNodeRecord(t)
	record.SourceV1 = "local"
	tests := []struct {
		procedure string
		message   any
		kind      string
		nonempty  bool
	}{
		{ardentsv1connect.NetworkServiceGetNetworkStatusProcedure, &protocol.GetNetworkStatusRequest{}, "network", false},
		{ardentsv1connect.NetworkServiceResolveRecordProcedure, &protocol.ResolveRecordRequest{Kind: "node", Subject: record.GetNodeFacts().GetPrincipal()}, "discovery-record", true},
		{ardentsv1connect.NetworkServiceResolveServiceProcedure, &protocol.ResolveServiceRequest{Service: "svc.echo"}, "service", true},
		{ardentsv1connect.NetworkServiceImportRecordProcedure, &protocol.ImportRecordRequest{Record: record}, "discovery-record", true},
	}
	require.Empty(t, fromDiscoveryRecord(record).Source)
	for _, test := range tests {
		target, err := CanonicalizeResource(test.procedure, test.message, identityaccess.ResourceKind(test.kind))
		require.NoError(t, err)
		require.Equal(t, test.kind, string(target.Kind))
		require.Equal(t, test.nonempty, target.ID != "")
	}
	for _, malformed := range []struct {
		procedure string
		message   any
	}{
		{ardentsv1connect.NetworkServiceResolveRecordProcedure, &protocol.ResolveRecordRequest{Kind: "node"}},
		{ardentsv1connect.NetworkServiceResolveServiceProcedure, &protocol.ResolveServiceRequest{Service: "svc echo"}},
		{ardentsv1connect.NetworkServiceImportRecordProcedure, &protocol.ImportRecordRequest{Record: &protocol.DiscoveryRecord{Version: 1}}},
	} {
		_, err := CanonicalizeResource(malformed.procedure, malformed.message, "discovery-record")
		require.ErrorIs(t, err, ErrInvalidResourceTarget)
	}

	withLegacyWireField := resourceTargetSignedNodeRecord(t)
	legacyWire := protowire.AppendTag(nil, 1, protowire.BytesType)
	withLegacyWireField.ProtoReflect().SetUnknown(protowire.AppendString(legacyWire, "legacy-id"))
	_, err := CanonicalizeResource(ardentsv1connect.NetworkServiceImportRecordProcedure, &protocol.ImportRecordRequest{Record: withLegacyWireField}, "discovery-record")
	require.ErrorIs(t, err, ErrInvalidResourceTarget)

	withUnknownFacts := resourceTargetSignedNodeRecord(t)
	unknownFactsWire := protowire.AppendTag(nil, 99, protowire.BytesType)
	withUnknownFacts.GetNodeFacts().ProtoReflect().SetUnknown(protowire.AppendString(unknownFactsWire, "unknown"))
	_, err = CanonicalizeResource(ardentsv1connect.NetworkServiceImportRecordProcedure, &protocol.ImportRecordRequest{Record: withUnknownFacts}, "discovery-record")
	require.ErrorIs(t, err, ErrInvalidResourceTarget)

	withInvalidTimestamp := resourceTargetSignedNodeRecord(t)
	withInvalidTimestamp.IssuedAtV1.Nanos = 1_000_000_000
	_, err = CanonicalizeResource(ardentsv1connect.NetworkServiceImportRecordProcedure, &protocol.ImportRecordRequest{Record: withInvalidTimestamp}, "discovery-record")
	require.ErrorIs(t, err, ErrInvalidResourceTarget)
}

func resourceTargetSignedNodeRecord(t *testing.T) *protocol.DiscoveryRecord {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	record := discoveryrecord.Record{Version: discoveryrecord.Version, Node: &discoveryrecord.NodeFacts{Principal: principal, PublicKey: base64.StdEncoding.EncodeToString(public)}, IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	canonical, err := discoveryrecord.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, canonical))
	return &protocol.DiscoveryRecord{Version: record.Version, Facts: &protocol.DiscoveryRecord_NodeFacts{NodeFacts: &protocol.NodeDiscoveryFacts{Principal: principal.String(), PublicKey: record.Node.PublicKey}}, IssuedAtV1: timestamppb.New(record.IssuedAt), ExpiresAtV1: timestamppb.New(record.ExpiresAt), SignatureV1: record.Signature}
}
