package connectrpc

import (
	dataapi "ardents/internal/data/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toObjectSnapshot(in dataapi.ObjectSnapshot) *ardentsv1.ObjectSnapshot {
	out := &ardentsv1.ObjectSnapshot{Id: in.ID, Type: in.Type, Owner: in.Owner, Body: toStruct(in.Body), CreatedAt: ts(in.CreatedAt)}
	for _, item := range in.BlobRefs {
		out.BlobRefs = append(out.BlobRefs, &ardentsv1.RefSnapshot{Kind: item.Kind, Id: item.ID})
	}
	return out
}

func fromObjectSnapshot(in *ardentsv1.ObjectSnapshot) dataapi.ObjectSnapshot {
	if in == nil {
		return dataapi.ObjectSnapshot{}
	}
	out := dataapi.ObjectSnapshot{ID: in.GetId(), Type: in.GetType(), Owner: in.GetOwner(), Body: fromStruct(in.GetBody()), CreatedAt: fromTS(in.GetCreatedAt())}
	for _, item := range in.GetBlobRefs() {
		out.BlobRefs = append(out.BlobRefs, dataapi.RefSnapshot{Kind: item.GetKind(), ID: item.GetId()})
	}
	return out
}
