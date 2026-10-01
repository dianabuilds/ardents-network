package runtimeplan

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestReaderDeclarationsProjectPinnedIdentitiesWithoutEffects(t *testing.T) {
	plan := readerDeclarations(t)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHeadless(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.AdministrationPrincipal != [32]byte{} || decoded.PublicationRoot != "" || decoded.ServiceInstanceRoot != "" {
		t.Fatal("Reader acquired Publisher declarations")
	}
	if _, pinned := decoded.NetworkAuthorities[sha256.Sum256(decoded.ClosedProfileAuthority)]; !pinned {
		t.Fatal("profile signer differs from declared State signer")
	}
	for _, path := range []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot, plan.ApplicationSocket} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("decode changed declared path %s: %v", path, err)
		}
	}
}

func TestHeadlessDeclarationsRefuseMixedRolesAndMalformedEnvelope(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Headless)
	}{
		{"Publisher Instance", func(p *Headless) { p.ServiceInstanceRoot = filepath.Join(filepath.Dir(p.NetworkStateRoot), "instance") }},
		{"Publisher Permission", func(p *Headless) { p.PublisherPermission.Maxima = [3]uint32{1} }},
		{"unsupported role", func(p *Headless) { p.Role = "publisher" }},
		{"overlapping roots", func(p *Headless) { p.EntryStateRoot = p.NetworkStateRoot }},
		{"unbounded allocation", func(p *Headless) { p.ReaderPermission.Maxima = [3]uint32{4097} }},
		{"unfixed identity", func(p *Headless) { p.ConnectionPrincipal = "01" }},
		{"unpinned signer", func(p *Headless) { p.ClosedProfileAuthority = strings.Repeat("00", 32) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := readerDeclarations(t)
			test.change(&plan)
			raw, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeHeadless(raw); err == nil {
				t.Fatal("invalid declaration crossed plan boundary")
			}
		})
	}
	plan := readerDeclarations(t)
	plan.Schema = "ardents-headless-runtime-v1"
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeHeadless(raw); !errors.Is(err, ErrHeadlessV1Retired) {
		t.Fatalf("retired plan = %v", err)
	}
	plan.Schema = "ardents-headless-runtime-v2"
	raw, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{append(append([]byte(nil), raw...), []byte(" {}")...), []byte(strings.Replace(string(raw), "{", "{\"unknown\":0,", 1)), []byte(strings.Repeat(" ", 16<<10+1))} {
		if _, err := DecodeHeadless(invalid); err == nil {
			t.Fatal("malformed envelope accepted")
		}
	}
}

func readerDeclarations(t *testing.T) Headless {
	t.Helper()
	root := t.TempDir()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := func(name string) string { return filepath.Join(root, name) }
	return Headless{Schema: "ardents-headless-runtime-v2", Role: "reader", NetworkStateRoot: path("state"), EntryStateRoot: path("entry"), LocalRoleStateRoot: path("roles"), TextTokenRoot: path("tokens"), ApplicationSocket: path("reader.sock"), TimeConfidenceFile: path("time.json"), ReaderPermission: Permission{RequestPath: path("request"), ResponsePath: path("response"), Maxima: [3]uint32{1}}, NetworkID: strings.Repeat("01", 32), BrokerID: strings.Repeat("02", 32), ConnectionPrincipal: strings.Repeat("03", 32), NetworkAuthorities: []string{hex.EncodeToString(public)}, ClosedProfileAuthority: hex.EncodeToString(public), NetworkThreshold: 1, NetworkProfile: carrier.ClosedRouteProfile}
}
