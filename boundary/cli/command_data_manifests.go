package cli

import (
	"context"
	"fmt"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) dataManifests(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.fail(fmt.Errorf("data manifests subcommand is required"))
	}
	switch args[0] {
	case "list":
		return a.dataManifestsList(ctx)
	case "get":
		return a.dataManifestsGet(ctx, args[1:])
	case "publish":
		return a.dataManifestsPublish(ctx, args[1:])
	default:
		return a.fail(fmt.Errorf("unknown data manifests subcommand %q", args[0]))
	}
}

func (a *app) dataManifestsList(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListManifests(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListManifestsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data manifests")
	for _, item := range resp.Msg.GetManifests() {
		printKV(a.stdout, "manifest", item.GetId())
		printKV(a.stdout, "  kind", item.GetKind())
		printKV(a.stdout, "  owner", item.GetOwner())
	}
	return 0
}

func (a *app) dataManifestsGet(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("manifest id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetManifest(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetManifestRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data manifest")
	printKV(a.stdout, "id", resp.Msg.GetId())
	printKV(a.stdout, "kind", resp.Msg.GetKind())
	printKV(a.stdout, "owner", resp.Msg.GetOwner())
	return 0
}

func (a *app) dataManifestsPublish(ctx context.Context, args []string) int {
	file, err := parseFileArg("data manifests publish", a.stderr, args)
	if err != nil {
		return a.fail(err)
	}
	manifest := &ardentsv1.ManifestSnapshot{}
	if err := loadProtoJSON(file, manifest); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().PublishManifest(callCtx, client.Request(a.cfg.Token, &ardentsv1.PublishManifestRequest{Manifest: manifest}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data manifest publish")
	printKV(a.stdout, "id", resp.Msg.GetId())
	printKV(a.stdout, "kind", resp.Msg.GetKind())
	return 0
}
