package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

// runDiagnosticTimeline projects the three bounded runtime event schemas from
// app JSON lines or journalctl JSON. It streams without retaining raw records.
func runDiagnosticTimeline(ctx context.Context, input io.ReadCloser, output io.Writer) error {
	if ctx == nil || input == nil || output == nil {
		return errors.New("diagnostic timeline input unavailable")
	}
	stop := context.AfterFunc(ctx, func() { _ = input.Close() })
	defer stop()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		row, present, err := diagnosticTimelineRow(scanner.Bytes())
		if err != nil {
			return err
		}
		if present {
			if _, err := io.WriteString(output, row); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return scanner.Err()
}

func diagnosticTimelineRow(raw []byte) (string, bool, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return "", false, nil
	}
	journalAt := time.Time{}
	pid := "-"
	if _, direct := fields["schema"]; !direct {
		message := diagnosticString(fields, "MESSAGE")
		if message == "" {
			return "", false, nil
		}
		journalAt = diagnosticJournalTime(diagnosticString(fields, "__REALTIME_TIMESTAMP"))
		if observed := diagnosticString(fields, "_PID"); diagnosticPID(observed) {
			pid = observed
		}
		if json.Unmarshal([]byte(message), &fields) != nil {
			return "", false, nil
		}
	}
	schema := diagnosticString(fields, "schema")
	kind := diagnosticString(fields, "kind")
	owner, role, carrier, state, reason := "", "", "", "", ""
	switch schema {
	case "ardents-node-event-v1":
		if kind != "lifecycle" && kind != "resource" && kind != "resource-sample" {
			return "", false, nil
		}
		owner, role, carrier, state, reason = "node", diagnosticString(fields, "assignment"),
			diagnosticString(fields, "carrier_profile"), diagnosticString(fields, "state"), diagnosticString(fields, "reason")
	case "ardents-source-event-v1":
		if kind != "source-ready" && kind != "source-wave-accepted" {
			return "", false, nil
		}
		owner = "source"
	case "ardents-headless-runtime-event-v1":
		switch kind {
		case "headless-runtime-ready", "headless-runtime-permission-required",
			"headless-runtime-publication-refresh-failed", "headless-runtime-publication-withdrawal-failed",
			"headless-runtime-connection-operation-failed":
		default:
			return "", false, nil
		}
		owner, role, reason = "endpoint", diagnosticString(fields, "surface"), diagnosticString(fields, "failure")
	default:
		return "", false, nil
	}
	if !diagnosticToken(kind, 64) || role != "" && !diagnosticToken(role, 64) ||
		carrier != "" && !diagnosticToken(carrier, 64) ||
		state != "" && !diagnosticToken(state, 32) || len(reason) > 256 {
		return "", false, errors.New("runtime diagnostic contains an invalid category")
	}
	at := time.Time{}
	clock := "event"
	if stamp := diagnosticString(fields, "at"); stamp != "" {
		var err error
		at, err = time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return "", false, errors.New("runtime diagnostic time is invalid")
		}
	}
	if at.IsZero() {
		at, clock = journalAt, "journal"
	}
	if at.IsZero() {
		return "", false, errors.New("runtime diagnostic has no event or journal time")
	}
	if role == "" {
		role = "-"
	}
	if carrier == "" {
		carrier = "-"
	}
	if state == "" {
		state = "-"
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%q\n",
		at.UTC().Format(time.RFC3339Nano), clock, owner, pid, role, carrier, kind, state, reason), true, nil
}

func diagnosticString(fields map[string]json.RawMessage, name string) string {
	var value string
	_ = json.Unmarshal(fields[name], &value)
	return value
}

func diagnosticJournalTime(raw string) time.Time {
	micros, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || micros <= 0 {
		return time.Time{}
	}
	return time.UnixMicro(micros).UTC()
}

func diagnosticToken(value string, limit int) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' ||
			ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

func diagnosticPID(value string) bool {
	if value == "" || len(value) > 10 {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
