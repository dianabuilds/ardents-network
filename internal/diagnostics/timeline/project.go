package timeline

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

// Project renders retained runtime events and current command reports from
// app JSON lines or journalctl JSON. It streams without retaining raw records.
func Project(ctx context.Context, input io.ReadCloser, output io.Writer) error {
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
	record, present, err := runtimeevent.Decode(raw)
	if err != nil || !present {
		return "", false, err
	}
	at, clock := record.At, "event"
	if at.IsZero() {
		at, clock = record.JournalAt, "journal"
	}
	stamp := at.UTC().Format(time.RFC3339Nano)
	if at.IsZero() {
		if record.Schema != runtimeevent.CommandReportSchema {
			return "", false, errors.New("runtime diagnostic has no event or journal time")
		}
		stamp, clock = "-", "unavailable"
	}
	if record.Measurements != nil {
		var rows strings.Builder
		pid := record.PID
		if pid == "" {
			pid = "-"
		}
		for _, value := range record.Measurements.Values {
			spec, _ := runtimeevent.LookupInstrument(value.Name).Spec()
			detail := "unavailable"
			if value.Available {
				detail = fmt.Sprintf("current=%d peak=%d", value.Current, value.Peak)
				if spec.Kind == runtimeevent.StageInstrument {
					detail += fmt.Sprintf(" calls=%d seconds=%g maximum_seconds=%g", value.Count, value.Seconds, value.MaximumSeconds)
				}
			}
			fmt.Fprintf(&rows, "%s\tsample\t%s\t%s\t-\t-\t%s\t%s\t%q\n", stamp, spec.Owner, pid, spec.Name, spec.Unit, detail)
		}
		return rows.String(), true, nil
	}

	pid, role, carrier, state := record.PID, record.Role, record.Carrier, record.State
	if pid == "" {
		pid = "-"
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
	detail := record.Reason
	if record.Schema == runtimeevent.CommandReportSchema {
		state, detail = record.Phase, record.Outcome
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%q\n",
		stamp, clock, record.Owner, pid, role, carrier, record.Kind, state, detail), true, nil
}
