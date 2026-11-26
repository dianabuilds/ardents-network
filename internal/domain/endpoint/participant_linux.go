//go:build linux

package endpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	endpointlaunch "github.com/dianabuilds/ardents-network/internal/domain/endpoint/launch"
	endpointpublisher "github.com/dianabuilds/ardents-network/internal/domain/endpoint/publisher"
	endpointreader "github.com/dianabuilds/ardents-network/internal/domain/endpoint/reader"
	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	networkdomain "github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

// ErrUnavailable retains an unsuccessful participant operation separately from
// ordinary canceled cleanup after requested shutdown.
var ErrUnavailable = errors.New("installed participant unavailable")

// Progress is a public permission-exchange observation, never readiness or authority.
type Progress struct {
	Phase  string
	Role   string
	Digest [32]byte
}

// Run retains the original installed participant until every IPC, Service and
// root borrower joins. report renders public progress only; it supplies no
// authority. The returned phase contains no paths, keys or remote identities.
func Run(ctx context.Context, root string, report func(Progress) error) (phase string, result error) {
	if ctx == nil || report == nil {
		return "", errors.New("invalid participant input")
	}
	phase = "installed-startup"
	if err := ctx.Err(); err != nil {
		// Pre-start refusal is not requested shutdown of an admitted participant.
		return phase, errors.Join(ErrUnavailable, err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	startup, err := endpointlaunch.OpenStartup(ctx, root)
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	defer func() {
		result = errors.Join(result, startup.Close())
	}()
	headlessRaw, sourceRaw, err := startup.Plans()
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	phase = "installed-declarations"
	plan, sourcePlan, err := Declarations(headlessRaw, sourceRaw)
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	local, err := installedLocalConfig(plan)
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	owner, err := executionruntime.New(local)
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	defer func() {
		result = errors.Join(result, owner.Close())
	}()
	if err := startup.CheckFull(); err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	phase = "reader-preparation"
	reader, err := owner.Prepare(ctx, local.Grants[0].Principal, execution.Connection)
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	defer func() {
		result = errors.Join(result, reader.Close())
	}()
	check := func() error { return errors.Join(startup.CheckFull(), reader.Check(), ctx.Err()) }
	var publisher *executionruntime.Preparation
	if plan.Role == "" {
		phase = "publisher-preparation"
		if err := check(); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		publisher, err = owner.Prepare(ctx, local.Grants[1].Principal, execution.Administration)
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		defer func() {
			result = errors.Join(result, publisher.Close())
		}()
		readerCheck := check
		check = func() error { return errors.Join(readerCheck(), publisher.Check()) }
	}
	if err := check(); err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	phase = "source-configuration"
	config, err := NetworkConfig(plan, sourcePlan, check)
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	// Source intake has its own State lifetime. Joined preparation provenance
	// gates permission bootstrap only and cannot stand in for a live worker.
	config.PermitWork = startup.Check
	phase = "network-open"
	network, err := state.OpenRetained(config)
	if err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	defer func() {
		result = errors.Join(result, network.Close())
	}()
	phase = "network-refresh"
	if _, err := network.Refresh(ctx); err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	authority := funding.Observe(network.CurrentRuntime, network.CurrentRuntimeWitness, funding.PresentationGuard(network.CurrentRuntime))
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := authority.Current()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	preparationCheck := check
	check = func() error {
		if err := preparationCheck(); err != nil {
			return err
		}
		_, _, err := observe()
		return err
	}
	holders := make(map[string]*stock.Owner)
	phase = "permission-bootstrap"
	for index, permission := range []PermissionPlan{plan.ReaderPermission, plan.PublisherPermission} {
		if index == 1 && plan.Role == "reader" {
			break
		}
		role, name := admission.AllocationUser, "reader"
		if index == 1 {
			role, name = admission.AllocationPublisher, "publisher"
		}
		if err := check(); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		// This new composition cannot adopt another owner's token history by
		// hiding its marker beneath fresh child roots.
		entries, err := os.ReadDir(plan.ApplicationTokenRoot)
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		for _, entry := range entries {
			if (entry.Name() != "reader" && entry.Name() != "publisher") || !entry.IsDir() {
				return phase, errors.New("foreign token history")
			}
		}
		root := filepath.Join(plan.ApplicationTokenRoot, name)
		holder, err := openParticipantStock(root, role, observe, check)
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		holders[name] = holder
		defer func() {
			result = errors.Join(result, holder.Close())
		}()
		// A platform-led restart erases the volatile holder; retire the prior
		// invocation's handover so this fresh invocation can publish its own fresh
		// holder request for external authority re-approval. First boot has no
		// handover and this is a no-op; the durable presentation journal is retained.
		if err := installedPermissionRetire(permission.RequestPath, permission.ResponsePath, check); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		originalFacts, _, err := observe()
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		request, digest, err := holder.Request(permission.Maxima)
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		decoded, err := admission.DecodePermissionRequest(request)
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		permissionCheck := func() error {
			if err := check(); err != nil {
				return err
			}
			current, _, err := observe()
			if err != nil {
				return err
			}
			if current != originalFacts {
				return errors.New("permission request authority changed")
			}
			return nil
		}
		if err := installedPermissionRequest(permission.RequestPath, request, permissionCheck); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		if err := report(Progress{Phase: "permission-pending", Role: name, Digest: digest}); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		if err := installedPermissionResponse(ctx, permission.ResponsePath, digest, decoded.Permission.NotAfter, holder, permissionCheck); err != nil {
			return phase, err
		}
		if err := report(Progress{Phase: "permission-accepted", Role: name, Digest: digest}); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
	}
	// Wait supervises actual finite Source waves under its durable exposure and
	// backoff rules. It is not an idle substitute for a missing Service owner.
	if err := reader.Close(); err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	if publisher != nil {
		if err := publisher.Close(); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
	}
	{
		phase = "service-route"
		// The actual provider budget must already exist; runtime cannot
		// initialize or reset Hosting history to manufacture capacity.
		current := func() error { return errors.Join(startup.Check(), ctx.Err()) }
		// Operation admission boundaries use the full-content installed
		// observation; inner effect brackets (current, PermitWork) use the
		// identity-only granularity.
		boundary := func() error { return errors.Join(startup.CheckFull(), ctx.Err()) }
		originalCurrent := authority.Current
		serviceCurrent := func() (networkdomain.RuntimeView, error) {
			// CurrentRuntime already calls this same Startup.Check through
			// config.PermitWork before reading Network state. Retain the caller
			// precheck and the original post-I/O Startup observation, without
			// repeating the full installed observation before that owner call.
			if err := ctx.Err(); err != nil {
				return networkdomain.RuntimeView{}, err
			}
			view, err := originalCurrent()
			return view, errors.Join(err, current())
		}
		serviceAuthority := funding.Observe(serviceCurrent, network.CurrentRuntimeWitness, funding.PresentationGuard(serviceCurrent))
		readerPlan := funding.Plan{EntryRoot: filepath.Join(plan.EntryStateRoot, "entry"), InteriorRoot: filepath.Join(plan.EntryStateRoot, "reader-interior"),
			HostingRoot: filepath.Join(plan.EntryStateRoot, "hosting"), Domain: 1,
			Work:        hosting.Traffic{Tx: admission.ForwardClass.ByteLimit(), Rx: admission.ForwardClass.ByteLimit()},
			Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()}}
		routes, err := funding.OpenParents(readerPlan, serviceAuthority)
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		defer func() {
			cancel()
			result = errors.Join(result, routes.Close())
		}()
		readerOwner := endpointreader.New(endpointreader.Config{Local: owner, Principal: local.Grants[0].Principal,
			Authority: serviceAuthority, Holder: holders["reader"], Check: current, Boundary: boundary, Plan: readerPlan, Parents: routes})
		if plan.Role == "" {
			phase = "administration-listener"
			publisherPlan := readerPlan
			publisherPlan.Domain = 4
			publisherPlan.InteriorRoot = filepath.Join(plan.EntryStateRoot, "introduction-interior")
			publisherPlan.SourceInteriorRoot = filepath.Join(plan.EntryStateRoot, "publisher-source-interior")
			publisherPlan.ResponderInteriorRoot = filepath.Join(plan.EntryStateRoot, "responder-interior")
			publisherOwner := endpointpublisher.New(endpointpublisher.Config{Context: ctx,
				Local: owner, Principal: local.Grants[1].Principal,
				Plan:  endpointpublisher.Plan{InstanceRoot: plan.ServiceInstanceRoot, Root: plan.PublicationRoot},
				Route: publisherPlan, Parents: routes, Authority: serviceAuthority,
				Holder: holders["publisher"], Check: current, Boundary: boundary, Failed: cancel})
			defer func() {
				cancel()
				result = errors.Join(result, publisherOwner.Close())
			}()
			if err := current(); err != nil {
				return phase, errors.Join(ErrUnavailable, err)
			}
			administrationServer, err := administration.Listen(plan.AdministrationSocket, publisherOwner)
			if err != nil {
				return phase, errors.Join(ErrUnavailable, err)
			}
			defer func() {
				cancel()
				result = errors.Join(result, administrationServer.Close())
			}()
		}
		if err := current(); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		phase = "reader-listener"
		server, err := application.Listen(plan.ApplicationSocket, readerOwner)
		if err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
		defer func() {
			cancel()
			result = errors.Join(result, server.Close())
		}()
		if err := current(); err != nil {
			return phase, errors.Join(ErrUnavailable, err)
		}
	}
	phase = "running"
	if err := network.Wait(ctx); err != nil {
		return phase, errors.Join(ErrUnavailable, err)
	}
	return phase, nil
}

// openParticipantStock cannot reconstruct a lost role directory or journal.
// Only explicit first preparation may create these histories.
func openParticipantStock(root string, role admission.AllocationRole, observe func() (admission.AuthorityFacts, time.Time, error), check func() error) (*stock.Owner, error) {
	if checkErr := check(); checkErr != nil {
		return nil, checkErr
	}
	return stock.OpenRetained(root, role, observe)
}
