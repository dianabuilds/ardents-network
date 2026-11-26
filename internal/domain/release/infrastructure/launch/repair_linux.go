package launch

import (
	"context"
	"errors"
	"io"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/node"
	"github.com/dianabuilds/ardents-network/internal/network/source/serving"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// repair observes the original public declaration and manager for one retained
// duty. Only State can clear an admitted original producer fence.
func repair(ctx context.Context, role, path string) (result error) {
	if ctx == nil || ctx.Err() != nil {
		return artifacts.ErrObservedInput
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	inputs := artifacts.NewObservedInputs(bounded, nil)
	defer func() { result = errors.Join(result, inputs.Close()) }()
	file, err := inputs.Acquire(path, 16<<10, false)
	if err != nil {
		return err
	}
	info := file.Info()
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || info.Mode().Perm()&0022 != 0 {
		return artifacts.ErrObservedInput
	}
	var network *state.AuthorityDeclaration
	var roots []string
	if role == "source" {
		plan, err := servingdeclaration.DecodeServingDeclaration(file.Bytes())
		if err != nil || !serving.ValidDeclaration(plan) {
			return artifacts.ErrObservedInput
		}
		network = (*state.AuthorityDeclaration)(plan.Network)
		roots = []string{network.Root, plan.ParticipationRoot}
	} else {
		plan, declaredRoots, err := node.RepairDeclaration(role, file.Bytes())
		pins := (*state.AuthorityDeclaration)(&plan)
		if err != nil || !pins.ValidShape() {
			return artifacts.ErrObservedInput
		}
		network, roots = pins, declaredRoots
	}
	if err := inputs.Check(); err != nil {
		return err
	}
	config, err := network.Config()
	if err != nil {
		return err
	}
	config.LocalRoleStateRoot = roots[1]
	config.PermitWork = inputs.Check
	switch role {
	case "source":
		return withRepairManager(bounded, systemd.OpenSourceRuntime, func(manager *systemd.SourceRuntime) error {
			return state.RepairRetainedSourceProducer(bounded, config, manager, path)
		})
	case "forwarding":
		return withRepairManager(bounded, systemd.OpenForwardingRuntime, func(manager *systemd.ForwardingRuntime) error {
			return state.RepairRetainedForwardingProducer(bounded, config, manager, path, roots)
		})
	case "join":
		return withRepairManager(bounded, systemd.OpenJoinRuntime, func(manager *systemd.JoinRuntime) error {
			return state.RepairRetainedJoinProducer(bounded, config, manager, path, roots)
		})
	case "descriptor":
		return withRepairManager(bounded, systemd.OpenDescriptorRuntime, func(manager *systemd.DescriptorRuntime) error {
			return state.RepairRetainedDescriptorProducer(bounded, config, manager, path, roots)
		})
	case "introduction":
		return withRepairManager(bounded, systemd.OpenIntroductionRuntime, func(manager *systemd.IntroductionRuntime) error {
			return state.RepairRetainedIntroductionProducer(bounded, config, manager, path, roots)
		})
	}
	return artifacts.ErrObservedInput
}

func withRepairManager[T any, P interface {
	*T
	Close() error
}](ctx context.Context, open func(context.Context) (P, error), repair func(P) error) (result error) {
	manager, err := open(ctx)
	if manager != nil {
		defer func() { result = errors.Join(result, manager.Close()) }()
	}
	if err != nil {
		return err
	}
	return repair(manager)
}

// RunRepair adapts retained recovery to the command writers and exit status.
func RunRepair(ctx context.Context, role string, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || ctx.Err() != nil || len(args) != 1 {
		return 2
	}
	if err := repair(ctx, role, args[0]); err != nil {
		if serving.WriteFailureReport(diagnostic, "repair", &serving.Terminal{FirstOwner: "state", First: err}, nil) != nil {
			return 2
		}
		return 1
	}
	if _, err := io.WriteString(out, "repaired\n"); err != nil {
		return 2
	}
	return 0
}
