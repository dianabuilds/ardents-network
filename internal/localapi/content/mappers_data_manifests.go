package content

import (
	appdata "ardents/internal/content"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func toManifestSnapshot(in appdata.Manifest) *ardentsv1.ManifestSnapshot {
	out := &ardentsv1.ManifestSnapshot{
		Id:        in.ID,
		Kind:      in.Kind,
		Owner:     in.Owner.String(),
		Access:    in.Access,
		Retention: in.Retention,
		Encrypted: in.Encrypted,
		Metadata:  rpc.Struct(in.Metadata),
		CreatedAt: rpc.Timestamp(in.CreatedAt),
	}
	for _, item := range in.Refs {
		out.Refs = append(out.Refs, &ardentsv1.RefSnapshot{Kind: item.Kind, Id: item.ID})
	}
	return out
}

func fromManifestSnapshot(in *ardentsv1.ManifestSnapshot) appdata.Manifest {
	if in == nil {
		return appdata.Manifest{}
	}
	out := appdata.Manifest{
		ID:        in.GetId(),
		Kind:      in.GetKind(),
		Access:    in.GetAccess(),
		Retention: in.GetRetention(),
		Encrypted: in.GetEncrypted(),
		Metadata:  rpc.Map(in.GetMetadata()),
		CreatedAt: rpc.Time(in.GetCreatedAt()),
	}
	for _, item := range in.GetRefs() {
		out.Refs = append(out.Refs, appdata.Ref{Kind: item.GetKind(), ID: item.GetId()})
	}
	return out
}
