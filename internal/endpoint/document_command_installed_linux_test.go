//go:build linux && document_worker_installed

package endpoint_test

import (
	"bytes"
	"context"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/application/administration"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	endpointpublisher "github.com/dianabuilds/ardents-network/internal/endpoint/publisher"
	endpointreader "github.com/dianabuilds/ardents-network/internal/endpoint/reader"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
)

func TestInstalledDocumentCommandsThroughJoinedNetworkTCP(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	t.Run(string(transport.ClosedCarrierTCP), func(t *testing.T) {
		for _, scenario := range []struct {
			name string
			size int
		}{
			{"empty", 0}, {"reference", 64 << 10}, {"maximum", document.MaximumBytes}, {"refresh", 64 << 10},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				runInstalledDocumentCommandsThroughJoinedNetwork(t, bytes.Repeat([]byte{'x'}, scenario.size), scenario.name == "refresh")
			})
		}
	})
}

func runInstalledDocumentCommandsThroughJoinedNetwork(t *testing.T, snapshot []byte, refresh bool) {
	t.Helper()
	if refresh && time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)) < 16*time.Minute {
		t.Fatal("invalid environment: installed document command test needs sixteen minutes in the original token hour")
	}
	f, store := installedServiceNetwork(t, string(transport.ClosedCarrierTCP))
	instance, target := installedRecoveryInstance(t, f.profile.NetworkID)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	readerPrincipal, publisherPrincipal := [32]byte{2}, [32]byte{3}
	local, err := executionruntime.New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{
		{Principal: readerPrincipal, Surface: execution.Connection}, {Principal: publisherPrincipal, Surface: execution.Administration},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := local.Close(); err != nil {
			t.Error(err)
		}
	}()
	root := t.TempDir()
	budgetRoot := filepath.Join(root, "hosting")
	start := time.Now().UTC().Truncate(time.Hour)
	// Several actual Publisher and Reader reservations coexist on one provider.
	if err := hosting.Initialize(budgetRoot, hosting.Policy{Provider: "installed document command test", Start: start, End: start.Add(2 * time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	plan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "reader"), HostingRoot: budgetRoot, Domain: 1,
		Work:        hosting.Traffic{Tx: admission.ForwardClass.ByteLimit(), Rx: admission.ForwardClass.ByteLimit()},
		Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()},
	}
	publisherRoute := plan
	publisherRoute.Domain = 4
	publisherRoute.InteriorRoot = filepath.Join(root, "introduction")
	publisherRoute.SourceInteriorRoot = filepath.Join(root, "publisher-source")
	publisherRoute.ResponderInteriorRoot = filepath.Join(root, "responder")
	roles := []selection.RoleConfig{
		{InteriorRoot: plan.InteriorRoot, Domain: 1}, {InteriorRoot: publisherRoute.InteriorRoot, Domain: 4},
		{InteriorRoot: publisherRoute.SourceInteriorRoot, Domain: 1}, {InteriorRoot: publisherRoute.ResponderInteriorRoot, Domain: 3},
	}
	if err := os.Mkdir(plan.EntryRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if err := os.Mkdir(role.InteriorRoot, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := selection.InitializeInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: f.current}, roles...); err != nil {
		t.Fatal(err)
	}
	authority := fundingAuthority(f.authority)
	routes, err := funding.OpenParents(plan, authority)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := routes.Close(); err != nil {
			t.Error(err)
		}
	}()
	reader := endpointreader.New(endpointreader.Config{Local: local, Principal: readerPrincipal, Plan: plan, Authority: authority,
		Holder: routePermissionStock(t, f, admission.AllocationUser, [3]uint32{32, 32, 0}), Check: ctx.Err, Parents: routes})
	publisher := endpointpublisher.New(endpointpublisher.Config{Context: ctx, Local: local, Principal: publisherPrincipal,
		Plan: instance, Route: publisherRoute, Authority: authority,
		Holder: routePermissionStock(t, f, admission.AllocationPublisher, [3]uint32{32, 32, 8}), Check: ctx.Err, Failed: cancel, Parents: routes})
	defer func() {
		if err := publisher.Close(); err != nil {
			t.Error(err)
		}
	}()
	sockets := t.TempDir()
	readSocket, publishSocket := filepath.Join(sockets, "read"), filepath.Join(sockets, "publish")
	readerListener, err := application.Listen(readSocket, reader)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := readerListener.Close(); err != nil {
			t.Error(err)
		}
	}()
	publisherListener, err := administration.Listen(publishSocket, publisher)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := publisherListener.Close(); err != nil {
			t.Error(err)
		}
	}()
	var awaitRefresh func()
	if refresh {
		awaitRefresh = func() {
			endpointfixture.VerifyInstalledDocumentRefresh(t, ctx, store, f.profile.NetworkID, f.profile.Digest, target, func(first reachability.Descriptor) {
				installedPublisherSubmitNonce(t, f, first, false)
			})
		}
	}
	endpointfixture.VerifyInstalledDocumentCommands(t, ctx, readSocket, publishSocket, snapshot, f.profile.NetworkID, target, awaitRefresh)
}
