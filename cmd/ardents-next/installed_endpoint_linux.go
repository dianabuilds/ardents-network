//go:build linux

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	installation "github.com/dianabuilds/ardents-network/internal/successor/installation/endpoint"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// The installed participant consumes original startup admission, then composes
// genuine Source, finite Stock and private Service owners. Startup declarations
// never substitute for each operation's qualified worker or current authority.
func runInstalledEndpoint(ctx context.Context, args []string, diagnostic io.Writer) (code int) {
	if len(args) != 2 || args[0] != "start-installed" || ctx == nil {
		return 2
	}
	if ctx.Err() != nil {
		return 1
	}
	phase := "installed-startup"
	defer func() {
		if code != 0 {
			// Local refusal diagnostics contain only the owning stage. Original
			// paths, authority bytes, Targets and error strings stay private.
			_ = json.NewEncoder(diagnostic).Encode(map[string]string{"phase": phase, "outcome": "unavailable"})
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	startup, err := installation.OpenStartup(ctx, args[1])
	if err != nil {
		return 1
	}
	defer func() {
		if startup.Close() != nil {
			code = 1
		}
	}()
	headlessRaw, sourceRaw, err := startup.Plans()
	if err != nil {
		return 1
	}
	phase = "installed-declarations"
	plan, sourcePlan, err := installedDeclarations(headlessRaw, sourceRaw)
	if err != nil {
		return 1
	}
	local, err := installedLocalConfig(plan)
	if err != nil {
		return 1
	}
	owner, err := executionruntime.New(local)
	if err != nil {
		return 1
	}
	defer func() {
		if owner.Close() != nil {
			code = 1
		}
	}()
	if err := startup.Check(); err != nil {
		return 1
	}
	phase = "reader-preparation"
	reader, err := owner.Prepare(ctx, local.Grants[0].Principal, execution.Connection)
	if err != nil {
		return 1
	}
	defer func() {
		if reader.Close() != nil {
			code = 1
		}
	}()
	check := func() error { return errors.Join(startup.Check(), reader.Check(), ctx.Err()) }
	var publisher *executionruntime.Preparation
	if plan.Role == "" {
		phase = "publisher-preparation"
		if err := check(); err != nil {
			return 1
		}
		publisher, err = owner.Prepare(ctx, local.Grants[1].Principal, execution.Administration)
		if err != nil {
			return 1
		}
		defer func() {
			if publisher.Close() != nil {
				code = 1
			}
		}()
		readerCheck := check
		check = func() error { return errors.Join(readerCheck(), publisher.Check()) }
	}
	if err := check(); err != nil {
		return 1
	}
	phase = "source-configuration"
	config, err := installedNetworkConfig(plan, sourcePlan, check)
	if err != nil {
		return 1
	}
	// Source intake has its own State lifetime. Joined preparation provenance
	// gates permission bootstrap only and cannot stand in for a live worker.
	config.PermitWork = startup.Check
	phase = "network-open"
	network, err := state.Open(config)
	if err != nil {
		return 1
	}
	defer func() {
		if network.Close() != nil {
			code = 1
		}
	}()
	phase = "network-refresh"
	if _, err := network.Refresh(ctx); err != nil {
		return 1
	}
	authority := networkAdmissionAuthority(network.CurrentRuntime, network.Close, network.CurrentRuntimeWitness)
	preparationCheck := check
	check = func() error {
		if err := preparationCheck(); err != nil {
			return err
		}
		_, _, err := authority.observe()
		return err
	}
	holders := make(map[string]*stock.Owner)
	phase = "permission-bootstrap"
	for index, permission := range []installedPermission{plan.ReaderPermission, plan.PublisherPermission} {
		if index == 1 && plan.Role == "reader" {
			break
		}
		role, name := admission.AllocationUser, "reader"
		if index == 1 {
			role, name = admission.AllocationPublisher, "publisher"
		}
		if err := check(); err != nil {
			return 1
		}
		// This new composition cannot adopt another owner's token history by
		// hiding its marker beneath fresh child roots.
		entries, err := os.ReadDir(plan.TextTokenRoot)
		if err != nil {
			return 1
		}
		for _, entry := range entries {
			if (entry.Name() != "reader" && entry.Name() != "publisher") || !entry.IsDir() {
				return 1
			}
		}
		root := filepath.Join(plan.TextTokenRoot, name)
		if err := os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return 1
		}
		if err := check(); err != nil {
			return 1
		}
		holder, err := stock.Open(root, role, authority.observe)
		if err != nil {
			return 1
		}
		holders[name] = holder
		defer func() {
			if holder.Close() != nil {
				code = 1
			}
		}()
		originalFacts, _, err := authority.observe()
		if err != nil {
			return 1
		}
		request, digest, err := holder.Request(permission.Maxima)
		if err != nil {
			return 1
		}
		decoded, err := admission.DecodePermissionRequest(request)
		if err != nil {
			return 1
		}
		permissionCheck := func() error {
			if err := check(); err != nil {
				return err
			}
			current, _, err := authority.observe()
			if err != nil || current != originalFacts {
				return errors.Join(errors.New("permission request authority changed"), err)
			}
			return nil
		}
		if err := installedPermissionRequest(permission.RequestPath, request, permissionCheck); err != nil {
			return 1
		}
		if json.NewEncoder(diagnostic).Encode(map[string]string{"phase": "permission-pending", "role": name, "request_digest": hex.EncodeToString(digest[:])}) != nil {
			return 1
		}
		if err := installedPermissionResponse(ctx, permission.ResponsePath, digest, decoded.Permission.NotAfter, holder, permissionCheck); err != nil {
			return 1
		}
		if json.NewEncoder(diagnostic).Encode(map[string]string{"phase": "permission-accepted", "role": name, "request_digest": hex.EncodeToString(digest[:])}) != nil {
			return 1
		}
	}
	// Wait supervises actual finite Source waves under its durable exposure and
	// backoff rules. It is not an idle substitute for a missing Service owner.
	if err := reader.Close(); err != nil {
		return 1
	}
	if publisher != nil {
		if err := publisher.Close(); err != nil {
			return 1
		}
	}
	{
		phase = "service-route"
		// The actual provider budget must already exist; runtime cannot
		// initialize or reset Hosting history to manufacture capacity.
		current := func() error { return errors.Join(startup.Check(), ctx.Err()) }
		originalCurrent := authority.current
		serviceAuthority := networkAdmissionAuthority(func() (networkdomain.RuntimeView, error) {
			// CurrentRuntime already calls this same Startup.Check through
			// config.PermitWork before reading Network state. Retain the caller
			// precheck and the original post-I/O Startup observation, without
			// repeating the full installed observation before that owner call.
			if err := ctx.Err(); err != nil {
				return networkdomain.RuntimeView{}, err
			}
			view, err := originalCurrent()
			return view, errors.Join(err, current())
		}, authority.close, authority.witness)
		readerPlan := routePrefixPlan{EntryRoot: filepath.Join(plan.EntryStateRoot, "entry"), InteriorRoot: filepath.Join(plan.EntryStateRoot, "reader-interior"),
			HostingRoot: filepath.Join(plan.EntryStateRoot, "hosting"), Domain: 1,
			Work:        hosting.Traffic{Tx: admission.ForwardClass.ByteLimit(), Rx: admission.ForwardClass.ByteLimit()},
			Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()}}
		routes, err := openInstalledRouteOwners(readerPlan, serviceAuthority)
		if err != nil {
			return 1
		}
		defer func() {
			cancel()
			if routes.Close() != nil {
				code = 1
			}
		}()
		readerOwner := &installedReader{local: owner, principal: local.Grants[0].Principal,
			authority: serviceAuthority, holder: holders["reader"], check: current, plan: readerPlan, route: routes}
		if plan.Role == "" {
			phase = "administration-listener"
			publisherPlan := readerPlan
			publisherPlan.Domain = 4
			publisherPlan.InteriorRoot = filepath.Join(plan.EntryStateRoot, "introduction-interior")
			publisherPlan.SourceInteriorRoot = filepath.Join(plan.EntryStateRoot, "publisher-source-interior")
			publisherPlan.ResponderInteriorRoot = filepath.Join(plan.EntryStateRoot, "responder-interior")
			publisherContext, publisherCancel := context.WithCancel(ctx)
			publisherOwner := &installedPublisher{ctx: publisherContext, cancel: publisherCancel,
				local: owner, principal: local.Grants[1].Principal,
				plan:      publicationPlan{InstanceRoot: plan.ServiceInstanceRoot, Root: plan.PublicationRoot},
				routePlan: publisherPlan, routes: routes, authority: serviceAuthority,
				holder: holders["publisher"], check: current, failed: cancel}
			defer func() {
				cancel()
				if publisherOwner.Close() != nil {
					code = 1
				}
			}()
			if err := current(); err != nil {
				return 1
			}
			administrationServer, err := administration.Listen(plan.AdministrationSocket, publisherOwner)
			if err != nil {
				return 1
			}
			defer func() {
				cancel()
				if administrationServer.Close() != nil {
					code = 1
				}
			}()
		}
		if err := current(); err != nil {
			return 1
		}
		phase = "reader-listener"
		server, err := application.Listen(plan.ApplicationSocket, readerOwner)
		if err != nil {
			return 1
		}
		defer func() {
			cancel()
			if server.Close() != nil {
				code = 1
			}
		}()
		if err := current(); err != nil {
			return 1
		}
	}
	phase = "running"
	if err := network.Wait(ctx); err != nil {
		return 1
	}
	return 0
}

func installedPermissionResponse(ctx context.Context, path string, digest [32]byte, deadline time.Time, holder *stock.Owner, check func() error) error {
	bounded, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := check(); err != nil {
			return err
		}
		raw, err := installedPermissionRead(path)
		if err == nil {
			if err := check(); err != nil {
				clear(raw)
				return err
			}
			err = holder.Import(digest, raw)
			clear(raw)
			return errors.Join(err, check())
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-bounded.Done():
			return bounded.Err()
		case <-tick.C:
		}
	}
}
