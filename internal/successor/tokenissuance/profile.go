package tokenissuance

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

// ProfilePlan fixes separately pinned Node identity and immutable key inventory.
type ProfilePlan struct {
	IdentityRoot    string
	IdentityBinding nodeidentity.Binding
	KeyRoot         string
	KeyBinding      issuance.Binding
	ProfileRoot     string
}

func InitializeProfile(ctx context.Context, p ProfilePlan) Result {
	return executeProfile(ctx, p, true)
}
func InspectProfile(ctx context.Context, p ProfilePlan) Result { return executeProfile(ctx, p, false) }
func executeProfile(ctx context.Context, p ProfilePlan, initialize bool) (r Result) {
	r = Result{Phase: "input", Outcome: "invalid-input"}
	if !issuance.Supported() {
		r.Outcome = "unsupported-platform"
		return r
	}
	if ctx == nil || !(Plan{AdmissionRoot: p.IdentityRoot, KeyRoot: p.KeyRoot, ResultRoot: p.ProfileRoot}).valid() {
		return r
	}
	if ctx.Err() != nil {
		r.Outcome = "canceled"
		return r
	}
	r.Phase = "open-identity"
	identity, err := nodeidentity.Open(ctx, p.IdentityRoot, p.IdentityBinding)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	defer func() {
		if e := identity.Close(); e != nil {
			r = Result{Phase: "close-identity", Outcome: "storage-uncertain"}
		}
	}()
	r.Phase = "open-keys"
	keys, err := issuance.Open(ctx, p.KeyRoot, p.KeyBinding)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	defer func() {
		if e := keys.Close(); e != nil {
			r = Result{Phase: "close-keys", Outcome: "storage-uncertain"}
		}
	}()
	public, err := identity.Public()
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	inventory, err := keys.Inventory()
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	if public.Network != inventory.Binding.Network || public.Node != inventory.Binding.Issuer || public.Signer != inventory.Binding.Signer {
		r.Outcome = "invalid-input"
		return r
	}
	r.Phase = "profile"
	if initialize {
		if err = issuance.InitializeProfile(ctx, p.ProfileRoot, keys, identity); err != nil {
			r.Outcome = category(err)
			return r
		}
	}
	profile, err := issuance.OpenProfile(ctx, p.ProfileRoot, keys)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	defer func() {
		if e := profile.Close(); e != nil {
			r = Result{Phase: "close-profile", Outcome: "storage-uncertain"}
		}
	}()
	r.Response, err = profile.Bytes()
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	r.Outcome = "profile-verified"
	if initialize {
		r.Outcome = "profile-initialized"
	}
	return r
}
