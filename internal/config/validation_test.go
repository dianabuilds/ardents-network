package config

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func TestValidateRejectsCrossFieldContradictions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Document)
		want   string
	}{
		{"local development exposure", func(d *Document) {
			d.Node.Profile = "local_development"
			d.Network.BindAddress = "0.0.0.0"
		}, "loopback"},
		{"replica minimum", func(d *Document) {
			d.Data.DesiredReplicas = 2
			d.Data.MinimumReplicas = 3
		}, "minimum_replicas"},
		{"privacy material", func(d *Document) {
			d.Privacy.Required = true
		}, "privacy.capability_store"},
		{"trusted process", func(d *Document) {
			d.Workloads.Executor = "trusted-process"
		}, "local_development"},
		{"unpaired workload probe", func(d *Document) {
			d.Workloads.Initial = []WorkloadSpec{{
				ID: "work.echo", Kind: "service", Owner: "node", Desired: "running",
				Config:   `{"image":"docker.io/library/alpine@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","user":"1000:1000"}`,
				Services: []ServiceConfig{{ID: "svc.echo", Type: "echo", Mode: "NetworkPublished", Endpoints: []string{"tcp://10.0.0.2:19000"}}},
			}}
		}, "paired endpoint"},
		{"unknown desired state", func(d *Document) {
			d.Workloads.Initial = []WorkloadSpec{{ID: "work.echo", Kind: "service", Owner: "node", Desired: "sometimes"}}
		}, "desired is unsupported"},
		{"retention ceiling", func(d *Document) {
			d.Data.DefaultLocalRetention = "2h"
			d.Policy.MaxLocalRetention = "1h"
		}, "exceeds its policy ceiling"},
		{"conflicting policy refs", func(d *Document) {
			d.Workloads.AllowedPolicyRefs = []string{"workload-policy"}
			d.Policy.AllowedPolicyRefs = []string{"operator-policy"}
		}, "conflicts"},
		{"pinning contradiction", func(d *Document) {
			d.Policy.DisableBlobPinning = true
			d.Policy.AllowPinRelayRetainedBlobs = true
		}, "while blob pinning is disabled"},
		{"missing application socket", func(d *Document) {
			d.ApplicationInterface = validApplicationInterface()
			d.ApplicationInterface.SocketPath = ""
		}, "application_interface.socket_path is required"},
		{"shared operator and application socket", func(d *Document) {
			d.ApplicationInterface = validApplicationInterface()
			d.ApplicationInterface.SocketPath = d.API.SocketPath
		}, "must differ"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := Defaults()
			tc.mutate(&doc)
			require.ErrorContains(t, Validate(doc), tc.want)
		})
	}
}

func validApplicationInterface() ApplicationInterfaceConfig {
	return ApplicationInterfaceConfig{Enabled: true, SocketPath: "/run/ardents/application.sock"}
}

func TestValidateAcceptsCompleteServiceNode(t *testing.T) {
	doc := Defaults()
	doc.Node.Name = "node-a"
	doc.Network.BootstrapPeers = []string{"/ip4/10.0.0.2/tcp/60000/p2p/peer"}
	require.NoError(t, Validate(doc))
}

func TestValidateAcceptsCompletePrivateChannelReferences(t *testing.T) {
	doc := Defaults()
	doc.Trust.Principals = []TrustedPrincipalConfig{trustedPrincipalConfig(t, "channel.issue")}
	doc.Privacy = PrivacyConfig{
		Required: true, CapabilityStore: "capabilities.db", CapabilityStoreKeyFile: "capabilities.key",
		ReplayKeyFile: "replay.key", Subject: "p_subject",
		Discovery: PrivacyChannelConfig{Reference: "discovery-ref", ReplayPath: "discovery-replay.db"},
		Data:      PrivacyChannelConfig{Reference: "data-ref", ReplayPath: "data-replay.db"},
	}
	require.NoError(t, Validate(doc))
}

func TestValidatePurposeScopedTrustRejectsUnknownDuplicateAndMismatchedEntries(t *testing.T) {
	valid := trustedPrincipalConfig(t, "channel.issue")
	tests := []struct {
		name   string
		mutate func(*Document)
		want   string
	}{
		{"unknown purpose", func(doc *Document) {
			entry := valid
			entry.Purposes = []identitytrust.Purpose{"channel.admin"}
			doc.Trust.Principals = []TrustedPrincipalConfig{entry}
		}, "trust.principals[0]"},
		{"duplicate purpose", func(doc *Document) {
			entry := valid
			entry.Purposes = []identitytrust.Purpose{identitytrust.PurposeChannelIssue, identitytrust.PurposeChannelIssue}
			doc.Trust.Principals = []TrustedPrincipalConfig{entry}
		}, "duplicated"},
		{"mismatched public key", func(doc *Document) {
			entry := valid
			entry.PublicKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x55}, ed25519.PublicKeySize))
			doc.Trust.Principals = []TrustedPrincipalConfig{entry}
		}, "does not match"},
		{"duplicate principal", func(doc *Document) {
			doc.Trust.Principals = []TrustedPrincipalConfig{valid, valid}
		}, "duplicated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := Defaults()
			tc.mutate(&doc)
			require.ErrorContains(t, Validate(doc), tc.want)
		})
	}
}

func TestValidatePrivateChannelsRequireChannelIssuePurpose(t *testing.T) {
	doc := Defaults()
	doc.Trust.Principals = []TrustedPrincipalConfig{trustedPrincipalConfig(t, "discovery.publish")}
	doc.Privacy = PrivacyConfig{
		Required: true, CapabilityStore: "capabilities.db", CapabilityStoreKeyFile: "capabilities.key",
		ReplayKeyFile: "replay.key", Subject: "p_subject",
		Discovery: PrivacyChannelConfig{Reference: "discovery-ref", ReplayPath: "discovery-replay.db"},
		Data:      PrivacyChannelConfig{Reference: "data-ref", ReplayPath: "data-replay.db"},
	}
	require.ErrorContains(t, Validate(doc), "channel.issue")
}

func trustedPrincipalConfig(t *testing.T, purposes ...identitytrust.Purpose) TrustedPrincipalConfig {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	return TrustedPrincipalConfig{Principal: principal.String(), PublicKey: base64.StdEncoding.EncodeToString(public), Purposes: purposes}
}

func TestValidateRejectsDormantPrivacyMaterial(t *testing.T) {
	doc := Defaults()
	doc.Privacy.Subject = "p_subject"
	require.ErrorContains(t, Validate(doc), "privacy.required=true")
}

func TestValidateRejectsRelativeAPISocketPath(t *testing.T) {
	doc := Defaults()
	doc.API.SocketPath = "run/ardents/control.sock"
	require.ErrorContains(t, Validate(doc), "api.socket_path must be absolute")
}
