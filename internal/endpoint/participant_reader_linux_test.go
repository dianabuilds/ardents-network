//go:build linux

package endpoint

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestReaderParticipantRequiresOnlyItsOwnInputs(t *testing.T) {
	root := t.TempDir()
	path := func(name string) string { return filepath.Join(root, name) }
	config := ClosedParticipantConfig{ReaderOnly: true,
		Network:   state.Config{Root: path("state"), NetworkID: [32]byte{1}, AcceptedProfile: carrier.ClosedRouteProfile},
		EntryRoot: path("entry"), LocalRoleRoot: path("roles"), TokenRoot: path("tokens"), ApplicationAddress: path("reader.sock"),
		BrokerID: [32]byte{2}, ConnectionPrincipal: [32]byte{3},
		ReaderPermission: PermissionFiles{RequestPath: path("request"), ResponsePath: path("response"), Maxima: [3]uint32{1}},
		Observe:          func(context.Context, ClosedParticipantEvent) error { return nil },
	}
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ClosedParticipantConfig){
		func(c *ClosedParticipantConfig) { c.AdministrationPrincipal = [32]byte{4} },
		func(c *ClosedParticipantConfig) { c.ServiceInstanceRoot = path("instance") },
		func(c *ClosedParticipantConfig) { c.PublicationRoot = path("publication") },
		func(c *ClosedParticipantConfig) { c.AdministrationAddress = path("admin.sock") },
		func(c *ClosedParticipantConfig) { c.PublisherPermission.Maxima = [3]uint32{1} },
		func(c *ClosedParticipantConfig) { c.ReaderPermission = PermissionFiles{} },
		func(c *ClosedParticipantConfig) { c.Network.Root = "" },
	} {
		mixed := config
		change(&mixed)
		if err := mixed.validate(); err == nil {
			t.Fatal("mixed or incomplete Reader config accepted")
		}
	}
	owner, err := newEndpoint(setup{NetworkID: config.Network.NetworkID, BrokerID: config.BrokerID, ConnectionPrincipal: config.ConnectionPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if owner.publications != nil || owner.publisherBinding != nil {
		t.Fatal("Reader opened Publisher owner")
	}
	if _, err := owner.Admit(config.ConnectionPrincipal, broker.Connection); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Admit(config.ConnectionPrincipal, broker.Administration); err == nil {
		t.Fatal("Reader admitted Administration")
	}
}
