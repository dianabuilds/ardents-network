package connectrpc

import (
	"context"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) PublishBlob(ctx context.Context, req *connect.Request[ardentsv1.PublishBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.BlobSnapshot, *rpcError) {
		if err := requireWrite(call, "data", "data.publish_blob"); err != nil {
			return nil, err
		}
		res, err := s.data.PublishBlob(fromBlobSnapshot(req.Msg.GetBlob()))
		if err != nil {
			return nil, mapAPIError("data", "data.publish_blob", "publish_failed", "data publish blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (s *Server) FetchBlob(ctx context.Context, req *connect.Request[ardentsv1.FetchBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.BlobSnapshot, *rpcError) {
		if err := requireWrite(call, "data", "data.fetch_blob"); err != nil {
			return nil, err
		}
		res, err := s.data.FetchBlob(ctx, req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("data", "data.fetch_blob", "fetch_failed", "data fetch blob failed", true, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (s *Server) GetBlob(ctx context.Context, req *connect.Request[ardentsv1.GetBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.BlobSnapshot, *rpcError) {
		if err := requireRead(call, "data", "data.get_blob"); err != nil {
			return nil, err
		}
		res, err := s.data.GetBlob(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("data", "data.get_blob", "failed", "data get blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (s *Server) ListBlobs(ctx context.Context, req *connect.Request[ardentsv1.ListBlobsRequest]) (*connect.Response[ardentsv1.ListBlobsResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListBlobsResponse, *rpcError) {
		if err := requireRead(call, "data", "data.list_blobs"); err != nil {
			return nil, err
		}
		items, err := s.data.ListBlobs()
		if err != nil {
			return nil, mapAPIError("data", "data.list_blobs", "failed", "data list blobs failed", false, err)
		}
		out := make([]*ardentsv1.BlobSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toBlobSnapshot(item))
		}
		return &ardentsv1.ListBlobsResponse{Blobs: out}, nil
	})
}

func (s *Server) ListBlobSources(ctx context.Context, req *connect.Request[ardentsv1.ListBlobSourcesRequest]) (*connect.Response[ardentsv1.ListBlobSourcesResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListBlobSourcesResponse, *rpcError) {
		if err := requireRead(call, "data", "data.blob_sources"); err != nil {
			return nil, err
		}
		return &ardentsv1.ListBlobSourcesResponse{Sources: toBlobSourceSnapshots(s.data.ListBlobSources(req.Msg.GetId()))}, nil
	})
}

func (s *Server) RetainBlob(ctx context.Context, req *connect.Request[ardentsv1.RetainBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.BlobSnapshot, *rpcError) {
		if err := requireWrite(call, "data", "data.retain_blob"); err != nil {
			return nil, err
		}
		res, err := s.data.RetainBlob(req.Msg.GetId(), fromTS(req.Msg.GetExpiresAt()))
		if err != nil {
			return nil, mapAPIError("data", "data.retain_blob", "retain_failed", "data retain blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (s *Server) PinBlob(ctx context.Context, req *connect.Request[ardentsv1.PinBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.BlobSnapshot, *rpcError) {
		if err := requireWrite(call, "data", "data.pin_blob"); err != nil {
			return nil, err
		}
		res, err := s.data.PinBlob(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("data", "data.pin_blob", "pin_failed", "data pin blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}

func (s *Server) DropBlob(ctx context.Context, req *connect.Request[ardentsv1.DropBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.BlobSnapshot, *rpcError) {
		if err := requireWrite(call, "data", "data.drop_blob"); err != nil {
			return nil, err
		}
		res, err := s.data.DropBlob(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("data", "data.drop_blob", "drop_failed", "data drop blob failed", false, err)
		}
		return toBlobSnapshot(res), nil
	})
}
