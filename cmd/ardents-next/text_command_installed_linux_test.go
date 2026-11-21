//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"
	endpointpublisher "github.com/dianabuilds/ardents-network/internal/successor/endpoint/publisher"
	endpointreader "github.com/dianabuilds/ardents-network/internal/successor/endpoint/reader"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestInstalledTextCommandsThroughJoinedNetworkTCP(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	t.Run(string(transport.ClosedCarrierTCP), func(t *testing.T) {
		for _, scenario := range []struct {
			name string
			size int
		}{
			{"empty", 0}, {"reference", 64 << 10}, {"maximum", textdocument.MaximumBytes}, {"refresh", 64 << 10},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				runInstalledTextCommandsThroughJoinedNetwork(t, bytes.Repeat([]byte{'x'}, scenario.size), scenario.name == "refresh")
			})
		}
	})
}

func runInstalledTextCommandsThroughJoinedNetwork(t *testing.T, snapshot []byte, refresh bool) {
	t.Helper()
	if refresh && time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)) < 16*time.Minute {
		t.Fatal("invalid environment: installed Text command test needs sixteen minutes in the original token hour")
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
	if err := hosting.Initialize(budgetRoot, hosting.Policy{Provider: "installed Text command test", Start: start, End: start.Add(2 * time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
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
		Plan: publisherPlan(instance), Route: publisherRoute, Authority: authority,
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
	input := filepath.Join(sockets, "document")
	if err := os.WriteFile(input, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	if output := installedTextCLI(t, ctx, nil, 0, "publish", publishSocket, input); len(output) != 0 {
		t.Fatal("publish wrote stdout")
	}
	if refresh {
		awaitInstalledTextCommandRefresh(t, ctx, store, target, f)
	}
	link := installedTextCLI(t, ctx, nil, 514, "link", publishSocket)
	if len(link) < 2 || link[len(link)-1] != '\n' || bytes.Count(link, []byte{'\n'}) != 1 {
		t.Fatal("link did not return one line")
	}
	linkNetwork, linkTarget, err := publication.ParseTargetLink(string(link[:len(link)-1]))
	if err != nil || linkNetwork != f.profile.NetworkID || linkTarget != target {
		t.Fatal("link did not identify the original authenticated Service", err)
	}
	actual := installedTextCLI(t, ctx, link, len(snapshot), "read", readSocket)
	if !bytes.Equal(actual, snapshot) {
		t.Fatalf("read returned different bytes, got length %d, wanted %d", len(actual), len(snapshot))
	}
	outcome, err := administration.Request(ctx, publishSocket, administration.Withdraw)
	if err != nil || outcome != administration.Withdrawn {
		t.Fatal("publication did not withdraw and join", err)
	}
	if _, err := administration.RequestPublishedLink(ctx, publishSocket); err == nil {
		t.Fatal("retired Publisher retained a current Link")
	}
}

func awaitInstalledTextCommandRefresh(t *testing.T, ctx context.Context, store *reachability.Store, target [32]byte, f *networkAdmissionFixture) {
	t.Helper()
	observe := func() reachability.Descriptor {
		raw, outcome, err := store.Lookup(target, f.profile.Digest, time.Now())
		if err != nil || outcome != reachability.Accepted {
			t.Fatal("signed Descriptor missing", err)
		}
		value, err := reachability.Verify(raw, target, f.profile.NetworkID, f.profile.Digest, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := observe()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("automatic Publication refresh missing", ctx.Err())
		case <-ticker.C:
		}
		next := observe()
		if next.Introduction.Revision == first.Introduction.Revision {
			continue
		}
		if next.Introduction.Revision != first.Introduction.Revision+1 || next.Introduction.Slot == first.Introduction.Slot || next.Introduction.RecipientKey == first.Introduction.RecipientKey || next.PublicationDigest != first.PublicationDigest {
			t.Fatal("automatic refresh did not produce one fresh receiving identity for the original Publication")
		}
		// Store commit precedes ACK; verify that the original recipient is
		// retired while its signed proof still verifies and the new one serves.
		overlap := time.NewTimer(62 * time.Second)
		select {
		case <-overlap.C:
		case <-ctx.Done():
			overlap.Stop()
			t.Fatal("original predecessor did not reach the retirement boundary", ctx.Err())
		}
		if err := first.Current(time.Now()); err != nil {
			t.Fatal("original signed Descriptor expired before predecessor refusal", err)
		}
		installedPublisherSubmitNonce(t, f, first, false)
		if current := observe(); current.Introduction.Revision != next.Introduction.Revision || current.Introduction.Slot != next.Introduction.Slot {
			t.Fatal("retiring the predecessor changed the accepting replacement")
		}
		return
	}
}

type installedTextOutput struct {
	body      bytes.Buffer
	remaining int
	cancel    context.CancelFunc
}

func (output *installedTextOutput) Write(body []byte) (int, error) {
	if len(body) > output.remaining {
		output.cancel()
		return 0, io.ErrShortBuffer
	}
	output.remaining -= len(body)
	return output.body.Write(body)
}

func installedTextCLI(t *testing.T, parent context.Context, input []byte, limit int, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 50*time.Second)
	defer cancel()
	stdout, stderr := &installedTextOutput{remaining: limit, cancel: cancel}, &installedTextOutput{remaining: 4096, cancel: cancel}
	command := exec.CommandContext(ctx, "/usr/lib/ardents/text-worker-root/ardents-text", args...)
	command.WaitDelay = 5 * time.Second
	command.Stdin, command.Stdout, command.Stderr = bytes.NewReader(input), stdout, stderr
	if err := errors.Join(command.Run(), ctx.Err()); err != nil || stderr.body.Len() != 0 {
		t.Fatalf("installed Text %s failed: %v, diagnostic %q", args[0], err, stderr.body.String())
	}
	return stdout.body.Bytes()
}
