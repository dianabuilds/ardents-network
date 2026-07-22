package network

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"ardents/internal/cli/client"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"

	"google.golang.org/protobuf/encoding/protojson"
)

func (a *Command) resolve(ctx context.Context, args []string) int {
	if len(args) == 0 {
		renderNetworkResolveUsage(a.ctx.Renderer.Err)
		return 2
	}
	switch args[0] {
	case "record":
		return a.resolveRecord(ctx, args[1:])
	case "service":
		return a.resolveService(ctx, args[1:])
	default:
		renderNetworkResolveUsage(a.ctx.Renderer.Err)
		return 2
	}
}

func (a *Command) resolveRecord(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network resolve record", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var req ardentsv1.ResolveRecordRequest
	fs.StringVar(&req.Subject, "subject", "", "subject id")
	fs.StringVar(&req.Kind, "kind", "", "record kind")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	if err := requireValue("subject", req.Subject); err != nil {
		return a.ctx.Failure(err)
	}
	if err := requireValue("kind", req.Kind); err != nil {
		return a.ctx.Failure(err)
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ResolveRecord(callCtx, client.Request(&req))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network resolve record")
	output.KV(a.ctx.Renderer.Out, "outcome", resp.Msg.GetOutcome())
	output.KV(a.ctx.Renderer.Out, "source", resp.Msg.GetSource())
	record := resp.Msg.GetRecord()
	output.KV(a.ctx.Renderer.Out, "record_id", record.GetId())
	output.KV(a.ctx.Renderer.Out, "subject", record.GetSubject())
	output.KV(a.ctx.Renderer.Out, "kind", record.GetKind())
	output.KV(a.ctx.Renderer.Out, "node", record.GetNode())
	output.KV(a.ctx.Renderer.Out, "trust", resp.Msg.GetTrust().GetState())
	output.KV(a.ctx.Renderer.Out, "usable", output.Bool(resp.Msg.GetTrust().GetUsable()))
	return 0
}

func (a *Command) resolveService(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network resolve service", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var req ardentsv1.ResolveServiceRequest
	fs.StringVar(&req.Service, "service", "", "service id")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	if err := requireValue("service", req.Service); err != nil {
		return a.ctx.Failure(err)
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ResolveService(callCtx, client.Request(&req))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network resolve service")
	output.KV(a.ctx.Renderer.Out, "service", resp.Msg.GetService())
	output.KV(a.ctx.Renderer.Out, "outcome", resp.Msg.GetOutcome())
	output.KV(a.ctx.Renderer.Out, "matches", fmt.Sprintf("%d", len(resp.Msg.GetMatches())))
	output.KV(a.ctx.Renderer.Out, "route_outcome", resp.Msg.GetRoute().GetOutcome())
	for _, match := range resp.Msg.GetMatches() {
		output.KV(a.ctx.Renderer.Out, "match_record", match.GetRecord().GetId())
		output.KV(a.ctx.Renderer.Out, "  node", match.GetRecord().GetNode())
		output.KV(a.ctx.Renderer.Out, "  trust", match.GetTrust().GetState())
	}
	return 0
}

func (a *Command) records(ctx context.Context, args []string) int {
	if len(args) == 0 {
		renderNetworkRecordsUsage(a.ctx.Renderer.Err)
		return 2
	}
	switch args[0] {
	case "list":
		return a.listRecords(ctx)
	case "import":
		return a.importRecord(ctx, args[1:])
	default:
		renderNetworkRecordsUsage(a.ctx.Renderer.Err)
		return 2
	}
}

func (a *Command) listRecords(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListRecords(callCtx, client.Request(&ardentsv1.ListRecordsRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network records")
	if len(resp.Msg.GetRecords()) == 0 {
		output.Writeln(a.ctx.Renderer.Out, "no discovery records")
		return 0
	}
	for _, item := range resp.Msg.GetRecords() {
		output.KV(a.ctx.Renderer.Out, "record", item.GetId())
		output.KV(a.ctx.Renderer.Out, "  kind", item.GetKind())
		output.KV(a.ctx.Renderer.Out, "  subject", item.GetSubject())
		output.KV(a.ctx.Renderer.Out, "  node", item.GetNode())
	}
	return 0
}

func (a *Command) importRecord(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network records import", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var file string
	fs.StringVar(&file, "file", "", "path to discovery record json or - for stdin")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	if err := requireValue("file", file); err != nil {
		return a.ctx.Failure(err)
	}
	record, err := loadDiscoveryRecord(a.ctx.Input, file)
	if err != nil {
		return a.ctx.Failure(err)
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ImportRecord(callCtx, client.Request(&ardentsv1.ImportRecordRequest{Record: record}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network records import")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	return 0
}

func loadDiscoveryRecord(input io.Reader, path string) (*ardentsv1.DiscoveryRecord, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(input)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read discovery record: %w", err)
	}
	record := &ardentsv1.DiscoveryRecord{}
	if err := protojson.Unmarshal(data, record); err != nil {
		return nil, fmt.Errorf("parse discovery record json: %w", err)
	}
	return record, nil
}

func renderNetworkResolveUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] network resolve <record|service>")
}

func renderNetworkRecordsUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] network records <list|import>")
}

func requireValue(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}
