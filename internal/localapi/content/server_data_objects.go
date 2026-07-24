package content

import (
	"ardents/internal/localapi/rpc"
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

func (h *QueryHandler) PublishObject(ctx context.Context, req *connect.Request[ardentsv1.PublishObjectRequest]) (*connect.Response[ardentsv1.ObjectSnapshot], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*ardentsv1.ObjectSnapshot, *rpc.Error) {
		owner, ownerErr := admittedOwner(call)
		if ownerErr != nil {
			return nil, ownerErr
		}
		object := fromObjectSnapshot(req.Msg.GetObject())
		object.Owner = owner
		res, err := h.commands.PublishObject(object)
		if err != nil {
			return nil, rpc.MapError("data", "data.publish_object", "publish_failed", "data publish object failed", false, err)
		}
		return toObjectSnapshot(res), nil
	})
}

func (h *QueryHandler) GetObject(ctx context.Context, req *connect.Request[ardentsv1.GetObjectRequest]) (*connect.Response[ardentsv1.ObjectSnapshot], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*ardentsv1.ObjectSnapshot, *rpc.Error) {
		owner, ownerErr := admittedOwner(call)
		if ownerErr != nil {
			return nil, ownerErr
		}
		res, ok := h.content.GetObject(owner, req.Msg.GetId())
		if !ok {
			return nil, rpc.NotFound("data", "data.get_object", "data object not found")
		}
		return toObjectSnapshot(res), nil
	})
}

func (h *QueryHandler) ListObjects(ctx context.Context, _ *connect.Request[ardentsv1.ListObjectsRequest]) (*connect.Response[ardentsv1.ListObjectsResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*ardentsv1.ListObjectsResponse, *rpc.Error) {
		owner, ownerErr := admittedOwner(call)
		if ownerErr != nil {
			return nil, ownerErr
		}
		items := h.content.ListObjects(owner)
		out := make([]*ardentsv1.ObjectSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toObjectSnapshot(item))
		}
		return &ardentsv1.ListObjectsResponse{Objects: out}, nil
	})
}
