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
		Network *networkAuthorityPlan    `json:"network,omitempty"`
		Role    admission.AllocationRole `json:"role"`
		Route   *routePrefixPlan         `json:"route,omitempty"`
	}
	if ctx == nil || admissionConfig(args, &config) != nil || !absoluteAdmissionPath(config.Root) || !validAdmissionAuthority(config.Profile, config.Network, config.Root) {
		return 2
	}
	if config.Route != nil && (config.Network == nil || !independentRouteRoots(config.Network.Root, config.Root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.HostingRoot)) {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	authority, err := openAdmissionAuthority(config.Profile, config.Network)
	if err != nil {
		return 1
	}
	defer func() {
		if authority.close() != nil {
			code = 1
		}
	}()
	o, err := stock.Open(config.Root, config.Role, authority.observe)
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
	var retainedIntent stock.IssuanceIntent
	var prefix routeHandle
	defer func() {
		if prefix.close != nil && prefix.close() != nil {
			code = 1
		}
	}()
	return admissionConsole(ctx, input, out, func(ctx context.Context, raw []byte) (any, bool, error) {
		var c holderCommand
		if err := decodeAdmissionObject(raw, &c); err != nil {
			return nil, false, err
		}
		var err error
		result := map[string]any{"outcome": "completed"}
		switch c.Operation {
		case "prefix-open":
			if prefix.close != nil {
				select {
				case <-prefix.done:
					err = errors.New("route prefix retired; close to retrieve outcome")
				default:
					err = errors.New("route prefix already open")
				}
				break
			}
			if config.Route == nil || authority.current == nil {
				err = errors.New("route prefix unavailable")
				break
			}
			prefix, err = startRoutePrefix(ctx, *config.Route, authority, o)
		case "prefix-close":
			if prefix.close == nil {
				err = errors.New("route prefix absent")
				break
			}
			err = prefix.close()
			prefix = routeHandle{}
		case "request":
			var wire []byte
			var digest [32]byte
			wire, digest, err = o.Request(c.Maxima)
			result["request"], result["digest"] = wire, digest
		case "import":
			err = o.Import(c.Digest, c.Payload)
		case "begin":
			if authority.intent != nil {
				if err = authority.intent(c.Intent); err != nil {
					break
				}
			}
			var next stock.Attempt
			next, err = o.Begin(c.Intent)
			if err == nil {
				attempt = next
				retainedIntent = c.Intent
				result["request"], result["deadline"], err = attempt.Request()
			}
		case "complete":
			if authority.intent != nil {
				if err = authority.intent(retainedIntent); err != nil {
					break
				}
			}
			var exchangeErr error
			if c.Failed {
				exchangeErr = errors.New("exchange failed")
			}
			err = attempt.Complete(c.Payload, exchangeErr)
		case "discard":
			attempt.Discard()
		case "take":
			if authority.presentation != nil {
				if err = authority.presentation(c.Presentation); err != nil {
					break
				}
			}
			result["token"], err = o.Take(ctx, c.Presentation, c.Class)
			if err == nil && authority.presentation != nil {
				if err = authority.presentation(c.Presentation); err != nil {
					delete(result, "token")
				}
			}
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
