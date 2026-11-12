//go:build linux

package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// JoinAuthorization retains only a genuine exact Node/JOIN candidate pair.
// Its zero value grants nothing. Source/Forwarding proofs, copied receipts and
// public descriptor facts cannot construct it. Native installation and startup
// still require separate original admission; this proof has no initial pin.
type JoinAuthorization struct{ pair Authorization }

func (a JoinAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}

// AuthenticateJoinCandidate consumes complete retained Release floors and one
// immutable JOIN inventory. Canonical Control uses it to verify the exact
// predecessor before forward signing; it grants no installed transition.
func AuthenticateJoinCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) (JoinAuthorization, error) {
	f, ok := c.Facts()
	if !ok || f.Invocation != "join" || ctx == nil || v == nil || f.Platform != "linux-amd64" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return JoinAuthorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return JoinAuthorization{}, err
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return JoinAuthorization{}, err
	}
	if !floors.Complete() {
		return JoinAuthorization{}, release.ErrTrustUnavailable
	}
	if err := c.Check(); err != nil {
		return JoinAuthorization{}, err
	}
	program, _ := c.File(ProgramName)
	template, _ := c.File(JoinTemplateName)
	descriptor, _ := c.File(JoinDescriptorName)
	root, _ := c.File("root.json")
	d, err := ValidateJoinResources(descriptor, program, template)
	if err != nil {
		return JoinAuthorization{}, err
	}
	a, err := authenticateBoundPair(ctx, v, c, f, in, d, program, template, descriptor, root, JoinGenerationTarget)
	if err != nil {
		return JoinAuthorization{}, err
	}
	return JoinAuthorization{pair: a}, nil
}
