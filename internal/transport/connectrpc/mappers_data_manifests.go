package connectrpc

import (
	dataapi "ardents/internal/data/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toManifestSnapshot(in dataapi.ManifestSnapshot) *ardentsv1.ManifestSnapshot {
	out := &ardentsv1.ManifestSnapshot{
		Id:        in.ID,
		Kind:      in.Kind,
		Owner:     in.Owner,
		Access:    in.Access,
		Retention: in.Retention,
		Encrypted: in.Encrypted,
		Metadata:  toStruct(in.Metadata),
		CreatedAt: ts(in.CreatedAt),
	}
	for _, item := range in.Refs {
		out.Refs = append(out.Refs, &ardentsv1.RefSnapshot{Kind: item.Kind, Id: item.ID})
	}
	return out
}

func fromManifestSnapshot(in *ardentsv1.ManifestSnapshot) dataapi.ManifestSnapshot {
	if in == nil {
		return dataapi.ManifestSnapshot{}
	}
	out := dataapi.ManifestSnapshot{
		ID:        in.GetId(),
		Kind:      in.GetKind(),
		Owner:     in.GetOwner(),
		Access:    in.GetAccess(),
		Retention: in.GetRetention(),
		Encrypted: in.GetEncrypted(),
		Metadata:  fromStruct(in.GetMetadata()),
		CreatedAt: fromTS(in.GetCreatedAt()),
	}
	for _, item := range in.GetRefs() {
		out.Refs = append(out.Refs, dataapi.RefSnapshot{Kind: item.GetKind(), ID: item.GetId()})
	}
	return out
}
