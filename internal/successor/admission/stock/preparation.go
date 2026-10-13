package stock

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
)

// Preparation excludes unrelated issuance across one original stock/opening
// sequence. It grants no permission, transport, token or worker authority.
type Preparation struct {
	self  *Preparation
	owner *Owner
	ctx   context.Context
}

func (o *Owner) Prepare(ctx context.Context) (*Preparation, error) {
	if o == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("stock preparation unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.failure != nil || o.preparing != nil || o.active != nil {
		return nil, errors.New("stock preparation unavailable")
	}
	p := &Preparation{owner: o, ctx: ctx}
	p.self = p
	o.preparing = p
	return p, nil
}

func (p *Preparation) Begin(intent IssuanceIntent) (Attempt, error) {
	if p == nil || p.self != p || p.owner == nil {
		return Attempt{}, errors.New("original stock preparation unavailable")
	}
	return p.owner.begin(intent, p)
}

// Missing returns only the ordered deficit for these requested challenges.
// The fresh observation is outside stock state; Begin/Take still independently
// validate current authority and irreversible effects.
func (p *Preparation) Missing(challenges []token.ClosedTokenContext) ([]token.ClosedTokenContext, error) {
	if p == nil || p.self != p || p.owner == nil || p.ctx.Err() != nil || len(challenges) == 0 || len(challenges) > token.MaximumBatchTokens {
		return nil, errors.New("stock preparation unavailable")
	}
	o := p.owner
	if err := o.beginUse(); err != nil {
		return nil, err
	}
	defer o.users.Done()
	profile, now, err := o.observe()
	if err != nil || profile.ValidateAt(now) != nil {
		return nil, errors.Join(errors.New("stock authority unavailable"), err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.failure != nil || o.preparing != p || p.ctx.Err() != nil || now.Before(o.floor) || !o.permission.CurrentFor(profile, now) {
		return nil, errors.New("original stock preparation retired")
	}
	o.floor = now
	requested := make(map[token.ClosedTokenContext]int)
	var missing []token.ClosedTokenContext
	for _, challenge := range challenges {
		if challenge.NetworkID != profile.NetworkID || challenge.ProfileDigest != profile.Digest || challenge.IssuerNodeID != profile.IssuerNodeID ||
			challenge.ReceiverNodeID == [32]byte{} || challenge.ReceiverDutyGeneration == 0 || challenge.Class < 1 || challenge.Class > 3 ||
			challenge.WindowStart != o.permission.accepted.NotBefore {
			return nil, errors.New("stock challenge binding unavailable")
		}
		requested[challenge]++
		if requested[challenge] > o.permission.StockCountForDuty(challenge.ProfileDigest, challenge.ReceiverNodeID, challenge.ReceiverDutyGeneration, challenge.Class) {
			missing = append(missing, challenge)
		}
	}
	return missing, nil
}

// Close returns only coordination after the caller's original exchange joins.
// It never cancels/completes an in-flight batch or refunds its quota.
func (p *Preparation) Close() error {
	if p == nil {
		return nil
	}
	if p.self != p || p.owner == nil {
		return errors.New("original stock preparation unavailable")
	}
	o := p.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.preparing != p {
		return nil
	}
	if o.active != nil {
		return errors.New("original stock exchange has not joined")
	}
	o.preparing = nil
	return nil
}
