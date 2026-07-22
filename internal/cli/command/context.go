// Package command owns shared command context, explicit input, and watch execution.
// It does not own area-specific commands or rendering.
package command

import (
	"context"
	"io"
	"time"

	"ardents/internal/cli/client"
	"ardents/internal/cli/output"

	"google.golang.org/protobuf/proto"
)

type WatchSnapshots func(
	context.Context,
	string,
	func(context.Context) (proto.Message, error),
	func(io.Writer, proto.Message),
) int

type Context struct {
	Client   *client.Client
	Input    io.Reader
	Timeout  time.Duration
	Interval time.Duration
	Watch    bool
	Renderer output.Renderer
	WatchRun WatchSnapshots
	Dispatch func(context.Context, []string) int
	Usage    func(io.Writer)
	Operator Operator
}

type Operator struct {
	Address, Name, Principal, Node, PublicKey string
}

func (c Context) Call(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, c.Timeout)
}

func (c Context) Failure(err error) int { return c.Renderer.Failure(err) }
