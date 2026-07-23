package content

import (
	appdata "ardents/internal/content"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func toObjectSnapshot(in appdata.Object) *ardentsv1.ObjectSnapshot {
	out := &ardentsv1.ObjectSnapshot{Id: in.ID, Type: in.Type, Owner: in.Owner.String(), Body: rpc.Struct(in.Body), CreatedAt: rpc.Timestamp(in.CreatedAt)}
	for _, item := range in.BlobRefs {
		out.BlobRefs = append(out.BlobRefs, &ardentsv1.RefSnapshot{Kind: item.Kind, Id: item.ID})
	}
	return out
}

func fromObjectSnapshot(in *ardentsv1.ObjectSnapshot) appdata.Object {
	if in == nil {
		return appdata.Object{}
	}
	out := appdata.Object{ID: in.GetId(), Type: in.GetType(), Body: rpc.Map(in.GetBody()), CreatedAt: rpc.Time(in.GetCreatedAt())}
	for _, item := range in.GetBlobRefs() {
		out.BlobRefs = append(out.BlobRefs, appdata.Ref{Kind: item.GetKind(), ID: item.GetId()})
	}
	return out
}
