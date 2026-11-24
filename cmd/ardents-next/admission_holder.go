package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
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
	Introduction  routeRecipient       `json:"introduction,omitzero"`
	Capsule       *routeCapsuleIntent  `json:"capsule,omitempty"`
}

type holderPlan struct {
	Root        string                   `json:"root"`
	Profile     string                   `json:"profile"`
	Network     *networkAuthorityPlan    `json:"network,omitempty"`
	Role        admission.AllocationRole `json:"role"`
	Route       *routePrefixPlan         `json:"route,omitempty"`
	Publication *publicationPlan         `json:"publication,omitempty"`
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
	if ctx == nil || !pathgrammar.Strict(config.Root) || !validAdmissionAuthority(config.Profile, config.Network, config.Root) {
		return 2
	}
	if config.Route != nil && (config.Network == nil || !pathgrammar.DisjointRoots(config.Network.Root, config.Root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.HostingRoot)) {
		return 2
	}
	if config.Route != nil && config.Route.SourceInteriorRoot != "" && !pathgrammar.DisjointRoots(config.Network.Root, config.Root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.HostingRoot, config.Route.SourceInteriorRoot) {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	if config.Publication != nil {
		operation, ok := preparation.(*executionruntime.Operation)
		if !ok || operation.CheckPublisher() != nil || config.Role != admission.AllocationPublisher || config.Route == nil || config.Route.Domain != 4 || !pathgrammar.DisjointRoots(config.Network.Root, config.Root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.SourceInteriorRoot, config.Route.ResponderInteriorRoot, config.Route.HostingRoot, config.Publication.InstanceRoot, config.Publication.Root) {
			return 2
		}
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
		}, authority.close, authority.witness)
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
		_ = json.NewEncoder(diagnostic).Encode(map[string]any{"operation": "admission.holder", "phase": "closed", "exit_code": code})
	}()
	var options session.Config
	if config.Route != nil {
		plan := fundingPlan(*config.Route)
		options.Route = &plan
	}
	if config.Publication != nil {
		plan := publisherPlan(*config.Publication)
		options.Publication = &plan
	}
	var conversation *session.Session
	switch permission := preparation.(type) {
	case nil:
		conversation, err = session.OpenLocal(o, fundingAuthority(authority), options)
	case *executionruntime.Preparation:
		conversation, err = session.OpenPreparation(o, fundingAuthority(authority), options, permission)
	case *executionruntime.Operation:
		conversation, err = session.OpenOperation(o, fundingAuthority(authority), options, permission)
	}
	if err != nil || conversation == nil {
		_ = retainCleanup(o.Close())
		return 1
	}
	defer func() {
		if conversation.Close() != nil {
			code = 1
		}
	}()
	return admissionConsole(ctx, input, out, func(ctx context.Context, raw []byte) (any, bool, error) {
		if err := conversation.Check(); err != nil {
			return nil, true, err
		}
		var c holderCommand
		if err := decodeAdmissionObject(raw, &c); err != nil {
			return nil, false, err
		}
		var err error
		result := map[string]any{"outcome": "completed"}
		switch c.Operation {
		case "publication-open":
			err = conversation.OpenPublication(ctx)
		case "publication-publish":
			result["link"], err = conversation.Publish(ctx)
		case "publication-refresh":
			result["link"], err = conversation.Refresh(ctx)
		case "publication-link":
			result["link"], err = conversation.Link(ctx)
		case "publication-withdraw":
			err = conversation.Withdraw(ctx)
		case "publication-serve":
			err = conversation.Serve(ctx)
		case "descriptor-publish":
			err = conversation.PublishDescriptor(ctx, c.Payload)
		case "capsule-submit":
			if c.Capsule != nil {
				if len(c.Payload) != 0 {
					clear(c.Capsule.Request)
					err = errors.New("route capsule input invalid")
				} else {
					err = conversation.SealSubmitCapsule(ctx, session.Recipient(c.Introduction), session.CapsuleIntent(*c.Capsule))
				}
			} else {
				err = conversation.SubmitCapsule(ctx, session.Recipient(c.Introduction), c.Payload)
			}
		case "descriptor-lookup":
			result["proof"], err = conversation.LookupDescriptor(ctx, c.Target)
		case "issuer-issue":
			err = conversation.Issue(ctx, c.Class, c.Receivers)
		case "rendezvous":
			var recipient session.Recipient
			recipient, err = conversation.Recipient(ctx, c.Choice)
			result["recipient"] = routeRecipient(recipient)
		case "join-open":
			err = conversation.OpenJoin(ctx, session.JoinIntent(c.Join))
		case "join-close":
			err = conversation.CloseJoin(ctx)
		case "registration-open":
			facts, opened := conversation.OpenRegistration(ctx, c.Revision)
			err = opened
			if err == nil {
				result["slot"], result["registration"] = facts.Slot, facts
			}
		case "registration-withdraw":
			err = conversation.WithdrawRegistration(ctx)
		case "registration-close":
			err = conversation.CloseRegistration(ctx)
		case "prefix-open":
			err = conversation.OpenPrefix(ctx)
		case "join-prefix-open":
			err = conversation.OpenJoinPrefix(ctx)
		case "bootstrap-open":
			err = conversation.OpenBootstrap(ctx)
		case "prefix-replenish":
			err = conversation.Replenish(ctx)
		case "prefix-close":
			err = conversation.ClosePrefix(ctx)
		case "bootstrap-close":
			err = conversation.CloseBootstrap(ctx)
		case "request":
			result["request"], result["digest"], err = conversation.Request(ctx, c.Maxima)
		case "import":
			err = conversation.Import(ctx, c.Digest, c.Payload)
		case "begin":
			result["request"], result["deadline"], err = conversation.Begin(ctx, c.Intent)
		case "complete":
			err = conversation.Complete(ctx, c.Payload, c.Failed)
		case "discard":
			err = conversation.Discard(ctx)
		case "take":
			result["token"], err = conversation.Take(ctx, c.Presentation, c.Class)
		case "refill-plan":
			result["receivers"], err = conversation.RefillPlan(ctx, c.Receivers, c.Class, c.RequiresToken)
		case "status":
			result["stock"], err = conversation.Status(ctx)
		case "close":
			if err = conversation.Finish(ctx); err != nil {
				return nil, true, err
			}
			return result, true, nil
		default:
			err = errors.New("unknown holder operation")
			if preparation != nil {
				return nil, false, err
			}
		}
		if errors.Is(err, session.ErrPurpose) {
			return nil, false, err
		}
		if errors.Is(err, session.ErrPermission) {
			return nil, true, err
		}
		if err != nil {
			stage := stock.TransferFailureStage(err)
			switch c.Operation {
			case "prefix-close", "bootstrap-close", "join-close", "registration-close":
				stage = framing.TerminalFailureStage(err)
			}
			return map[string]string{"outcome": "refused", "stage": stage}, false, err
		}
		return result, false, nil
	})
}
