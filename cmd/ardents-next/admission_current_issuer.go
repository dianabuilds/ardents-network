package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
)

func runAdmissionCurrentIssuer(ctx context.Context, args []string, out io.Writer) int {
	var c struct {
		Plan    issuer.Plan `json:"plan"`
		Profile string      `json:"profile"`
		Batch   []byte      `json:"batch"`
		Kind    quota.Kind  `json:"kind"`
	}
	if ctx == nil || admissionConfig(args, &c) != nil || !absoluteAdmissionPath(c.Profile) {
		return 2
	}
	r := issuer.IssueCurrent(ctx, c.Plan, c.Batch, c.Kind, admissionObserver(c.Profile))
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
