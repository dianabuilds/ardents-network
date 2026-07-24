package content

import (
	"ardents/internal/localapi/rpc"
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

func (h *QueryHandler) PublishManifest(ctx context.Context, req *connect.Request[ardentsv1.PublishManifestRequest]) (*connect.Response[ardentsv1.ManifestSnapshot], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*ardentsv1.ManifestSnapshot, *rpc.Error) {
		owner, ownerErr := admittedOwner(call)
		if ownerErr != nil {
			return nil, ownerErr
		}
		manifest := fromManifestSnapshot(req.Msg.GetManifest())
		manifest.Owner = owner
		res, err := h.commands.PublishManifest(manifest)
		if err != nil {
			return nil, rpc.MapError("data", "data.publish_manifest", "publish_failed", "data publish manifest failed", false, err)
		}
		return toManifestSnapshot(res), nil
	})
}

func (h *QueryHandler) GetManifest(ctx context.Context, req *connect.Request[ardentsv1.GetManifestRequest]) (*connect.Response[ardentsv1.ManifestSnapshot], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*ardentsv1.ManifestSnapshot, *rpc.Error) {
		owner, ownerErr := admittedOwner(call)
		if ownerErr != nil {
			return nil, ownerErr
		}
		res, ok := h.content.GetManifest(owner, req.Msg.GetId())
		if !ok {
			return nil, rpc.NotFound("data", "data.get_manifest", "data manifest not found")
		}
		return toManifestSnapshot(res), nil
	})
}

func (h *QueryHandler) ListManifests(ctx context.Context, _ *connect.Request[ardentsv1.ListManifestsRequest]) (*connect.Response[ardentsv1.ListManifestsResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*ardentsv1.ListManifestsResponse, *rpc.Error) {
		owner, ownerErr := admittedOwner(call)
		if ownerErr != nil {
			return nil, ownerErr
		}
		items := h.content.ListManifests(owner)
		out := make([]*ardentsv1.ManifestSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toManifestSnapshot(item))
		}
		return &ardentsv1.ListManifestsResponse{Manifests: out}, nil
	})
}
