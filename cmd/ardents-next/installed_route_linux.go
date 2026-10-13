//go:build linux

package main

import (
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// installedRouteOwners retains one installation-wide Entry history and one
// actual provider budget. Request/Publication composition borrows them; only
// the installed participant closes them after all such consumers have joined.
type installedRouteOwners struct {
	entry                  *selection.Installation
	budget                 *hosting.Budget
	entryRoot, hostingRoot string
	once                   sync.Once
	result                 error
}

func openInstalledRouteOwners(plan routePrefixPlan, authority admissionAuthority) (*installedRouteOwners, error) {
	if authority.current == nil || !independentRouteRoots(plan.EntryRoot, plan.HostingRoot) {
		return nil, errors.New("installed Route owners are unavailable")
	}
	entry, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: authority.current, Exclusions: plan.Exclusions})
	if err != nil {
		return nil, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return nil, errors.Join(err, entry.Close())
	}
	return &installedRouteOwners{entry: entry, budget: budget, entryRoot: plan.EntryRoot, hostingRoot: plan.HostingRoot}, nil
}

func (owners *installedRouteOwners) matches(plan routePrefixPlan) bool {
	return owners != nil && owners.entry != nil && owners.budget != nil && owners.entryRoot == plan.EntryRoot && owners.hostingRoot == plan.HostingRoot
}

func (owners *installedRouteOwners) Close() error {
	if owners == nil {
		return nil
	}
	owners.once.Do(func() { owners.result = errors.Join(owners.entry.Close(), owners.budget.Close()) })
	return owners.result
}
