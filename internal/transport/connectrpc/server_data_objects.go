package connectrpc

import (
	"context"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) PublishObject(ctx context.Context, req *connect.Request[ardentsv1.PublishObjectRequest]) (*connect.Response[ardentsv1.ObjectSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ObjectSnapshot, *rpcError) {
		if err := requireWrite(call, "data", "data.publish_object"); err != nil {
			return nil, err
		}
		res, err := s.data.PublishObject(fromObjectSnapshot(req.Msg.GetObject()))
		if err != nil {
			return nil, mapAPIError("data", "data.publish_object", "publish_failed", "data publish object failed", false, err)
		}
		return toObjectSnapshot(res), nil
	})
}

func (s *Server) GetObject(ctx context.Context, req *connect.Request[ardentsv1.GetObjectRequest]) (*connect.Response[ardentsv1.ObjectSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ObjectSnapshot, *rpcError) {
		if err := requireRead(call, "data", "data.get_object"); err != nil {
			return nil, err
		}
		res, err := s.data.GetObject(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("data", "data.get_object", "failed", "data get object failed", false, err)
		}
		return toObjectSnapshot(res), nil
	})
}

func (s *Server) ListObjects(ctx context.Context, req *connect.Request[ardentsv1.ListObjectsRequest]) (*connect.Response[ardentsv1.ListObjectsResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListObjectsResponse, *rpcError) {
		if err := requireRead(call, "data", "data.list_objects"); err != nil {
			return nil, err
		}
		items, err := s.data.ListObjects()
		if err != nil {
			return nil, mapAPIError("data", "data.list_objects", "failed", "data list objects failed", false, err)
		}
		out := make([]*ardentsv1.ObjectSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toObjectSnapshot(item))
		}
		return &ardentsv1.ListObjectsResponse{Objects: out}, nil
	})
}

func (s *Server) GetDataInventory(ctx context.Context, req *connect.Request[ardentsv1.GetDataInventoryRequest]) (*connect.Response[ardentsv1.DataInventorySnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.DataInventorySnapshot, *rpcError) {
		if err := requireRead(call, "data", "data.inventory"); err != nil {
			return nil, err
		}
		return toDataInventorySnapshot(s.data.DataInventory()), nil
	})
}
