package content

import (
	"context"

	"ardents/internal/cli/client"
	ardentsv1 "ardents/internal/localapi/protocol"
)

func (a *Command) dataObjects(ctx context.Context, args []string) int {
	resource := catalogResource[*ardentsv1.ObjectSnapshot, *ardentsv1.ListObjectsResponse]{
		command:        a,
		singular:       "object",
		plural:         "objects",
		attribute:      "type",
		newSnapshot:    func() *ardentsv1.ObjectSnapshot { return &ardentsv1.ObjectSnapshot{} },
		attributeValue: func(item *ardentsv1.ObjectSnapshot) string { return item.GetType() },
		list: func(callCtx context.Context) (*ardentsv1.ListObjectsResponse, error) {
			return connectMessage(a.ctx.Client.Service().ListObjects(callCtx, client.Request(&ardentsv1.ListObjectsRequest{})))
		},
		items: func(response *ardentsv1.ListObjectsResponse) []*ardentsv1.ObjectSnapshot {
			return response.GetObjects()
		},
		get: func(callCtx context.Context, id string) (*ardentsv1.ObjectSnapshot, error) {
			return connectMessage(a.ctx.Client.Service().GetObject(callCtx, client.Request(&ardentsv1.GetObjectRequest{Id: id})))
		},
		publish: func(callCtx context.Context, item *ardentsv1.ObjectSnapshot) (*ardentsv1.ObjectSnapshot, error) {
			return connectMessage(a.ctx.Client.Service().PublishObject(callCtx, client.Request(&ardentsv1.PublishObjectRequest{Object: item})))
		},
	}
	return resource.run(ctx, args)
}
