package session

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
)

type LocalCommand struct {
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
	Join          LocalJoinIntent      `json:"join,omitzero"`
	Target        [32]byte             `json:"target,omitzero"`
	Introduction  LocalRecipient       `json:"introduction,omitzero"`
	Capsule       *LocalCapsuleIntent  `json:"capsule,omitempty"`
}

type LocalRecipient struct {
	Node       [32]byte  `json:"node"`
	Generation uint64    `json:"generation"`
	NotAfter   time.Time `json:"not_after"`
}

type LocalJoinIntent struct {
	Choice        uint8     `json:"choice,omitzero"`
	Node          [32]byte  `json:"node"`
	Generation    uint64    `json:"generation"`
	Secret        [32]byte  `json:"secret"`
	Context       [32]byte  `json:"context"`
	Deadline      time.Time `json:"deadline"`
	SetupDeadline time.Time `json:"setup_deadline"`
}

// Local candidate bytes grant no receiving or Connection authority.
type LocalCapsuleIntent struct {
	Slot          [32]byte  `json:"slot"`
	DeliveryNonce [32]byte  `json:"delivery_nonce"`
	Revision      uint64    `json:"revision"`
	Expiry        time.Time `json:"expiry"`
	RecipientKey  [32]byte  `json:"recipient_key"`
	Request       []byte    `json:"request"`
}

// HandleLocalCommand executes one bounded local Session operation.
func (conversation *Session) HandleLocalCommand(ctx context.Context, raw []byte, permissionRequired bool) (any, bool, error) {
	if err := conversation.Check(); err != nil {
		return nil, true, err
	}
	var c LocalCommand
	if err := artifacts.DecodeObject(raw, &c); err != nil {
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
				err = conversation.SealSubmitCapsule(ctx, Recipient(c.Introduction), CapsuleIntent(*c.Capsule))
			}
		} else {
			err = conversation.SubmitCapsule(ctx, Recipient(c.Introduction), c.Payload)
		}
	case "descriptor-lookup":
		result["proof"], err = conversation.LookupDescriptor(ctx, c.Target)
	case "issuer-issue":
		err = conversation.Issue(ctx, c.Class, c.Receivers)
	case "rendezvous":
		var recipient Recipient
		recipient, err = conversation.Recipient(ctx, c.Choice)
		result["recipient"] = LocalRecipient(recipient)
	case "join-open":
		err = conversation.OpenJoin(ctx, JoinIntent(c.Join))
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
		if permissionRequired {
			return nil, false, err
		}
	}
	if errors.Is(err, ErrPurpose) {
		return nil, false, err
	}
	if errors.Is(err, ErrPermission) {
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
}
