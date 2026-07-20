package cli

import (
	"context"
	"fmt"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) dataObjects(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.fail(fmt.Errorf("data objects subcommand is required"))
	}
	switch args[0] {
	case "list":
		return a.dataObjectsList(ctx)
	case "get":
		return a.dataObjectsGet(ctx, args[1:])
	case "publish":
		return a.dataObjectsPublish(ctx, args[1:])
	default:
		return a.fail(fmt.Errorf("unknown data objects subcommand %q", args[0]))
	}
}

func (a *app) dataObjectsList(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListObjects(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListObjectsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data objects")
	for _, item := range resp.Msg.GetObjects() {
		printKV(a.stdout, "object", item.GetId())
		printKV(a.stdout, "  type", item.GetType())
		printKV(a.stdout, "  owner", item.GetOwner())
	}
	return 0
}

func (a *app) dataObjectsGet(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("object id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetObject(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetObjectRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data object")
	printKV(a.stdout, "id", resp.Msg.GetId())
	printKV(a.stdout, "type", resp.Msg.GetType())
	printKV(a.stdout, "owner", resp.Msg.GetOwner())
	return 0
}

func (a *app) dataObjectsPublish(ctx context.Context, args []string) int {
	file, err := parseFileArg("data objects publish", a.stderr, args)
	if err != nil {
		return a.fail(err)
	}
	obj := &ardentsv1.ObjectSnapshot{}
	if err := loadProtoJSON(file, obj); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().PublishObject(callCtx, client.Request(a.cfg.Token, &ardentsv1.PublishObjectRequest{Object: obj}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data object publish")
	printKV(a.stdout, "id", resp.Msg.GetId())
	printKV(a.stdout, "type", resp.Msg.GetType())
	return 0
}
