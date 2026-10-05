package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"

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
	Revision      uint64               `json:"revision,omitzero"`
	Choice        uint8                `json:"choice,omitzero"`
	Join          routeJoinIntent      `json:"join,omitzero"`
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
	if config.Route != nil && config.Route.SourceInteriorRoot != "" && !independentRouteRoots(config.Network.Root, config.Root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.HostingRoot, config.Route.SourceInteriorRoot) {
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
	var routeContext routeJoinContext
	var registration routeRegistration
	var joined net.Conn
	closeRoute := func() error {
		var result error
		if joined != nil {
			result = joined.Close()
		}
		// Prefix closure stops and joins all physical children before returning
		// its reservations. Retrieve the child's retained result before Stock
		// and Network roots may close, including an explicit console close.
		if prefix.close != nil {
			result = errors.Join(result, prefix.close())
		}
		if registration.close != nil {
			result = errors.Join(result, registration.close())
		}
		if routeContext.close != nil {
			result = errors.Join(result, routeContext.close())
		}
		return result
	}
	defer func() {
		if closeRoute() != nil {
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
		case "rendezvous":
			if prefix.recipient == nil {
				err = errors.New("route Rendezvous selection unavailable")
				break
			}
			result["recipient"], err = prefix.recipient(c.Choice)
		case "join-open":
			if prefix.join == nil || joined != nil {
				err = errors.New("route JOIN unavailable")
				break
			}
			joined, err = prefix.join(ctx, c.Join)
		case "join-close":
			if joined == nil {
				err = errors.New("route JOIN absent")
				break
			}
			err = joined.Close()
			if err == nil {
				joined = nil
			}
		case "registration-open":
			if registration.close != nil {
				select {
				case <-registration.done:
					err = errors.New("route registration retired; close to retrieve outcome")
				default:
					err = errors.New("route registration already open")
				}
				break
			}
			if prefix.register == nil || c.Revision == 0 {
				err = errors.New("route registration unavailable")
				break
			}
			registration, err = prefix.register(ctx, c.Revision)
			if err == nil {
				result["slot"] = registration.slot
			}
		case "registration-withdraw":
			if registration.withdraw == nil {
				err = errors.New("route registration absent")
				break
			}
			err = registration.withdraw(ctx)
			// Retain the same handle/result after failed withdrawal; no retry can
			// reacquire its slot or silently replace its original lifetime.
			if err == nil {
				registration = routeRegistration{}
			}
		case "registration-close":
			if registration.close == nil {
				err = errors.New("route registration absent")
				break
			}
			err = registration.close()
			if err == nil {
				registration = routeRegistration{}
			}
		case "prefix-open", "join-prefix-open":
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
			if c.Operation == "join-prefix-open" {
				if routeContext.open == nil {
					routeContext, err = newRouteJoinContext(ctx, *config.Route, authority, o)
				}
				if err == nil {
					prefix, err = routeContext.open(ctx)
				}
			} else {
				if routeContext.close != nil {
					err = errors.New("holder console retains its JOIN context")
				} else {
					prefix, err = startRoutePrefix(ctx, *config.Route, authority, o)
				}
			}
		case "prefix-close":
			if prefix.close == nil {
				err = errors.New("route prefix absent")
				break
			}
			err = prefix.close()
			if err == nil {
				prefix = routeHandle{}
			}
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
			if err := errors.Join(closeRoute(), o.Close()); err != nil {
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
