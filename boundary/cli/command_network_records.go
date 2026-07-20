package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func (a *app) resolve(ctx context.Context, args []string) int {
	if len(args) == 0 {
		renderNetworkResolveUsage(a.stderr)
		return 2
	}
	switch args[0] {
	case "record":
		return a.resolveRecord(ctx, args[1:])
	case "service":
		return a.resolveService(ctx, args[1:])
	default:
		renderNetworkResolveUsage(a.stderr)
		return 2
	}
}

func (a *app) resolveRecord(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network resolve record", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var req ardentsv1.ResolveRecordRequest
	fs.StringVar(&req.Subject, "subject", "", "subject id")
	fs.StringVar(&req.Kind, "kind", "", "record kind")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	if err := requireValue("subject", req.Subject); err != nil {
		return a.fail(err)
	}
	if err := requireValue("kind", req.Kind); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ResolveRecord(callCtx, client.Request(a.cfg.Token, &req))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network resolve record")
	printKV(a.stdout, "outcome", resp.Msg.GetOutcome())
	printKV(a.stdout, "source", resp.Msg.GetSource())
	record := resp.Msg.GetRecord()
	printKV(a.stdout, "record_id", record.GetId())
	printKV(a.stdout, "subject", record.GetSubject())
	printKV(a.stdout, "kind", record.GetKind())
	printKV(a.stdout, "node", record.GetNode())
	printKV(a.stdout, "trust", resp.Msg.GetTrust().GetState())
	printKV(a.stdout, "usable", boolString(resp.Msg.GetTrust().GetUsable()))
	return 0
}

func (a *app) resolveService(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network resolve service", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var req ardentsv1.ResolveServiceRequest
	fs.StringVar(&req.Service, "service", "", "service id")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	if err := requireValue("service", req.Service); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ResolveService(callCtx, client.Request(a.cfg.Token, &req))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network resolve service")
	printKV(a.stdout, "service", resp.Msg.GetService())
	printKV(a.stdout, "outcome", resp.Msg.GetOutcome())
	printKV(a.stdout, "matches", fmt.Sprintf("%d", len(resp.Msg.GetMatches())))
	printKV(a.stdout, "route_outcome", resp.Msg.GetRoute().GetOutcome())
	for _, match := range resp.Msg.GetMatches() {
		printKV(a.stdout, "match_record", match.GetRecord().GetId())
		printKV(a.stdout, "  node", match.GetRecord().GetNode())
		printKV(a.stdout, "  trust", match.GetTrust().GetState())
	}
	return 0
}

func (a *app) records(ctx context.Context, args []string) int {
	if len(args) == 0 {
		renderNetworkRecordsUsage(a.stderr)
		return 2
	}
	switch args[0] {
	case "list":
		return a.listRecords(ctx)
	case "import":
		return a.importRecord(ctx, args[1:])
	default:
		renderNetworkRecordsUsage(a.stderr)
		return 2
	}
}

func (a *app) listRecords(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListRecords(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListRecordsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network records")
	if len(resp.Msg.GetRecords()) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "no discovery records")
		return 0
	}
	for _, item := range resp.Msg.GetRecords() {
		printKV(a.stdout, "record", item.GetId())
		printKV(a.stdout, "  kind", item.GetKind())
		printKV(a.stdout, "  subject", item.GetSubject())
		printKV(a.stdout, "  node", item.GetNode())
	}
	return 0
}

func (a *app) importRecord(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network records import", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var file string
	fs.StringVar(&file, "file", "", "path to discovery record json or - for stdin")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	if err := requireValue("file", file); err != nil {
		return a.fail(err)
	}
	record, err := loadDiscoveryRecord(file)
	if err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ImportRecord(callCtx, client.Request(a.cfg.Token, &ardentsv1.ImportRecordRequest{Record: record}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network records import")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	return 0
}

func loadDiscoveryRecord(path string) (*ardentsv1.DiscoveryRecord, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = ioReadAll(os.Stdin)
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
