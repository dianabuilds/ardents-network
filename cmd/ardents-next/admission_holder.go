package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
)

type holderPlan struct {
	Root        string                   `json:"root"`
	Profile     string                   `json:"profile"`
	Network     *networkAuthorityPlan    `json:"network,omitempty"`
	Role        admission.AllocationRole `json:"role"`
	Route       *funding.Declaration     `json:"route,omitempty"`
	Publication *publicationPlan         `json:"publication,omitempty"`
}

func runAdmissionHolder(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) int {
	var config holderPlan
	if admissionConfig(args, &config) != nil {
		return 2
	}
	return runHolderPlan(ctx, config, input, out, diagnostic, nil)
}
func localHolderPlan(config holderPlan) session.LocalHolderPlan {
	var options session.Config
	if config.Route != nil {
		plan := config.Route.Plan()
		options.Route = &plan
	}
	if config.Publication != nil {
		plan := publisherPlan(*config.Publication)
		options.Publication = &plan
	}
	return session.LocalHolderPlan{Root: config.Root, Profile: config.Profile, Network: config.Network, Role: config.Role, Options: options}
}
func runHolderPlan(ctx context.Context, config holderPlan, input io.ReadCloser, out, diagnostic io.Writer, preparation executionruntime.Permission) int {
	plan := localHolderPlan(config)
	return session.RunLocalHolder(ctx, plan.Root, plan.Profile, plan.Network, plan.Role, plan.Options, input, out, diagnostic, preparation)
}
