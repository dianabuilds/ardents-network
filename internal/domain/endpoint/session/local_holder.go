package session

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

// localHolder retains one Session and its original local authority owner.
type localHolder struct {
	Session    *Session
	authority  funding.AdmissionAuthority
	permission executionruntime.Permission
}

func retainLocalCleanup(permission executionruntime.Permission, err error) error {
	if operation, ok := permission.(*executionruntime.Operation); ok {
		operation.RetainCleanup(err)
	}
	return err
}

// openLocalHolder acquires the original authority and Stock and transfers Stock
// to the selected Session. The authority remains owned until closeAuthority.
func openLocalHolder(ctx context.Context, root, profile string, networkPlan *state.AuthorityDeclaration, role admission.AllocationRole, config Config, permission executionruntime.Permission) (*localHolder, error) {
	if err := ValidateLocalConfiguration(ctx, root, profile, networkPlan, role, config, permission); err != nil {
		return nil, err
	}
	authority, err := funding.OpenAdmissionAuthority(profile, networkPlan)
	if err != nil {
		return nil, err
	}
	if permission != nil {
		original := authority.Current
		if original == nil {
			_ = authority.Close()
			return nil, errors.New("live Network authority unavailable")
		}
		authority = funding.NetworkAdmissionAuthority(func() (network.RuntimeView, error) {
			if err := permission.Check(); err != nil {
				return network.RuntimeView{}, err
			}
			view, err := original()
			return view, errors.Join(err, permission.Check())
		}, authority.Close, authority.Witness)
	}
	holder, err := stock.Open(root, role, authority.ObserveFacts)
	if err != nil {
		return nil, errors.Join(err, retainLocalCleanup(permission, authority.Close()))
	}
	conversation, err := OpenWithPermission(holder, funding.Observe(authority.Current, authority.Witness, authority.Presentation), config, permission)
	if err != nil || conversation == nil {
		return &localHolder{authority: authority, permission: permission}, errors.Join(errors.New("holder session unavailable"), err, retainLocalCleanup(permission, holder.Close()))
	}
	return &localHolder{Session: conversation, authority: authority, permission: permission}, nil
}

// closeAuthority returns the retained authority only after Session.Close joins.
func (holder *localHolder) closeAuthority() error {
	return retainLocalCleanup(holder.permission, holder.authority.Close())
}
