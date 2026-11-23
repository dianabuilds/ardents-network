package plan_test

import (
	"strings"
	"testing"

	requestinput "github.com/dianabuilds/ardents-network/internal/endpoint/plan"
)

func endpointDeclFixture() requestinput.Declaration {
	return requestinput.Declaration{
		BundleRoot:       "/var/lib/ardents/artifacts/release-one",
		InstallationRoot: "/var/lib/ardents/endpoint",
		ReleaseFloorRoot: "/var/lib/ardents/release-trust",
		Headless: requestinput.Headless{
			Role:                 "",
			NetworkStateRoot:     "/var/lib/ardents/roles/endpoint/state",
			EntryStateRoot:       "/var/lib/ardents/roles/endpoint/entry",
			LocalRoleStateRoot:   "/var/lib/ardents/roles/endpoint/participation",
			ApplicationTokenRoot: "/var/lib/ardents/roles/endpoint/token",
			PublicationRoot:      "/var/lib/ardents/roles/endpoint/publication",
			ServiceInstanceRoot:  "/var/lib/ardents/roles/endpoint/instance",
			ApplicationSocket:    "/run/ardents/endpoint/application.sock",
			AdministrationSocket: "/run/ardents/endpoint/administration.sock",
			ReaderPermission: requestinput.Permission{
				RequestPath:  "/run/ardents/endpoint/reader-request",
				ResponsePath: "/run/ardents/endpoint/reader-response",
			},
			PublisherPermission: requestinput.Permission{
				RequestPath:  "/run/ardents/endpoint/publisher-request",
				ResponsePath: "/run/ardents/endpoint/publisher-response",
			},
		},
		Source: requestinput.Source{
			ClientCertificate: "/etc/ardents/client.crt",
			ClientKey:         "/etc/ardents/client.key",
		},
	}
}

func endpointLaunchFixture() requestinput.PlatformLaunch {
	return requestinput.PlatformLaunch{
		Role:          "endpoint",
		Distribution:  "/var/lib/ardents/artifacts/release-one",
		ReleaseRoot:   "/var/lib/ardents/release-trust",
		Environment:   "closed",
		Network:       "private",
		Program:       "/var/lib/ardents/endpoint/generations/" + strings.Repeat("a", 64) + "/ardents-linux-amd64",
		Configuration: "/etc/ardents/endpoint.json",
		ConfigSHA256:  strings.Repeat("a", 64),
		WritableRoots: []string{"/var/lib/ardents/roles/endpoint/state", "/var/lib/ardents/roles/endpoint/entry"},
		Inputs: map[string]string{
			"/etc/ardents/client.crt": strings.Repeat("c", 64),
			"/etc/ardents/client.key": strings.Repeat("d", 64),
		},
	}
}

func TestValidEndpointPlatformLaunchRejectsExpansionAndWritableTrust(t *testing.T) {
	for name, change := range map[string]func(*requestinput.PlatformLaunch){
		"role-escape":    func(p *requestinput.PlatformLaunch) { p.Role = "other" },
		"program-layout": func(p *requestinput.PlatformLaunch) { p.Program = "/usr/lib/ardents/ardents-linux-amd64" },
		"program-base": func(p *requestinput.PlatformLaunch) {
			p.Program = "/var/lib/ardents/endpoint/generations/" + strings.Repeat("a", 64) + "/ardents-node-linux-amd64"
		},
		"specifier":     func(p *requestinput.PlatformLaunch) { p.Program += "%i" },
		"configuration": func(p *requestinput.PlatformLaunch) { p.Configuration = "/var/lib/ardents/endpoint.json" },
		"newline":       func(p *requestinput.PlatformLaunch) { p.Distribution += "\nExecStart=/bin/sh" },
		"trust":         func(p *requestinput.PlatformLaunch) { p.ReleaseRoot = p.WritableRoots[0] },
		"artifact":      func(p *requestinput.PlatformLaunch) { p.Distribution = p.WritableRoots[0] },
		"program-trust": func(p *requestinput.PlatformLaunch) { p.WritableRoots[0] = "/var/lib/ardents/endpoint/generations" },
		"overlap":       func(p *requestinput.PlatformLaunch) { p.WritableRoots[1] = p.WritableRoots[0] + "/child" },
		"pin":           func(p *requestinput.PlatformLaunch) { p.ConfigSHA256 = strings.Repeat("z", 64) },
		"input-count":   func(p *requestinput.PlatformLaunch) { delete(p.Inputs, "/etc/ardents/client.key") },
		"mutable-input": func(p *requestinput.PlatformLaunch) {
			delete(p.Inputs, "/etc/ardents/client.key")
			p.Inputs[p.WritableRoots[0]+"/client.key"] = strings.Repeat("d", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := endpointLaunchFixture()
			if !p.Valid() {
				t.Fatal("healthy endpoint declaration refused")
			}
			change(&p)
			if p.Valid() {
				t.Fatal("unsafe endpoint platform declaration accepted")
			}
		})
	}
}

func TestPlatformEndpointRootsProjectsMutableRootsAndSocketParents(t *testing.T) {
	decl := endpointDeclFixture()
	roots, err := requestinput.PlatformWritableRoots(decl)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/run/ardents/endpoint",
		"/var/lib/ardents/roles/endpoint/entry",
		"/var/lib/ardents/roles/endpoint/instance",
		"/var/lib/ardents/roles/endpoint/participation",
		"/var/lib/ardents/roles/endpoint/publication",
		"/var/lib/ardents/roles/endpoint/state",
		"/var/lib/ardents/roles/endpoint/token",
	}
	if len(roots) != len(want) {
		t.Fatalf("unexpected root projection: %v", roots)
	}
	for index := range want {
		if roots[index] != want[index] {
			t.Fatalf("root %d = %q, want %q (%v)", index, roots[index], want[index], roots)
		}
	}
	// Reader-only drops Publication/Instance and the administration/publisher parents.
	decl.Headless.Role = "reader"
	roots, err = requestinput.PlatformWritableRoots(decl)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		if strings.HasSuffix(root, "/publication") || strings.HasSuffix(root, "/instance") {
			t.Fatalf("reader-only projection retained publisher root %q", root)
		}
	}
}

func TestPlatformEndpointRootsRefusesImmutableOverlapAndInvalidPaths(t *testing.T) {
	for name, change := range map[string]func(*requestinput.Declaration){
		"installation-overlap": func(d *requestinput.Declaration) { d.Headless.NetworkStateRoot = d.InstallationRoot + "/state" },
		"bundle-overlap":       func(d *requestinput.Declaration) { d.Headless.EntryStateRoot = d.BundleRoot },
		"release-overlap":      func(d *requestinput.Declaration) { d.Headless.ApplicationTokenRoot = d.ReleaseFloorRoot + "/token" },
		"relative-root":        func(d *requestinput.Declaration) { d.Headless.NetworkStateRoot = "relative/state" },
		"root-escape":          func(d *requestinput.Declaration) { d.Headless.ApplicationSocket = "/application.sock" },
		"newline":              func(d *requestinput.Declaration) { d.Headless.EntryStateRoot += "\nExecStart=/bin/sh" },
	} {
		t.Run(name, func(t *testing.T) {
			decl := endpointDeclFixture()
			change(&decl)
			if _, err := requestinput.PlatformWritableRoots(decl); err == nil {
				t.Fatal("unsafe endpoint write allowance accepted")
			}
		})
	}
}

func TestPlatformEndpointInputsBindsReadOnlyCredentialPair(t *testing.T) {
	decl := endpointDeclFixture()
	inputs, err := requestinput.PlatformSourceCredentials(decl)
	if err != nil || len(inputs) != 2 || inputs[0] != "/etc/ardents/client.crt" || inputs[1] != "/etc/ardents/client.key" {
		t.Fatal("credential pair not bound exactly", inputs, err)
	}
	for name, change := range map[string]func(*requestinput.Declaration){
		"mutable-cert": func(d *requestinput.Declaration) {
			d.Source.ClientCertificate = "/var/lib/ardents/roles/endpoint/client.crt"
		},
		"mutable-key": func(d *requestinput.Declaration) { d.Source.ClientKey = "/var/lib/ardents/roles/endpoint/client.key" },
		"duplicate":   func(d *requestinput.Declaration) { d.Source.ClientKey = d.Source.ClientCertificate },
		"relative":    func(d *requestinput.Declaration) { d.Source.ClientCertificate = "client.crt" },
	} {
		t.Run(name, func(t *testing.T) {
			decl := endpointDeclFixture()
			change(&decl)
			if _, err := requestinput.PlatformSourceCredentials(decl); err == nil {
				t.Fatal("unbound/mutable endpoint credential admitted")
			}
		})
	}
}
