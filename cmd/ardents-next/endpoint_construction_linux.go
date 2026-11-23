//go:build linux

package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

// runEndpointPlanConstruction is the production plan constructor: it renders
// headless.json and source.json from authenticated accepted-State facts, the
// operator's bounded local declaration and the distributed Source plan
// artifact. It never edits or adopts existing plan bytes.
func runEndpointPlanConstruction(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) != 4 {
		return 2
	}
	for _, path := range args {
		if !absoluteAdmissionPath(path) {
			return 2
		}
	}
	if err := ctx.Err(); err != nil {
		return 1
	}
	ownerRaw, err := readBounded(args[0], 8<<10)
	if err != nil {
		return 1
	}
	var owner networkAuthorityPlan
	if decodeAdmissionObject(ownerRaw, &owner) != nil {
		return 1
	}
	// The established local Network owner projection builds the reference
	// configuration; the constructor itself refuses pins that do not open the
	// accepted retained history.
	config, err := networkStateConfig(&owner)
	if err != nil {
		return 1
	}
	sourceRaw, err := readBounded(args[1], 32<<10)
	if err != nil {
		return 1
	}
	var declared endpoint.SourcePlan
	if decodeAdmissionObject(sourceRaw, &declared) != nil {
		return 1
	}
	declarationRaw, err := readBounded(args[2], 8<<10)
	if err != nil {
		return 1
	}
	var declaration endpoint.PlanDeclaration
	if decodeAdmissionObject(declarationRaw, &declaration) != nil {
		return 1
	}
	if err := endpoint.PreparePlans(ctx, config, declared, declaration, args[3]); err != nil {
		return 1
	}
	if json.NewEncoder(diagnostic).Encode(map[string]string{"phase": "plan-construction", "outcome": "constructed"}) != nil {
		return 1
	}
	return 0
}
