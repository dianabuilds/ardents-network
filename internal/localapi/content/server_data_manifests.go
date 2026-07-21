package content

import (
	"ardents/internal/localapi/rpc"
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

func (h *QueryHandler) PublishManifest(_ context.Context, req *connect.Request[ardentsv1.PublishManifestRequest]) (*connect.Response[ardentsv1.ManifestSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ManifestSnapshot, *rpc.Error) {
		if err := rpc.RequireWrite(call, "data", "data.publish_manifest"); err != nil {
			return nil, err
		}
		res, err := h.commands.PublishManifest(fromManifestSnapshot(req.Msg.GetManifest()))
		if err != nil {
			return nil, rpc.MapError("data", "data.publish_manifest", "publish_failed", "data publish manifest failed", false, err)
		}
		return toManifestSnapshot(res), nil
	})
}

func (h *QueryHandler) GetManifest(_ context.Context, req *connect.Request[ardentsv1.GetManifestRequest]) (*connect.Response[ardentsv1.ManifestSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ManifestSnapshot, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.get_manifest"); err != nil {
			return nil, err
		}
		res, ok := h.content.GetManifest(req.Msg.GetId())
		if !ok {
			return nil, rpc.NotFound("data", "data.get_manifest", "data manifest not found")
		}
		return toManifestSnapshot(res), nil
	})
}

func (h *QueryHandler) ListManifests(_ context.Context, req *connect.Request[ardentsv1.ListManifestsRequest]) (*connect.Response[ardentsv1.ListManifestsResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListManifestsResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.list_manifests"); err != nil {
			return nil, err
		}
		items := h.content.ListManifests()
		out := make([]*ardentsv1.ManifestSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toManifestSnapshot(item))
		}
		return &ardentsv1.ListManifestsResponse{Manifests: out}, nil
	})
}
