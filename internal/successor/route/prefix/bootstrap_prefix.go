package prefix

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// BootstrapConfig retains one genuine Domain-1 leg and physical reservation.
// It has no presentation callback: bootstrap cannot acquire private admission.
type BootstrapConfig struct {
	Leg      selection.Leg
	Current  func() (network.RuntimeView, error)
	Deadline time.Time
	Release  func() error
}

// BootstrapPrefix is a separate, finite issuer-only lifetime. It shares the
// framing/generation machinery, but never exposes an admitted Prefix or upgrades
// its children. The holder must join it before opening fresh admitted channels.
type BootstrapPrefix struct{ lifetime *Prefix }

func OpenBootstrap(ctx context.Context, config BootstrapConfig) (*BootstrapPrefix, error) {
	p, err := openPrefix(ctx, Config{Leg: config.Leg, Current: config.Current, Deadline: config.Deadline, Release: config.Release}, nil, true)
	if err != nil {
		return nil, err
	}
	return &BootstrapPrefix{lifetime: p}, nil
}

func (p *BootstrapPrefix) Issue(ctx context.Context, end time.Time, begin func(IssuerBinding) (IssuerBatch, error)) error {
	if p == nil {
		return net.ErrClosed
	}
	return p.lifetime.Issue(ctx, end, begin)
}

func (p *BootstrapPrefix) Close() error {
	if p == nil {
		return nil
	}
	return p.lifetime.Close()
}

func (p *BootstrapPrefix) Done() <-chan struct{} { return p.lifetime.Done() }

// CleanupResult reports only the original joined physical retirement.
func (p *BootstrapPrefix) CleanupResult() (error, bool) {
	if p == nil {
		return nil, true
	}
	return p.lifetime.CleanupResult()
}

// The two typed lifetimes share transport setup, without manufacturing a token
// presentation or inferring a mode from peer bytes.
func presentPrefixRole(ctx, caller context.Context, conn net.Conn, authority role.Authority, hello ardp.Hello, present role.Presentation, bootstrap bool) error {
	if !bootstrap {
		return role.Present(ctx, caller, conn, authority, hello, present)
	}
	check := func() error {
		_, err := authority.Hello(hello, false)
		return errors.Join(err, ctx.Err(), caller.Err())
	}
	if err := check(); err != nil {
		return err
	}
	flight, err := bootstrapFlight(hello)
	if err != nil {
		return err
	}
	// Both canonical records are already available. Submit them together so
	// nested TLS does not create a separate credit exchange for each record.
	// Both fresh authority checks precede output; ACCEPT still has its own
	// post-I/O check below.
	if err := check(); err != nil {
		return err
	}
	if _, err := io.Copy(conn, bytes.NewReader(flight)); err != nil {
		return err
	}
	return errors.Join(framing.Accepted(conn), check())
}

// bootstrapFlight encodes only already available records. It grants no role
// authority and adds no buffering delay or new wire envelope.
func bootstrapFlight(hello ardp.Hello) ([]byte, error) {
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		return nil, err
	}
	helloFrame, err := ardp.EncodeFrame(ardp.Frame{Kind: ardp.KindHello, Body: body})
	if err != nil {
		return nil, err
	}
	bootstrapFrame, err := ardp.EncodeFrame(ardp.Frame{Kind: ardp.KindBootstrap, Body: []byte{2}})
	if err != nil {
		return nil, err
	}
	return append(helloFrame, bootstrapFrame...), nil
}
