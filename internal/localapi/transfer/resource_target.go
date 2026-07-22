package transfer

import (
	"errors"

	contentapi "ardents/internal/content"
	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	transferapi "ardents/internal/transfer"
)

var ErrInvalidResourceTarget = errors.New("transfer resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind identityaccess.ResourceKind) (identityaccess.ResourceTarget, error) {
	target := identityaccess.ResourceTarget{Kind: kind}
	var id string
	var err error
	valid := true
	switch procedure {
	case ardentsv1connect.TransferServiceFetchBlobProcedure:
		request, ok := message.(*protocol.FetchBlobRequest)
		if !ok {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	case ardentsv1connect.TransferServiceListBlobSourcesProcedure:
		request, ok := message.(*protocol.ListBlobSourcesRequest)
		if !ok {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	case ardentsv1connect.TransferServiceGetTransferProcedure:
		request, ok := message.(*protocol.GetTransferRequest)
		if !ok {
			valid = false
			break
		}
		id, err = transferapi.AccessResourceID(request.GetId())
	case ardentsv1connect.TransferServiceListTransfersProcedure:
		_, valid = message.(*protocol.ListTransfersRequest)
	default:
		valid = false
	}
	if !valid || err != nil {
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
	target.ID = id
	return target, nil
}
