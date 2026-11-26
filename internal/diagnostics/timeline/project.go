package timeline

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

// Project renders the three bounded runtime event schemas from
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
	if at.IsZero() {
		return "", false, errors.New("runtime diagnostic has no event or journal time")
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
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%q\n",
		at.UTC().Format(time.RFC3339Nano), clock, record.Owner, pid, role, carrier, record.Kind, state, record.Reason), true, nil
}
