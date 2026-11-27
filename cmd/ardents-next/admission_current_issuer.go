package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

func runAdmissionCurrentIssuer(ctx context.Context, args []string, out io.Writer) int {
	var c struct {
		Plan    issuer.Plan                 `json:"plan"`
		Profile string                      `json:"profile"`
		Network *state.AuthorityDeclaration `json:"network,omitempty"`
		Batch   []byte                      `json:"batch"`
		Kind    quota.Kind                  `json:"kind"`
	}
	if ctx == nil || admissionConfig(args, &c) != nil {
		return 2
	}
	r, err := funding.IssueCurrent(ctx, c.Plan, c.Profile, c.Network, c.Batch, c.Kind)
	if errors.Is(err, funding.ErrInvalidCurrentIssuer) {
		return 2
	}
	if err != nil {
		return 1
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
