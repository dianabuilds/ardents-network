package content

import (
	"ardents/internal/localapi/rpc"
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

func (h *QueryHandler) PublishBlob(_ context.Context, req *connect.Request[ardentsv1.PublishBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		if err := rpc.RequireWrite(call, "data", "data.publish_blob"); err != nil {
			return nil, err
		}
		res, err := h.commands.PublishBlob(fromBlobSnapshot(req.Msg.GetBlob()))
		if err != nil {
			return nil, rpc.MapError("data", "data.publish_blob", "publish_failed", "data publish blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) GetBlob(_ context.Context, req *connect.Request[ardentsv1.GetBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.get_blob"); err != nil {
			return nil, err
		}
		res, ok := h.content.GetBlob(req.Msg.GetId())
		if !ok {
			return nil, rpc.NotFound("data", "data.get_blob", "data blob not found")
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) ListBlobs(_ context.Context, req *connect.Request[ardentsv1.ListBlobsRequest]) (*connect.Response[ardentsv1.ListBlobsResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListBlobsResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.list_blobs"); err != nil {
			return nil, err
		}
		items := h.content.ListBlobs()
		out := make([]*ardentsv1.BlobSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toBlobSnapshot(item))
		}
		return &ardentsv1.ListBlobsResponse{Blobs: out}, nil
	})
}

func (h *QueryHandler) RetainBlob(_ context.Context, req *connect.Request[ardentsv1.RetainBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		if err := rpc.RequireWrite(call, "data", "data.retain_blob"); err != nil {
			return nil, err
		}
		res, err := h.commands.RetainBlob(req.Msg.GetId(), rpc.Time(req.Msg.GetExpiresAt()))
		if err != nil {
			return nil, rpc.MapError("data", "data.retain_blob", "retain_failed", "data retain blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) PinBlob(_ context.Context, req *connect.Request[ardentsv1.PinBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		if err := rpc.RequireWrite(call, "data", "data.pin_blob"); err != nil {
			return nil, err
		}
		res, err := h.commands.PinBlob(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("data", "data.pin_blob", "pin_failed", "data pin blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (h *QueryHandler) DropBlob(_ context.Context, req *connect.Request[ardentsv1.DropBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		if err := rpc.RequireWrite(call, "data", "data.drop_blob"); err != nil {
			return nil, err
		}
		res, err := h.commands.DropBlob(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("data", "data.drop_blob", "drop_failed", "data drop blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}
