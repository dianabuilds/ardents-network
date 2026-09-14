package main

import (
	"errors"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// hostingInitializationPlan is a one-time operator declaration for a single
// provider period. Node plans reopen this root but cannot replace its policy.
type hostingInitializationPlan struct {
	Schema string                 `json:"schema"`
	Root   string                 `json:"root"`
	Policy resource.HostingPolicy `json:"policy"`
}

func runHosting(arguments []string) error {
	if len(arguments) != 3 || arguments[0] != "initialize" || arguments[1] != "--config" {
		return errors.New("usage: ardents-node hosting initialize --config PATH")
	}
	var plan hostingInitializationPlan
	if err := decodeOperatorInput(arguments[2], 16<<10, &plan); err != nil {
		return err
	}
	if plan.Schema != "ardents-hosting-initialization-v1" || !filepath.IsAbs(plan.Root) || filepath.Clean(plan.Root) != plan.Root {
		return errors.New("hosting initialization plan is invalid")
	}
	return resource.InitializeHosting(plan.Root, plan.Policy)
}
