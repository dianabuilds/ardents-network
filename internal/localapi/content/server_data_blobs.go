package content

import (
	"ardents/internal/localapi/rpc"
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

func (h *QueryHandler) PublishBlob(ctx context.Context, req *connect.Request[ardentsv1.PublishBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		res, err := h.commands.PublishBlob(fromBlobSnapshot(req.Msg.GetBlob()))
		if err != nil {
			return nil, rpc.MapError("data", "data.publish_blob", "publish_failed", "data publish blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) GetBlob(ctx context.Context, req *connect.Request[ardentsv1.GetBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		res, ok := h.content.GetBlob(req.Msg.GetId())
		if !ok {
			return nil, rpc.NotFound("data", "data.get_blob", "data blob not found")
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) ListBlobs(ctx context.Context, _ *connect.Request[ardentsv1.ListBlobsRequest]) (*connect.Response[ardentsv1.ListBlobsResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.ListBlobsResponse, *rpc.Error) {
		items := h.content.ListBlobs()
		out := make([]*ardentsv1.BlobSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toBlobSnapshot(item))
		}
		return &ardentsv1.ListBlobsResponse{Blobs: out}, nil
	})
}

func (h *QueryHandler) RetainBlob(ctx context.Context, req *connect.Request[ardentsv1.RetainBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		res, err := h.commands.RetainBlob(req.Msg.GetId(), rpc.Time(req.Msg.GetExpiresAt()))
		if err != nil {
			return nil, rpc.MapError("data", "data.retain_blob", "retain_failed", "data retain blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) PinBlob(ctx context.Context, req *connect.Request[ardentsv1.PinBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		res, err := h.commands.PinBlob(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("data", "data.pin_blob", "pin_failed", "data pin blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) DropBlob(ctx context.Context, req *connect.Request[ardentsv1.DropBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		res, err := h.commands.DropBlob(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("data", "data.drop_blob", "drop_failed", "data drop blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}
