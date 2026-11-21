package systemd

import (
	"context"
	"errors"
	"os"
)

// Infrastructure observes one of the six fixed service objects. It performs
// only bounded GetAll calls, joins each physical helper and exposes no manager
// mutation, arbitrary unit, bus path, command or environment selection.
// The caller must compare the loaded configuration and its current invocation;
// these detached properties supply neither Release permission nor domain join.
func Infrastructure(ctx context.Context, role string) (Properties, Properties, error) {
	switch role {
	case "source", "forwarding", "issuer", "descriptor", "introduction", "join":
		return properties(ctx, "/org/freedesktop/systemd1/unit/ardents_2d"+role+"_2eservice", "Service")
	default:
		return nil, nil, ErrInput
	}
}

// InfrastructureSiblingsAbsent preserves the fixed native slot exclusion. A
// failed read is not absence: only each original manager's exact NoSuchUnit and
// FileNotFound replies count. The absence-only connection admits no unit/job
// objects, mutation, load or reference; all native users join before return.
func InfrastructureSiblingsAbsent(ctx context.Context, role string) (result error) {
	names, err := platformSiblingNames(role)
	if err != nil || ctx == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	r, err := openManagerReference(ctx, nodeSlotsReply)
	if r != nil {
		defer func() { result = errors.Join(result, r.close()) }()
	}
	if err != nil {
		return err
	}
	if err := r.initializeManagerDeclaration(); err != nil {
		return err
	}
	return r.performOriginal(func(ctx context.Context, r *managerReference) error {
		for _, name := range names {
			if err := nodeNameAbsent(ctx, r, name); err != nil {
				return err
			}
		}
		return nil
	})
}

func platformSiblingNames(role string) ([]string, error) {
	if role == "source" {
		return []string{"ardents-source.socket"}, nil
	}
	switch role {
	case "forwarding", "issuer", "descriptor", "introduction", "join":
	default:
		return nil, ErrInput
	}
	names := make([]string, 0, 9)
	for _, unit := range nodeUnitNames {
		if unit != "ardents-"+role+".service" {
			names = append(names, unit)
		}
		names = append(names, unit[:len(unit)-len("service")]+"socket")
	}
	return names, nil
}
