package content

import (
	"context"

	"ardents/internal/cli/client"
	ardentsv1 "ardents/internal/localapi/protocol"
)

func (a *Command) dataManifests(ctx context.Context, args []string) int {
	resource := catalogResource[*ardentsv1.ManifestSnapshot, *ardentsv1.ListManifestsResponse]{
		command:        a,
		singular:       "manifest",
		plural:         "manifests",
		attribute:      "kind",
		newSnapshot:    func() *ardentsv1.ManifestSnapshot { return &ardentsv1.ManifestSnapshot{} },
		attributeValue: func(item *ardentsv1.ManifestSnapshot) string { return item.GetKind() },
		list: func(callCtx context.Context) (*ardentsv1.ListManifestsResponse, error) {
			return connectMessage(a.ctx.Client.Service().ListManifests(callCtx, client.Request(a.ctx.Token, &ardentsv1.ListManifestsRequest{})))
		},
		items: func(response *ardentsv1.ListManifestsResponse) []*ardentsv1.ManifestSnapshot {
			return response.GetManifests()
		},
		get: func(callCtx context.Context, id string) (*ardentsv1.ManifestSnapshot, error) {
			return connectMessage(a.ctx.Client.Service().GetManifest(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetManifestRequest{Id: id})))
		},
		publish: func(callCtx context.Context, item *ardentsv1.ManifestSnapshot) (*ardentsv1.ManifestSnapshot, error) {
			return connectMessage(a.ctx.Client.Service().PublishManifest(callCtx, client.Request(a.ctx.Token, &ardentsv1.PublishManifestRequest{Manifest: item})))
		},
	}
	return resource.run(ctx, args)
}
