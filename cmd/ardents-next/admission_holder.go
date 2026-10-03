package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
)

type holderCommand struct {
	Operation     string               `json:"operation"`
	Maxima        [3]uint32            `json:"maxima,omitzero"`
	Digest        [32]byte             `json:"digest,omitzero"`
	Payload       []byte               `json:"payload,omitzero"`
	Intent        stock.IssuanceIntent `json:"intent,omitzero"`
	Presentation  stock.Presentation   `json:"presentation,omitzero"`
	Class         uint8                `json:"class,omitzero"`
	Failed        bool                 `json:"failed,omitzero"`
	Receivers     [][32]byte           `json:"receivers,omitzero"`
	RequiresToken bool                 `json:"requires_token,omitzero"`
}

func runAdmissionHolder(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) (code int) {
	var config struct {
		Root    string                   `json:"root"`
		Profile string                   `json:"profile"`
		Role    admission.AllocationRole `json:"role"`
	}
	if ctx == nil || admissionConfig(args, &config) != nil || !absoluteAdmissionPath(config.Root) || !absoluteAdmissionPath(config.Profile) {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	o, err := stock.Open(config.Root, config.Role, admissionObserver(config.Profile))
	if err != nil {
		return 1
	}
	defer func() {
		if o.Close() != nil {
			code = 1
		}
		_ = json.NewEncoder(diagnostic).Encode(map[string]any{"operation": "admission.holder", "phase": "closed", "exit_code": code})
	}()
	var attempt stock.Attempt
	return admissionConsole(ctx, input, out, func(ctx context.Context, raw []byte) (any, bool, error) {
		var c holderCommand
		if err := decodeAdmissionObject(raw, &c); err != nil {
			return nil, false, err
		}
		var err error
		result := map[string]any{"outcome": "completed"}
		switch c.Operation {
		case "request":
			var wire []byte
			var digest [32]byte
			wire, digest, err = o.Request(c.Maxima)
			result["request"], result["digest"] = wire, digest
		case "import":
			err = o.Import(c.Digest, c.Payload)
		case "begin":
			var next stock.Attempt
			next, err = o.Begin(c.Intent)
			if err == nil {
				attempt = next
				result["request"], result["deadline"], err = attempt.Request()
			}
		case "complete":
			var exchangeErr error
			if c.Failed {
				exchangeErr = errors.New("exchange failed")
			}
			err = attempt.Complete(c.Payload, exchangeErr)
		case "discard":
			attempt.Discard()
		case "take":
			result["token"], err = o.Take(ctx, c.Presentation, c.Class)
		case "refill-plan":
			result["receivers"], err = o.RefillPlan(c.Receivers, c.Class, c.RequiresToken)
		case "status":
			result["stock"] = o.Status()
		case "close":
			if err := o.Close(); err != nil {
				return nil, true, err
			}
			return result, true, nil
		default:
			err = errors.New("unknown holder operation")
		}
		if err != nil {
			return map[string]string{"outcome": "refused", "stage": stock.TransferFailureStage(err)}, false, err
		}
		return result, false, nil
	})
}
