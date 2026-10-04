package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
)

func runAdmissionCurrentIssuer(ctx context.Context, args []string, out io.Writer) int {
	var c struct {
		Plan    issuer.Plan           `json:"plan"`
		Profile string                `json:"profile"`
		Network *networkAuthorityPlan `json:"network,omitempty"`
		Batch   []byte                `json:"batch"`
		Kind    quota.Kind            `json:"kind"`
	}
	if ctx == nil || admissionConfig(args, &c) != nil || !validAdmissionAuthority(c.Profile, c.Network, c.Plan.KeyRoot, c.Plan.AdmissionRoot, c.Plan.ResultRoot) {
		return 2
	}
	authority, err := openAdmissionAuthority(c.Profile, c.Network)
	if err != nil {
		return 1
	}
	r := issuer.IssueCurrent(ctx, c.Plan, c.Batch, c.Kind, func() (admission.AuthorityFacts, time.Time, error) {
		return authority.issuer(c.Plan.KeyBinding.Signer)
	})
	if authority.close() != nil {
		r.Response = nil
		r.Phase, r.Outcome = "close-network", "storage-uncertain"
	}
	phase, outcome := r.Status()
	if json.NewEncoder(out).Encode(map[string]any{"phase": phase, "outcome": outcome, "response": r.Response}) != nil {
		return 2
	}
	if r.Response != nil {
		return 0
	}
	if ctx.Err() != nil {
		return 130
	}
	return 1
}
