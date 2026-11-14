//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/reader"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
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
	worker, err := local.LaunchReader(ctx, principal)
	if err != nil {
		return 1
	}
	defer func() {
		if worker.Close() != nil {
			code = 1
		}
	}()
	operation, err := worker.BeginOperation(ctx)
	if err != nil {
		return 1
	}
	defer operation.Close()
	contract, err := operation.BindStreamContract(512, uint64(textdocument.MaximumBytes+13))
	if err != nil {
		return 1
	}
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
		if err := operation.CheckReader(); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := original()
		return view, errors.Join(err, operation.CheckReader())
	}, authority.close, authority.witness)
	holder, err := stock.Open(plan.Root, plan.Role, authority.observe)
	if err != nil {
		return 1
	}
	defer func() {
		if holder.Close() != nil {
			code = 1
		}
	}()
	source, closeSource, err := reader.OpenSource(operation, fundingPlan(*plan.Route), fundingAuthority(authority), holder, target)
	if err != nil {
		return 1
	}
	defer func() {
		if closeSource() != nil {
			code = 1
		}
	}()
	stream, err := source.OpenStream(contract)
	if err != nil {
		return 1
	}
	transferred, result, err := operation.DeliverReaderStream(contract, stream)
	if !transferred {
		_ = stream.Close()
		operation.RetainCleanup(stream.JoinedCleanup())
	}
	if err != nil || result == nil {
		return 1
	}
	defer result.Close()
	if err := closeSource(); err != nil {
		return 1
	}
	operation.Close()
	if worker.Close() != nil || !worker.CompletedCurrent() || ctx.Err() != nil {
		return 1
	}
	body, err := result.TakeBytes()
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
