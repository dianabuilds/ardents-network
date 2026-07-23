package content

import (
	"errors"

	contentapi "ardents/internal/content"
	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrInvalidResourceTarget = errors.New("content resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind identityaccess.ResourceKind) (identityaccess.ResourceTarget, error) {
	target := identityaccess.ResourceTarget{Kind: kind}
	var id string
	var err error
	valid := true
	switch procedure {
	case ardentsv1connect.ContentServicePublishObjectProcedure:
		request, ok := message.(*protocol.PublishObjectRequest)
		if !ok || request.GetObject() == nil {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetObject().GetId())
	case ardentsv1connect.ContentServiceGetObjectProcedure:
		request, ok := message.(*protocol.GetObjectRequest)
		if !ok {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	case ardentsv1connect.ContentServiceListObjectsProcedure:
		_, valid = message.(*protocol.ListObjectsRequest)
	case ardentsv1connect.ContentServicePublishBlobProcedure:
		request, ok := message.(*protocol.PublishBlobRequest)
		if !ok || request.GetBlob() == nil {
			valid = false
			break
		}
		var command contentapi.PublishBlobCommand
		command, err = fromBlobSnapshot(request.GetBlob())
		if err == nil {
			id, err = contentapi.PublishBlobAccessResourceID(command)
		}
	case ardentsv1connect.ContentServiceGetBlobProcedure:
		request, ok := message.(*protocol.GetBlobRequest)
		if !ok {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	case ardentsv1connect.ContentServiceListBlobsProcedure:
		_, valid = message.(*protocol.ListBlobsRequest)
	case ardentsv1connect.ContentServicePublishManifestProcedure:
		request, ok := message.(*protocol.PublishManifestRequest)
		if !ok || request.GetManifest() == nil {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetManifest().GetId())
	case ardentsv1connect.ContentServiceGetManifestProcedure:
		request, ok := message.(*protocol.GetManifestRequest)
		if !ok {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	case ardentsv1connect.ContentServiceListManifestsProcedure:
		_, valid = message.(*protocol.ListManifestsRequest)
	case ardentsv1connect.ContentServiceGetDataInventoryProcedure:
		_, valid = message.(*protocol.GetDataInventoryRequest)
	case ardentsv1connect.RetentionServiceRetainBlobProcedure:
		request, ok := message.(*protocol.RetainBlobRequest)
		if !ok || request.GetExpiresAt() != nil && request.GetExpiresAt().CheckValid() != nil {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	case ardentsv1connect.RetentionServicePinBlobProcedure:
		request, ok := message.(*protocol.PinBlobRequest)
		if !ok {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	case ardentsv1connect.RetentionServiceDropBlobProcedure:
		request, ok := message.(*protocol.DropBlobRequest)
		if !ok {
			valid = false
			break
		}
		id, err = contentapi.AccessResourceID(request.GetId())
	default:
		valid = false
	}
	if !valid || err != nil {
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
	target.ID = id
	return target, nil
}
