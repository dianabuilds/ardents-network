package introduction

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
)

// SubmissionRefusal is an actual matching fixed transport outcome. It grants
// neither a retry nor recipient/Connection authority.
type SubmissionRefusal struct{ Status uint8 }

func (err SubmissionRefusal) Error() string {
	return fmt.Sprintf("introduction submission refused (%d)", err.Status)
}

// Submit sends only the sealed capsule through the original Source and exact
// Introduction duty. It consumes genuine Control Stock on fresh role TLS;
// successful transport does not authenticate a Service Connection.
func Submit(ctx context.Context, source *prefix.Prefix, duty network.RetainedDuty, envelope capsule.Envelope, excluded []route.Member) (result error) {
	if source == nil || ctx == nil {
		return errors.New("introduction submission Source absent")
	}
	if _, err := capsule.Parse(envelope.Bytes()); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	body, err := EncodeDelivery(nonce, envelope)
	if err != nil {
		return err
	}
	terminal, err := source.OpenSubmissionChannel(ctx, duty, envelope.Header().Expiry, excluded)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, terminal.CloseSetup(), terminal.CheckOriginal())
		terminal.FinishSetup()
	}()
	if err := terminal.Check(); err != nil {
		return err
	}
	if err := ardp.WriteFrame(terminal.Stream(), ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		return err
	}
	if err := terminal.Check(); err != nil {
		return err
	}
	frame, err := terminal.ReadFrame()
	if err != nil {
		return errors.Join(err, terminal.Check())
	}
	if frame.Kind != ardp.KindResult || frame.Lane != 0 {
		return errors.New("introduction submission lane-zero RESULT required")
	}
	status, err := DecodeResult(frame.Body, nonce)
	if err != nil {
		return err
	}
	if err := terminal.Check(); err != nil {
		return err
	}
	if err := terminal.FinishExchange(); err != nil {
		return err
	}
	if status != 0 {
		return SubmissionRefusal{Status: status}
	}
	return nil
}
