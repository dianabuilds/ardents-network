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

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/installation"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// The installed participant owns genuine Source refresh and qualified offline
// permission bootstrap. Private Publication/Connection are still separate
// dependencies; startup never projects their absent readiness as success.
func runInstalledEndpoint(ctx context.Context, args []string, diagnostic io.Writer) (code int) {
	if len(args) != 2 || args[0] != "start-installed" || ctx == nil {
		return 2
	}
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
	config, err := installedNetworkConfig(plan, sourcePlan, check)
	if err != nil {
		return 1
	}
	// Source intake has its own State lifetime. Joined preparation provenance
	// gates permission bootstrap only and cannot stand in for a live worker.
	config.PermitWork = startup.Check
	network, err := state.Open(config)
	if err != nil {
		return 1
	}
	defer func() {
		if network.Close() != nil {
			code = 1
		}
	}()
	if _, err := network.Refresh(ctx); err != nil {
		return 1
	}
	authority := networkAdmissionAuthority(network.CurrentRuntime, network.Close)
	preparationCheck := check
	check = func() error {
		if err := preparationCheck(); err != nil {
			return err
		}
		_, _, err := authority.observe()
		return err
	}
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
