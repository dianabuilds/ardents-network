package content

import (
	"ardents/internal/localapi/rpc"
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

func (h *QueryHandler) PublishObject(_ context.Context, req *connect.Request[ardentsv1.PublishObjectRequest]) (*connect.Response[ardentsv1.ObjectSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ObjectSnapshot, *rpc.Error) {
		if err := rpc.RequireWrite(call, "data", "data.publish_object"); err != nil {
			return nil, err
		}
		res, err := h.commands.PublishObject(fromObjectSnapshot(req.Msg.GetObject()))
		if err != nil {
			return nil, rpc.MapError("data", "data.publish_object", "publish_failed", "data publish object failed", false, err)
		}
		return toObjectSnapshot(res), nil
	})
}

func (h *QueryHandler) GetObject(_ context.Context, req *connect.Request[ardentsv1.GetObjectRequest]) (*connect.Response[ardentsv1.ObjectSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ObjectSnapshot, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.get_object"); err != nil {
			return nil, err
		}
		res, ok := h.content.GetObject(req.Msg.GetId())
		if !ok {
			return nil, rpc.NotFound("data", "data.get_object", "data object not found")
		}
		return toObjectSnapshot(res), nil
	})
}

func (h *QueryHandler) ListObjects(_ context.Context, req *connect.Request[ardentsv1.ListObjectsRequest]) (*connect.Response[ardentsv1.ListObjectsResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListObjectsResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.list_objects"); err != nil {
			return nil, err
		}
		items := h.content.ListObjects()
		out := make([]*ardentsv1.ObjectSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toObjectSnapshot(item))
		}
		return &ardentsv1.ListObjectsResponse{Objects: out}, nil
	})
}
