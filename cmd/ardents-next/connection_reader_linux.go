//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/reader"
	"github.com/dianabuilds/ardents-network/internal/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/network"
)

// This ordinary Reader consumes already provisioned finite Stock and a verified
// Target Link. Instance/Authority material is not a Reader input. Qualified
// original workers and genuine Route owners precede every Application effect.
func runExecutionTextRead(ctx context.Context, plan holderPlan, generation, principal [32]byte, target string, out io.Writer) (code int) {
	if ctx == nil || plan.Role != admission.AllocationUser || plan.Network == nil || plan.Route == nil || plan.Route.Domain != 1 ||
		!independentRouteRoots(plan.Network.Root, plan.Root, plan.Route.EntryRoot, plan.Route.InteriorRoot, plan.Route.HostingRoot) {
		return 2
	}
	local, err := executionruntime.New(execution.Config{ID: generation, Grants: []execution.Grant{{Principal: principal, Surface: execution.Connection}}})
	if err != nil {
		return 2
	}
	defer func() {
		if local.Close() != nil {
			code = 1
		}
	}()
	read, err := reader.Begin(ctx, local, principal)
	if err != nil {
		return 1
	}
	defer func() {
		if read.Close() != nil {
			code = 1
		}
	}()
	authority, err := openAdmissionAuthority("", plan.Network)
	if err != nil {
		return 1
	}
	defer func() {
		if authority.close() != nil {
			code = 1
		}
	}()
	original := authority.current
	if original == nil {
		return 1
	}
	authority = networkAdmissionAuthority(func() (network.RuntimeView, error) {
		if err := read.Check(); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := original()
		return view, errors.Join(err, read.Check())
	}, authority.close, authority.witness)
	holder, err := stock.OpenRetained(plan.Root, plan.Role, authority.observe)
	if err != nil {
		return 1
	}
	defer func() {
		if holder.Close() != nil {
			code = 1
		}
	}()
	body, err := read.Read(fundingPlan(*plan.Route), fundingAuthority(authority), holder, target)
	if err != nil {
		return 1
	}
	defer clear(body)
	if json.NewEncoder(out).Encode(struct {
		Outcome  string `json:"outcome"`
		Document string `json:"document"`
	}{"text-received", string(body)}) != nil {
		return 2
	}
	return 0
}
