package config

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
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
		}, "privacy.channel_grant_store"},
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

func TestValidateAuthorityRequiresSeparateExplicitProductionInputs(t *testing.T) {
	dir := t.TempDir()
	valid := func() Document {
		doc := Defaults()
		doc.Node.DataDir = filepath.Join(dir, "node")
		doc.Authority = AuthorityConfig{
			Enabled:                  true,
			StorePath:                filepath.Join(dir, "authority", "realm-authority.db"),
			StoreKeyFile:             filepath.Join(dir, "authority-secrets", "store.key"),
			SignerFile:               filepath.Join(dir, "authority-secrets", "signer.json"),
			CheckpointRepositoryPath: filepath.Join(dir, "independent-checkpoints"),
		}
		return doc
	}
	require.NoError(t, Validate(valid()))
	withSuccessor := valid()
	withSuccessor.Authority.SuccessorSignerFile =
		filepath.Join(dir, "authority-secrets", "successor-signer.json")
	require.NoError(t, Validate(withSuccessor))

	for name, mutate := range map[string]func(*Document){
		"disabled material": func(doc *Document) {
			doc.Authority.Enabled = false
		},
		"disabled recovery mode": func(doc *Document) {
			doc.Authority.Enabled = false
			doc.Authority.StorePath = ""
			doc.Authority.StoreKeyFile = ""
			doc.Authority.SignerFile = ""
			doc.Authority.CheckpointRepositoryPath = ""
			doc.Authority.RecoveryOnly = true
		},
		"missing signer": func(doc *Document) {
			doc.Authority.SignerFile = ""
		},
		"successor equals current signer": func(doc *Document) {
			doc.Authority.SuccessorSignerFile = doc.Authority.SignerFile
		},
		"checkpoint under node state": func(doc *Document) {
			doc.Authority.CheckpointRepositoryPath = filepath.Join(doc.Node.DataDir, "checkpoints")
		},
		"checkpoint contains store": func(doc *Document) {
			doc.Authority.StorePath = filepath.Join(doc.Authority.CheckpointRepositoryPath, "authority.db")
		},
	} {
		t.Run(name, func(t *testing.T) {
			doc := valid()
			mutate(&doc)
			require.Error(t, Validate(doc))
		})
	}
}

func TestValidateAcceptsCompleteServiceNode(t *testing.T) {
	doc := Defaults()
	doc.Node.Name = "node-a"
	doc.Node.ImageReference = "registry.example/ardents/node@sha256:" + strings.Repeat("a", 64)
	doc.Network.BootstrapPeers = []string{"/ip4/10.0.0.2/tcp/60000/p2p/peer"}
	require.NoError(t, Validate(doc))
}

func TestValidateRejectsMutableNodeImageReference(t *testing.T) {
	doc := Defaults()
	doc.Node.ImageReference = "registry.example/ardents/node:latest"
	require.ErrorContains(t, Validate(doc), "node.image_reference")
}

func TestValidateRequiresFiniteWakuStoreRetentionForPersistentProfiles(t *testing.T) {
	for _, profile := range []string{"service_node", "local_development"} {
		t.Run(profile, func(t *testing.T) {
			doc := Defaults()
			doc.Node.Profile = profile
			if profile == "local_development" {
				doc.Network.BindAddress = "127.0.0.1"
			}
			doc.Network.Limits.StoreMaxMessages = 0
			require.ErrorContains(t, Validate(doc), "network.limits.store_max_messages")

			doc = Defaults()
			doc.Node.Profile = profile
			if profile == "local_development" {
				doc.Network.BindAddress = "127.0.0.1"
			}
			doc.Network.Limits.StoreMaxAgeSeconds = 0
			require.ErrorContains(t, Validate(doc), "network.limits.store_max_age_seconds")

			doc = Defaults()
			doc.Node.Profile = profile
			if profile == "local_development" {
				doc.Network.BindAddress = "127.0.0.1"
			}
			doc.Network.Limits.StoreMaxBytes = 0
			require.ErrorContains(t, Validate(doc), "network.limits.store_max_bytes")
		})
	}
}

func TestValidateAllowsDisabledWakuStoreRetentionForConstrainedClient(t *testing.T) {
	doc := Defaults()
	doc.Node.Profile = "constrained_light_client"
	doc.Network.Limits.StoreMaxMessages = 0
	doc.Network.Limits.StoreMaxAgeSeconds = 0
	doc.Network.Limits.StoreMaxBytes = 0
	require.NoError(t, Validate(doc))
}

func TestValidateAcceptsCompletePrivateChannelReferences(t *testing.T) {
	doc := Defaults()
	doc.Trust.Principals = []TrustedPrincipalConfig{trustedPrincipalConfig(t, "channel.issue")}
	doc.Privacy = PrivacyConfig{
		Required: true, ChannelGrantStore: "channel-grants.db", ChannelGrantStoreKeyFile: "channel-grants.key",
		ReplayKeyFile: "replay.key", Subject: "p_subject",
		Discovery: PrivacyChannelConfig{Reference: "discovery-ref", ReplayPath: "discovery-replay.db"},
		Data:      PrivacyChannelConfig{Reference: "data-ref", ReplayPath: "data-replay.db"},
	}
	require.NoError(t, Validate(doc))
}

func TestValidatePrivateReplayStoresByPhysicalIdentity(t *testing.T) {
	t.Run("dot alias", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "replay.db")
		alias := dir + string(os.PathSeparator) + "." + string(os.PathSeparator) + "replay.db"
		require.ErrorContains(t, Validate(privateReplayDocument(t, path, alias)), "same physical store")
	})

	t.Run("symlink directory alias", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		require.NoError(t, os.Mkdir(target, 0o700))
		alias := filepath.Join(dir, "alias")
		if err := os.Symlink(target, alias); err != nil {
			t.Skipf("directory symlinks are unavailable: %v", err)
		}
		require.ErrorContains(t, Validate(privateReplayDocument(t,
			filepath.Join(target, "replay.db"), filepath.Join(alias, "replay.db"))), "same physical store")
	})

	t.Run("dangling file symlink alias", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "replay.db")
		alias := filepath.Join(dir, "replay-alias.db")
		if err := os.Symlink(target, alias); err != nil {
			t.Skipf("file symlinks are unavailable: %v", err)
		}
		require.ErrorContains(t, Validate(privateReplayDocument(t, target, alias)), "same physical store")
	})

	t.Run("hard link alias", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "replay.db")
		alias := filepath.Join(dir, "replay-link.db")
		require.NoError(t, os.WriteFile(path, []byte("existing"), 0o600))
		if err := os.Link(path, alias); err != nil {
			t.Skipf("hard links are unavailable: %v", err)
		}
		require.ErrorContains(t, Validate(privateReplayDocument(t, path, alias)), "same physical store")
	})

	t.Run("case-only alias is rejected portably", func(t *testing.T) {
		dir := t.TempDir()
		lower := filepath.Join(dir, "replay.db")
		upper := filepath.Join(dir, "REPLAY.DB")
		require.ErrorContains(t, Validate(privateReplayDocument(t, lower, upper)), "same physical store")
	})

	t.Run("different files", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, Validate(privateReplayDocument(t,
			filepath.Join(dir, "discovery.db"), filepath.Join(dir, "data.db"))))
	})
}

func privateReplayDocument(t *testing.T, discoveryPath, dataPath string) Document {
	t.Helper()
	doc := Defaults()
	doc.Trust.Principals = []TrustedPrincipalConfig{trustedPrincipalConfig(t, "channel.issue")}
	doc.Privacy = PrivacyConfig{
		Required: true, ChannelGrantStore: "channel-grants.db", ChannelGrantStoreKeyFile: "channel-grants.key",
		ReplayKeyFile: "replay.key", Subject: "p_subject",
		Discovery: PrivacyChannelConfig{Reference: "discovery-ref", ReplayPath: discoveryPath},
		Data:      PrivacyChannelConfig{Reference: "data-ref", ReplayPath: dataPath},
	}
	return doc
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
		Required: true, ChannelGrantStore: "channel-grants.db", ChannelGrantStoreKeyFile: "channel-grants.key",
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
	require.ErrorContains(t, Validate(doc), "privacy.delivery_enabled=true")
}

func TestValidateAcceptsDeliveryOnlyPrivacyBootstrap(t *testing.T) {
	doc := Defaults()
	doc.Trust.Principals = []TrustedPrincipalConfig{trustedPrincipalConfig(t, "channel.issue")}
	doc.Privacy = PrivacyConfig{
		DeliveryEnabled: true, ChannelGrantStore: "channel-grants.db",
		ChannelGrantStoreKeyFile: "channel-grants.key", Subject: "p_subject",
	}
	require.NoError(t, Validate(doc))
}

func TestValidateRejectsRelativeAPISocketPath(t *testing.T) {
	doc := Defaults()
	doc.API.SocketPath = "run/ardents/control.sock"
	require.ErrorContains(t, Validate(doc), "api.socket_path must be absolute")
}
