package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxWatchRetries = 5

type watchNotice struct {
	Kind      string `json:"kind"`
	Time      string `json:"time"`
	Session   string `json:"session"`
	Attempt   int    `json:"attempt,omitempty"`
	Message   string `json:"message,omitempty"`
	Recovered bool   `json:"recovered,omitempty"`
}

func (a *app) watchSnapshots(
	ctx context.Context,
	session string,
	fetch func(context.Context) (proto.Message, error),
	renderHuman func(io.Writer, proto.Message),
) int {
	initial, err := a.fetchWatchSnapshot(ctx, fetch)
	if err != nil {
		return a.fail(err)
	}
	last, ok := a.renderWatchSnapshot(session, initial, renderHuman)
	if !ok {
		return 1
	}

	ticker := time.NewTicker(a.cfg.Interval)
	defer ticker.Stop()

	retries := 0
	for {
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
		}

		next, nextRetries, code := a.watchTick(ctx, session, last, retries, fetch, renderHuman)
		switch code {
		case 0:
			last = next
			retries = nextRetries
		case watchTickSkip:
			retries = nextRetries
			continue
		default:
			return 1
		}
	}
}

const (
	watchTickSkip = -1
)

func (a *app) watchTick(
	ctx context.Context,
	session, last string,
	retries int,
	fetch func(context.Context) (proto.Message, error),
	renderHuman func(io.Writer, proto.Message),
) (string, int, int) {
	msg, err := a.fetchWatchSnapshot(ctx, fetch)
	if err != nil {
		next, stop := a.handleWatchError(session, retries, err)
		if stop {
			return last, next, 1
		}
		return last, next, watchTickSkip
	}
	if retries > 0 {
		a.renderWatchNotice(session, "recovered", retries, "", true)
	}
	next, err := watchFingerprint(msg)
	if err != nil {
		a.fail(fmt.Errorf("%s fingerprint: %w", session, err))
		return last, retries, 1
	}
	if next == last {
		return last, 0, watchTickSkip
	}
	if !a.printWatchUpdate(session, msg, renderHuman) {
		return last, retries, 1
	}
	return next, 0, 0
}

func (a *app) handleWatchError(session string, retries int, err error) (int, bool) {
	retries++
	a.renderWatchNotice(session, "retry", retries, err.Error(), false)
	if retries >= maxWatchRetries {
		a.fail(fmt.Errorf("%s watch exhausted retry budget: %w", session, err))
		return retries, true
	}
	return retries, false
}

func (a *app) renderWatchSnapshot(
	session string,
	msg proto.Message,
	renderHuman func(io.Writer, proto.Message),
) (string, bool) {
	fp, err := watchFingerprint(msg)
	if err != nil {
		a.fail(fmt.Errorf("%s fingerprint: %w", session, err))
		return "", false
	}
	if !a.printWatchUpdate("", msg, renderHuman) {
		return "", false
	}
	return fp, true
}

func (a *app) printWatchUpdate(
	session string,
	msg proto.Message,
	renderHuman func(io.Writer, proto.Message),
) bool {
	if a.jsonMode() {
		if err := renderJSONLine(a.stdout, msg); err != nil {
			a.fail(fmt.Errorf("%s render: %w", session, err))
			return false
		}
		return true
	}
	if session != "" {
		_, _ = fmt.Fprintf(a.stdout, "%s update: %s\n", formatTS(time.Now()), session)
	}
	renderHuman(a.stdout, msg)
	return true
}

func (a *app) fetchWatchSnapshot(
	ctx context.Context,
	fetch func(context.Context) (proto.Message, error),
) (proto.Message, error) {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	msg, err := fetch(callCtx)
	if err == nil {
		return msg, nil
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, ctx.Err()
	}
	return nil, err
}

func (a *app) renderWatchNotice(session, kind string, attempt int, message string, recovered bool) {
	notice := watchNotice{
		Kind:      kind,
		Time:      formatTS(time.Now()),
		Session:   session,
		Attempt:   attempt,
		Message:   message,
		Recovered: recovered,
	}
	if a.jsonMode() {
		data, err := json.Marshal(notice)
		if err == nil {
			_, _ = fmt.Fprintln(a.stdout, string(data))
		}
		return
	}
	switch kind {
	case "retry":
		_, _ = fmt.Fprintf(a.stdout, "%s watch retry: %s attempt=%d error=%s\n", notice.Time, session, attempt, message)
	case "recovered":
		_, _ = fmt.Fprintf(a.stdout, "%s watch recovered: %s after=%d\n", notice.Time, session, attempt)
	}
}

func watchFingerprint(msg proto.Message) (string, error) {
	data, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
