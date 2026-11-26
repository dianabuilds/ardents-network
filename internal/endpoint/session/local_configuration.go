package session

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

var (
	ErrInvalidLocalConfiguration  = errors.New("invalid holder configuration")
	ErrLocalConfigurationCanceled = errors.New("holder configuration canceled")
)

// ValidateLocalConfiguration checks the original roots and Publisher permission
// before opening any local authority or retained Stock.
func ValidateLocalConfiguration(ctx context.Context, root, profile string, network *state.AuthorityDeclaration, role admission.AllocationRole, config Config, permission executionruntime.Permission) error {
	if ctx == nil || !pathgrammar.Strict(root) || !funding.ValidAdmissionAuthority(profile, network, root) {
		return ErrInvalidLocalConfiguration
	}
	if config.Route != nil && (network == nil || !pathgrammar.DisjointRoots(network.Root, root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.HostingRoot)) {
		return ErrInvalidLocalConfiguration
	}
	if config.Route != nil && config.Route.SourceInteriorRoot != "" && !pathgrammar.DisjointRoots(network.Root, root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.HostingRoot, config.Route.SourceInteriorRoot) {
		return ErrInvalidLocalConfiguration
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(ErrLocalConfigurationCanceled, err)
	}
	if config.Publication != nil {
		operation, ok := permission.(*executionruntime.Operation)
		if !ok || operation.CheckPublisher() != nil || role != admission.AllocationPublisher || config.Route == nil || config.Route.Domain != 4 || !pathgrammar.DisjointRoots(network.Root, root, config.Route.EntryRoot, config.Route.InteriorRoot, config.Route.SourceInteriorRoot, config.Route.ResponderInteriorRoot, config.Route.HostingRoot, config.Publication.InstanceRoot, config.Publication.Root) {
			return ErrInvalidLocalConfiguration
		}
	}
	return nil
}

// OpenWithPermission takes Stock on success and preserves the selected original
// local, preparation or live Execution lifetime.
func OpenWithPermission(holder *stock.Owner, authority funding.Authority, config Config, permission executionruntime.Permission) (*Session, error) {
	switch value := permission.(type) {
	case nil:
		return OpenLocal(holder, authority, config)
	case *executionruntime.Preparation:
		return OpenPreparation(holder, authority, config, value)
	case *executionruntime.Operation:
		return OpenOperation(holder, authority, config, value)
	default:
		return nil, ErrPermission
	}
}
