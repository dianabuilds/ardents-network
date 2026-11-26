package funding

import (
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/route/selection"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

// Parents retains one installation-wide Entry history and one
// actual provider budget. Request/Publication composition borrows them; only
// the installed participant closes them after all such consumers have joined.
type Parents struct {
	entry                  *selection.Installation
	budget                 *hosting.Budget
	entryRoot, hostingRoot string
	once                   sync.Once
	result                 error
}

func OpenParents(plan Plan, authority Authority) (*Parents, error) {
	if authority.current == nil || !pathcheck.DisjointRoots(plan.EntryRoot, plan.HostingRoot) {
		return nil, errors.New("installed Route owners are unavailable")
	}
	entry, err := selection.OpenRetainedInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: authority.current, Exclusions: plan.Exclusions})
	if err != nil {
		return nil, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return nil, errors.Join(err, entry.Close())
	}
	return &Parents{entry: entry, budget: budget, entryRoot: plan.EntryRoot, hostingRoot: plan.HostingRoot}, nil
}

func (owners *Parents) Matches(plan Plan) bool {
	return owners != nil && owners.entry != nil && owners.budget != nil && owners.entryRoot == plan.EntryRoot && owners.hostingRoot == plan.HostingRoot
}

func (owners *Parents) Close() error {
	if owners == nil {
		return nil
	}
	owners.once.Do(func() { owners.result = errors.Join(owners.entry.Close(), owners.budget.Close()) })
	return owners.result
}

// Entry borrows the original installation-wide selection owner. The caller
// joins all of its role selections before Parents.Close.
func (owners *Parents) Entry() *selection.Installation { return owners.entry }

// Budget borrows the one original provider budget. It is not independently
// reopened by each Reader or Publisher; all borrowers join before Close.
func (owners *Parents) Budget() *hosting.Budget { return owners.budget }
