package content

import (
	"testing"

	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCanonicalizeResourceUsesExactContentTargets(t *testing.T) {
	tests := []struct {
		procedure string
		message   any
		kind      identityaccess.ResourceKind
		id        string
	}{
		{ardentsv1connect.ContentServicePublishObjectProcedure, &protocol.PublishObjectRequest{Object: &protocol.ObjectSnapshot{Id: "obj-1", Owner: "claimed-owner"}}, "content-object", "obj-1"},
		{ardentsv1connect.ContentServiceGetObjectProcedure, &protocol.GetObjectRequest{Id: "obj-1"}, "content-object", "obj-1"},
		{ardentsv1connect.ContentServiceListObjectsProcedure, &protocol.ListObjectsRequest{}, "content-object-collection", ""},
		{ardentsv1connect.ContentServiceGetBlobProcedure, &protocol.GetBlobRequest{Id: "blob-1"}, "content-blob", "blob-1"},
		{ardentsv1connect.ContentServiceListBlobsProcedure, &protocol.ListBlobsRequest{}, "content-blob-collection", ""},
		{ardentsv1connect.ContentServicePublishManifestProcedure, &protocol.PublishManifestRequest{Manifest: &protocol.ManifestSnapshot{Id: "manifest-1", Owner: "claimed-owner"}}, "content-manifest", "manifest-1"},
		{ardentsv1connect.ContentServiceGetManifestProcedure, &protocol.GetManifestRequest{Id: "manifest-1"}, "content-manifest", "manifest-1"},
		{ardentsv1connect.ContentServiceListManifestsProcedure, &protocol.ListManifestsRequest{}, "content-manifest-collection", ""},
		{ardentsv1connect.ContentServiceGetDataInventoryProcedure, &protocol.GetDataInventoryRequest{}, "content-inventory", ""},
		{ardentsv1connect.RetentionServiceRetainBlobProcedure, &protocol.RetainBlobRequest{Id: "blob-1", ExpiresAt: timestamppb.Now()}, "content-blob", "blob-1"},
		{ardentsv1connect.RetentionServicePinBlobProcedure, &protocol.PinBlobRequest{Id: "blob-1"}, "content-blob", "blob-1"},
		{ardentsv1connect.RetentionServiceDropBlobProcedure, &protocol.DropBlobRequest{Id: "blob-1"}, "content-blob", "blob-1"},
	}
	for _, test := range tests {
		target, err := CanonicalizeResource(test.procedure, test.message, test.kind)
		require.NoError(t, err, test.procedure)
		require.Equal(t, test.kind, target.Kind)
		require.Equal(t, test.id, target.ID)
	}
}

func TestCanonicalizePublishBlobDerivesPayloadTarget(t *testing.T) {
	request := &protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{Payload: []byte("hello")}}
	target, err := CanonicalizeResource(ardentsv1connect.ContentServicePublishBlobProcedure, request, "content-blob")
	require.NoError(t, err)
	require.NotEmpty(t, target.ID)
	request.Blob.Id = target.ID
	request.Blob.Cid = target.ID
	again, err := CanonicalizeResource(ardentsv1connect.ContentServicePublishBlobProcedure, request, "content-blob")
	require.NoError(t, err)
	require.Equal(t, target, again)
}

func TestCanonicalizeResourceRejectsMalformedContentBeforeMutation(t *testing.T) {
	tests := []struct {
		procedure string
		message   any
	}{
		{ardentsv1connect.ContentServicePublishObjectProcedure, &protocol.PublishObjectRequest{}},
		{ardentsv1connect.ContentServicePublishObjectProcedure, &protocol.PublishObjectRequest{Object: &protocol.ObjectSnapshot{}}},
		{ardentsv1connect.ContentServicePublishBlobProcedure, &protocol.PublishBlobRequest{}},
		{ardentsv1connect.ContentServicePublishBlobProcedure, &protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{}}},
		{ardentsv1connect.ContentServicePublishManifestProcedure, &protocol.PublishManifestRequest{Manifest: &protocol.ManifestSnapshot{}}},
		{ardentsv1connect.ContentServiceGetObjectProcedure, &protocol.GetObjectRequest{Id: " obj"}},
		{ardentsv1connect.RetentionServiceRetainBlobProcedure, &protocol.RetainBlobRequest{Id: "blob", ExpiresAt: &timestamppb.Timestamp{Seconds: 253402300800}}},
		{ardentsv1connect.RetentionServicePinBlobProcedure, &protocol.PinBlobRequest{}},
		{ardentsv1connect.ContentServiceListObjectsProcedure, &protocol.ListBlobsRequest{}},
		{"/ardents.v1.ContentService/Unknown", &protocol.ListObjectsRequest{}},
	}
	for _, test := range tests {
		_, err := CanonicalizeResource(test.procedure, test.message, "content-object")
		require.ErrorIs(t, err, ErrInvalidResourceTarget)
	}
}
