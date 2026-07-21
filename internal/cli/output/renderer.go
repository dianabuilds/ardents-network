// Package output owns shared human and JSON output mechanics.
// It does not own product-specific projections.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Renderer struct {
	Out, Err io.Writer
	JSON     bool
	state    *writeState
}

type writeState struct {
	mu  sync.Mutex
	err error
}

type trackedWriter struct {
	target io.Writer
	state  *writeState
}

func NewRenderer(out, errOut io.Writer, jsonOutput bool) Renderer {
	state := &writeState{}
	return Renderer{Out: trackedWriter{target: out, state: state}, Err: trackedWriter{target: errOut, state: state}, JSON: jsonOutput, state: state}
}

func (w trackedWriter) Write(raw []byte) (int, error) {
	written, err := w.target.Write(raw)
	if err != nil {
		w.state.record(err)
	}
	return written, err
}

func (s *writeState) record(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func (r Renderer) OutputError() error {
	if r.state == nil {
		return nil
	}
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	return r.state.err
}

func Writef(writer io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(writer, format, args...); err != nil {
		recordWriteError(writer, err)
	}
}

func Writeln(writer io.Writer, args ...any) {
	if _, err := fmt.Fprintln(writer, args...); err != nil {
		recordWriteError(writer, err)
	}
}

func recordWriteError(writer io.Writer, err error) {
	if tracked, ok := writer.(trackedWriter); ok {
		tracked.state.record(err)
		return
	}
	slog.Error("write CLI output", "error", err)
}

func (r Renderer) Message(message proto.Message) {
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(message)
	if err != nil {
		Writef(r.Err, "error: marshal output: %v\n", err)
		return
	}
	Writeln(r.Out, string(data))
}

func (r Renderer) Header(title string) { Writeln(r.Out, title) }

func (r Renderer) KV(key, value string) {
	if value != "" {
		Writef(r.Out, "%s: %s\n", key, value)
	}
}

func Header(writer io.Writer, title string) { Writeln(writer, title) }

func KV(writer io.Writer, key, value string) {
	if value != "" {
		Writef(writer, "%s: %s\n", key, value)
	}
}

func Status(writer io.Writer, status *protocol.OperationStatus) {
	if status == nil {
		return
	}
	KV(writer, "status", status.GetState())
	KV(writer, "reason", status.GetReason())
	KV(writer, "accepted", fmt.Sprint(status.GetAccepted()))
}

func JSON(writer io.Writer, message proto.Message) {
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(message)
	if err == nil {
		Writeln(writer, string(data))
	}
}

func JSONLine(writer io.Writer, message proto.Message) error {
	data, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(message)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func ConsumeEvents(renderer Renderer, stream *connect.ServerStreamForClient[protocol.EventEnvelope], limit int) int {
	printed := 0
	for stream.Receive() {
		if renderer.JSON {
			if err := JSONLine(renderer.Out, stream.Msg()); err != nil {
				return renderer.Failure(err)
			}
		} else {
			Event(renderer.Out, stream.Msg())
		}
		printed++
		if limit > 0 && printed >= limit {
			return 0
		}
	}
	if err := stream.Err(); err != nil {
		return renderer.Failure(err)
	}
	return 0
}

func Bool(value bool) string { return fmt.Sprint(value) }

func Timestamp(value *timestamppb.Timestamp) string {
	if value == nil {
		return "-"
	}
	return value.AsTime().UTC().Format(time.RFC3339)
}

func FormatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func Event(writer io.Writer, event *protocol.EventEnvelope) {
	payload := ""
	if event.GetPayload() != nil {
		payload = fmt.Sprint(event.GetPayload().AsMap())
	}
	Writef(writer, "%s [%s/%s] %s %s\n", Timestamp(event.GetTime()), event.GetDomain(), event.GetType(), event.GetResource(), payload)
}

func (r Renderer) Status(status *protocol.OperationStatus) {
	if status == nil {
		return
	}
	r.KV("status", status.GetState())
	r.KV("reason", status.GetReason())
	r.KV("accepted", fmt.Sprint(status.GetAccepted()))
}

func (r Renderer) CSV(key string, values []string) {
	if len(values) > 0 {
		r.KV(key, strings.Join(values, ", "))
	}
}

func (r Renderer) Failure(err error) int {
	payload := errorPayload{Message: err.Error()}
	if connectErr, ok := errors.AsType[*connect.Error](err); ok {
		payload.Code, payload.Message = connectErr.Code().String(), connectErr.Message()
		for _, detail := range connectErr.Details() {
			message, detailErr := detail.Value()
			apiError, ok := message.(*protocol.Error)
			if detailErr == nil && ok {
				payload = errorPayload{Code: apiError.GetCode(), Category: apiError.GetCategory(), Message: apiError.GetMessage(), Domain: apiError.GetDomain(), Operation: apiError.GetOperation(), Reason: apiError.GetReason(), Retryable: apiError.GetRetryable()}
				break
			}
		}
	}
	if r.JSON {
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			Writef(r.Err, "error: marshal failure output: %v\n", marshalErr)
			return 1
		}
		Writeln(r.Err, string(encoded))
	} else {
		renderError(r.Err, payload)
	}
	return 1
}

type errorPayload struct {
	Code      string `json:"code"`
	Category  string `json:"category,omitempty"`
	Message   string `json:"message"`
	Domain    string `json:"domain,omitempty"`
	Operation string `json:"operation,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Retryable bool   `json:"retryable"`
}

func renderError(writer io.Writer, value errorPayload) {
	for _, item := range [][2]string{{"error", value.Code}, {"category", value.Category}, {"domain", value.Domain}, {"operation", value.Operation}, {"message", value.Message}, {"reason", value.Reason}} {
		if item[1] != "" {
			Writef(writer, "%s: %s\n", item[0], item[1])
		}
	}
	if value.Retryable {
		Writeln(writer, "retryable: true")
	}
}
