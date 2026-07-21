package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"ardents/internal/cli/output"

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

func (c Context) RunWatch(
	ctx context.Context,
	session string,
	fetch func(context.Context) (proto.Message, error),
	renderHuman func(io.Writer, proto.Message),
) int {
	initial, err := c.fetchWatchSnapshot(ctx, fetch)
	if err != nil {
		return c.Failure(err)
	}
	last, ok := c.renderWatchSnapshot(session, initial, renderHuman)
	if !ok {
		return 1
	}

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	retries := 0
	for {
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
		}

		next, nextRetries, code := c.watchTick(ctx, session, last, retries, fetch, renderHuman)
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

func (c Context) watchTick(
	ctx context.Context,
	session, last string,
	retries int,
	fetch func(context.Context) (proto.Message, error),
	renderHuman func(io.Writer, proto.Message),
) (string, int, int) {
	msg, err := c.fetchWatchSnapshot(ctx, fetch)
	if err != nil {
		next, stop := c.handleWatchError(session, retries, err)
		if stop {
			return last, next, 1
		}
		return last, next, watchTickSkip
	}
	if retries > 0 {
		c.renderWatchNotice(session, "recovered", retries, "", true)
	}
	next, err := watchFingerprint(msg)
	if err != nil {
		c.Failure(fmt.Errorf("%s fingerprint: %w", session, err))
		return last, retries, 1
	}
	if next == last {
		return last, 0, watchTickSkip
	}
	if !c.printWatchUpdate(session, msg, renderHuman) {
		return last, retries, 1
	}
	return next, 0, 0
}

func (c Context) handleWatchError(session string, retries int, err error) (int, bool) {
	retries++
	c.renderWatchNotice(session, "retry", retries, err.Error(), false)
	if retries >= maxWatchRetries {
		c.Failure(fmt.Errorf("%s watch exhausted retry budget: %w", session, err))
		return retries, true
	}
	return retries, false
}

func (c Context) renderWatchSnapshot(
	session string,
	msg proto.Message,
	renderHuman func(io.Writer, proto.Message),
) (string, bool) {
	fp, err := watchFingerprint(msg)
	if err != nil {
		c.Failure(fmt.Errorf("%s fingerprint: %w", session, err))
		return "", false
	}
	if !c.printWatchUpdate("", msg, renderHuman) {
		return "", false
	}
	return fp, true
}

func (c Context) printWatchUpdate(
	session string,
	msg proto.Message,
	renderHuman func(io.Writer, proto.Message),
) bool {
	if c.Renderer.JSON {
		if err := output.JSONLine(c.Renderer.Out, msg); err != nil {
			c.Failure(fmt.Errorf("%s render: %w", session, err))
			return false
		}
		return true
	}
	if session != "" {
		output.Writef(c.Renderer.Out, "%s update: %s\n", output.FormatTime(time.Now()), session)
	}
	renderHuman(c.Renderer.Out, msg)
	return true
}

func (c Context) fetchWatchSnapshot(
	ctx context.Context,
	fetch func(context.Context) (proto.Message, error),
) (proto.Message, error) {
	callCtx, cancel := c.Call(ctx)
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

func (c Context) renderWatchNotice(session, kind string, attempt int, message string, recovered bool) {
	notice := watchNotice{
		Kind:      kind,
		Time:      output.FormatTime(time.Now()),
		Session:   session,
		Attempt:   attempt,
		Message:   message,
		Recovered: recovered,
	}
	if c.Renderer.JSON {
		data, err := json.Marshal(notice)
		if err != nil {
			c.Failure(fmt.Errorf("marshal watch notice: %w", err))
			return
		}
		output.Writeln(c.Renderer.Out, string(data))
		return
	}
	switch kind {
	case "retry":
		output.Writef(c.Renderer.Out, "%s watch retry: %s attempt=%d error=%s\n", notice.Time, session, attempt, message)
	case "recovered":
		output.Writef(c.Renderer.Out, "%s watch recovered: %s after=%d\n", notice.Time, session, attempt)
	}
}

func watchFingerprint(msg proto.Message) (string, error) {
	data, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
