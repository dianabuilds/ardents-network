package config

import (
	"testing"

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
		{"wildcard operator capability", func(d *Document) {
			d.API.Capabilities = []string{"*"}
		}, "explicit action names"},
		{"duplicate operator capability", func(d *Document) {
			d.API.Capabilities = []string{"node.status", "node.status"}
		}, "duplicate"},
		{"invalid credential expiry", func(d *Document) {
			d.API.CredentialExpiresAt = "tomorrow"
		}, "RFC3339"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := Defaults()
			doc.API.TokenFile = "token"
			tc.mutate(&doc)
			require.ErrorContains(t, Validate(doc), tc.want)
		})
	}
}

func TestValidateAcceptsCompleteServiceNode(t *testing.T) {
	doc := Defaults()
	doc.Node.Name = "node-a"
	doc.API.TokenFile = "token"
	doc.Network.BootstrapPeers = []string{"/ip4/10.0.0.2/tcp/60000/p2p/peer"}
	require.NoError(t, Validate(doc))
}

func TestValidateAcceptsCompletePrivateChannelReferences(t *testing.T) {
	doc := Defaults()
	doc.Privacy = PrivacyConfig{
		Required: true, CapabilityStore: "capabilities.db", CapabilityStoreKeyFile: "capabilities.key",
		ReplayKeyFile: "replay.key", Subject: "p_subject", TrustedIssuers: map[string]string{"p_issuer": "public"},
		Discovery: PrivacyChannelConfig{Reference: "discovery-ref", ReplayPath: "discovery-replay.db"},
		Data:      PrivacyChannelConfig{Reference: "data-ref", ReplayPath: "data-replay.db"},
	}
	require.NoError(t, Validate(doc))
}

func TestValidateRejectsDormantPrivacyMaterial(t *testing.T) {
	doc := Defaults()
	doc.Privacy.Subject = "p_subject"
	require.ErrorContains(t, Validate(doc), "privacy.required=true")
}
