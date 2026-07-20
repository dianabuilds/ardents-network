package connectrpc

import (
	"context"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) PublishManifest(ctx context.Context, req *connect.Request[ardentsv1.PublishManifestRequest]) (*connect.Response[ardentsv1.ManifestSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ManifestSnapshot, *rpcError) {
		if err := requireWrite(call, "data", "data.publish_manifest"); err != nil {
			return nil, err
		}
		res, err := s.data.PublishManifest(fromManifestSnapshot(req.Msg.GetManifest()))
		if err != nil {
			return nil, mapAPIError("data", "data.publish_manifest", "publish_failed", "data publish manifest failed", false, err)
		}
		return toManifestSnapshot(res), nil
	})
}

func (s *Server) GetManifest(ctx context.Context, req *connect.Request[ardentsv1.GetManifestRequest]) (*connect.Response[ardentsv1.ManifestSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ManifestSnapshot, *rpcError) {
		if err := requireRead(call, "data", "data.get_manifest"); err != nil {
			return nil, err
		}
		res, err := s.data.GetManifest(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("data", "data.get_manifest", "failed", "data get manifest failed", false, err)
		}
		return toManifestSnapshot(res), nil
	})
}

func (s *Server) ListManifests(ctx context.Context, req *connect.Request[ardentsv1.ListManifestsRequest]) (*connect.Response[ardentsv1.ListManifestsResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListManifestsResponse, *rpcError) {
		if err := requireRead(call, "data", "data.list_manifests"); err != nil {
			return nil, err
		}
		items, err := s.data.ListManifests()
		if err != nil {
			return nil, mapAPIError("data", "data.list_manifests", "failed", "data list manifests failed", false, err)
		}
		out := make([]*ardentsv1.ManifestSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toManifestSnapshot(item))
		}
		return &ardentsv1.ListManifestsResponse{Manifests: out}, nil
	})
}
