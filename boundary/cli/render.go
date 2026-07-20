package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func renderJSON(w io.Writer, msg proto.Message) {
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		_, _ = fmt.Fprintf(w, "{\"error\":\"marshal json: %v\"}\n", err)
		return
	}
	_, _ = fmt.Fprintln(w, string(data))
}

func renderJSONLine(w io.Writer, msg proto.Message) error {
	data, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func printHeader(w io.Writer, title string) {
	_, _ = fmt.Fprintln(w, title)
}

func printStatusLine(w io.Writer, status *ardentsv1.OperationStatus) {
	if status == nil {
		return
	}
	printKV(w, "status", status.GetState())
	printKV(w, "reason", status.GetReason())
	printKV(w, "accepted", boolString(status.GetAccepted()))
}

func printKV(w io.Writer, key, value string) {
	if value == "" {
		return
	}
	_, _ = fmt.Fprintf(w, "%s: %s\n", key, value)
}

func printEvent(w io.Writer, evt *ardentsv1.EventEnvelope) {
	_, _ = fmt.Fprintf(w, "%s [%s/%s] %s %s\n",
		formatProtoTS(evt.GetTime()),
		evt.GetDomain(),
		evt.GetType(),
		evt.GetResource(),
		formatStruct(payloadMap(evt)))
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func formatTS(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	return ts.UTC().Format(time.RFC3339)
}

func formatProtoTS(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return "-"
	}
	return formatTS(ts.AsTime())
}

func payloadMap(evt *ardentsv1.EventEnvelope) map[string]any {
	if evt == nil || evt.GetPayload() == nil {
		return nil
	}
	return evt.GetPayload().AsMap()
}

func formatStruct(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, m[key]))
	}
	return strings.Join(parts, " ")
}

func consumeNodeEvents(w io.Writer, output string, stream *connect.ServerStreamForClient[ardentsv1.EventEnvelope], limit int) int {
	printed := 0
	for stream.Receive() {
		item := stream.Msg()
		if output == "json" {
			if err := renderJSONLine(w, item); err != nil {
				_, _ = fmt.Fprintf(w, "{\"error\":\"render event: %v\"}\n", err)
				return 1
			}
		} else {
			printEvent(w, item)
		}
		printed++
		if limit > 0 && printed >= limit {
			return 0
		}
	}
	if err := stream.Err(); err != nil {
		renderError(w, output == "json", err)
		return 1
	}
	return 0
}
