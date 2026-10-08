package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
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
	Target        [32]byte             `json:"target,omitzero"`
}

type holderPlan struct {
	Root    string                   `json:"root"`
	Profile string                   `json:"profile"`
	Network *networkAuthorityPlan    `json:"network,omitempty"`
	Role    admission.AllocationRole `json:"role"`
	Route   *routePrefixPlan         `json:"route,omitempty"`
}

func runAdmissionHolder(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) int {
	var config holderPlan
	if admissionConfig(args, &config) != nil {
		return 2
	}
	return runHolderPlan(ctx, config, input, out, diagnostic, nil)
}

func runHolderPlan(ctx context.Context, config holderPlan, input io.ReadCloser, out, diagnostic io.Writer, preparation executionruntime.Permission) (code int) {
	retainCleanup := func(err error) error {
		if operation, live := preparation.(*executionruntime.Operation); live {
			operation.RetainCleanup(err)
		}
		return err
	}
	if ctx == nil || !absoluteAdmissionPath(config.Root) || !validAdmissionAuthority(config.Profile, config.Network, config.Root) {
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
	if preparation != nil {
		original := authority.current
		if original == nil {
			_ = authority.close()
			return 1
		}
		authority = networkAdmissionAuthority(func() (network.RuntimeView, error) {
			if err := preparation.Check(); err != nil {
				return network.RuntimeView{}, err
			}
			view, err := original()
			return view, errors.Join(err, preparation.Check())
		}, authority.close)
	}
	defer func() {
		if retainCleanup(authority.close()) != nil {
			code = 1
		}
	}()
	o, err := stock.Open(config.Root, config.Role, authority.observe)
	if err != nil {
		return 1
	}
	defer func() {
		if retainCleanup(o.Close()) != nil {
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
	// This console session is the exact local context. Prefix/worker retirement
	// does not clear its private lookup facts; final context closure does.
	lookupHistory := &reachability.History{}
	closeRoute := func() error {
		var result error
		if joined != nil {
			result = retainCleanup(joined.Close())
		}
		// Prefix closure stops and joins all physical children before returning
		// its reservations. Retrieve the child's retained result before Stock
		// and Network roots may close, including an explicit console close.
		if prefix.close != nil {
			result = errors.Join(result, retainCleanup(prefix.close()))
		}
		if registration.close != nil {
			result = errors.Join(result, retainCleanup(registration.close()))
		}
		if routeContext.close != nil {
			result = errors.Join(result, retainCleanup(routeContext.close()))
		}
		result = errors.Join(result, retainCleanup(lookupHistory.Close()))
		return retainCleanup(result)
	}
	defer func() {
		if closeRoute() != nil {
			code = 1
		}
	}()
	return admissionConsole(ctx, input, out, func(ctx context.Context, raw []byte) (any, bool, error) {
		if preparation != nil {
			if err := preparation.Check(); err != nil {
				return nil, true, err
			}
		}
		var c holderCommand
		if err := decodeAdmissionObject(raw, &c); err != nil {
			return nil, false, err
		}
		if _, joined := preparation.(*executionruntime.Preparation); joined && !preparationOperation(c.Operation) {
			return nil, false, errors.New("joined preparation grants only permission bootstrap")
		}
		if _, live := preparation.(*executionruntime.Operation); live && !executionRouteOperation(c.Operation) {
			return nil, false, errors.New("execution holder grants only bounded Route preparation and Control")
		}
		var err error
		result := map[string]any{"outcome": "completed"}
		switch c.Operation {
		case "descriptor-publish":
			if prefix.publishDescriptor == nil {
				err = errors.New("route Descriptor publication unavailable")
				break
			}
			err = prefix.publishDescriptor(ctx, c.Payload)
		case "descriptor-lookup":
			if prefix.lookupDescriptor == nil {
				err = errors.New("route Descriptor lookup unavailable")
				break
			}
			result["proof"], err = prefix.lookupDescriptor(ctx, c.Target, lookupHistory)
		case "issuer-issue":
			if prefix.issue == nil {
				err = errors.New("route issuer exchange unavailable")
				break
			}
			err = prefix.issue(ctx, c.Class, c.Receivers)
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
		case "prefix-open", "join-prefix-open", "bootstrap-open":
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
			} else if c.Operation == "bootstrap-open" {
				if routeContext.close != nil {
					err = errors.New("holder console retains its JOIN context")
				} else {
					prefix, err = startRouteBootstrap(ctx, *config.Route, authority, o)
				}
			} else {
				if routeContext.close != nil {
					err = errors.New("holder console retains its JOIN context")
				} else {
					prefix, err = startRoutePrefix(ctx, *config.Route, authority, o)
				}
			}
		case "prefix-replenish":
			if prefix.replenish == nil {
				err = errors.New("route replenishment absent")
				break
			}
			err = prefix.replenish(ctx)
		case "prefix-close", "bootstrap-close":
			if c.Operation == "bootstrap-close" && !prefix.bootstrap {
				err = errors.New("route bootstrap absent")
				break
			}
			if prefix.close == nil {
				err = errors.New("route prefix absent")
				break
			}
			err = retainCleanup(prefix.close())
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
			if c.Failed {
				// A failed exchange preserves pending bytes without verification
				// or deposit. Its normal terminal transition needs no deposit guard.
				err = attempt.Complete(c.Payload, errors.New("exchange failed"))
			} else {
				// The console caller may retire during signature verification. Keep
				// that original lifetime at the same final deposit boundary as Route.
				err = attempt.CompleteBound(c.Payload, nil, ctx.Err)
			}
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
			if err := errors.Join(closeRoute(), retainCleanup(o.Close())); err != nil {
				return nil, true, err
			}
			if preparation != nil {
				if err := preparation.Check(); err != nil {
					return nil, true, err
				}
			}
			return result, true, nil
		default:
			err = errors.New("unknown holder operation")
		}
		if err != nil {
			stage := stock.TransferFailureStage(err)
			switch c.Operation {
			case "prefix-close", "bootstrap-close", "join-close", "registration-close":
				stage = framing.TerminalFailureStage(err)
			}
			return map[string]string{"outcome": "refused", "stage": stage}, false, err
		}
		if preparation != nil {
			if err := preparation.Check(); err != nil {
				return nil, true, err
			}
		}
		return result, false, nil
	})
}

// Completed launch provenance supports only the original permission/bootstrap
// purpose. JOIN, application byte effects and publication need a live Job and
// genuine neighboring owners; a joined worker's Grant cannot authorize them.
func preparationOperation(operation string) bool {
	switch operation {
	case "request", "import", "bootstrap-open", "bootstrap-close", "issuer-issue", "status", "close":
		return true
	}
	return false
}

// This live invocation consumer drives only genuine protected setup and
// Control. Private Publication/Connection and their Service effects remain
// unavailable even while a qualified worker is live.
func executionRouteOperation(operation string) bool {
	switch operation {
	case "request", "import", "bootstrap-open", "bootstrap-close", "issuer-issue",
		"prefix-open", "prefix-replenish", "prefix-close", "descriptor-lookup",
		"begin", "complete", "discard", "take", "refill-plan", "status", "close":
		return true
	default:
		return false
	}
}
