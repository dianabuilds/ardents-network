package transfer

import (
	"testing"

	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
)

func TestCanonicalizeResourceUsesExactTransferTargets(t *testing.T) {
	tests := []struct {
		procedure string
		message   any
		kind      identityaccess.ResourceKind
		id        string
	}{
		{ardentsv1connect.TransferServiceFetchBlobProcedure, &protocol.FetchBlobRequest{Id: "blob-1"}, "content-blob", "blob-1"},
		{ardentsv1connect.TransferServiceListBlobSourcesProcedure, &protocol.ListBlobSourcesRequest{Id: "blob-1"}, "content-blob", "blob-1"},
		{ardentsv1connect.TransferServiceGetTransferProcedure, &protocol.GetTransferRequest{Id: "transfer-1"}, "transfer", "transfer-1"},
		{ardentsv1connect.TransferServiceListTransfersProcedure, &protocol.ListTransfersRequest{}, "transfer-collection", ""},
	}
	for _, test := range tests {
		target, err := CanonicalizeResource(test.procedure, test.message, test.kind)
		require.NoError(t, err)
		require.Equal(t, test.kind, target.Kind)
		require.Equal(t, test.id, target.ID)
	}
}

func TestCanonicalizeResourceRejectsMalformedTransfer(t *testing.T) {
	for _, test := range []struct {
		procedure string
		message   any
	}{
		{ardentsv1connect.TransferServiceFetchBlobProcedure, &protocol.FetchBlobRequest{}},
		{ardentsv1connect.TransferServiceListBlobSourcesProcedure, &protocol.ListBlobSourcesRequest{Id: " blob"}},
		{ardentsv1connect.TransferServiceGetTransferProcedure, &protocol.GetTransferRequest{}},
		{ardentsv1connect.TransferServiceListTransfersProcedure, &protocol.ListBlobSourcesRequest{}},
		{"/ardents.v1.TransferService/Unknown", &protocol.ListTransfersRequest{}},
	} {
		_, err := CanonicalizeResource(test.procedure, test.message, "transfer")
		require.ErrorIs(t, err, ErrInvalidResourceTarget)
	}
}
