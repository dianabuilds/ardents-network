package topology

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ardents/internal/cli/client"
	configurationcmd "ardents/internal/cli/configuration"
	"ardents/internal/deployment"
	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeFactory struct {
	opened []string
	closed []string
}

func (factory *fakeFactory) Open(cfg configurationcmd.Config) (openedClient, error) {
	factory.opened = append(factory.opened, cfg.ExpectedNode)
	calls := fakeProtectedCalls{
		node: cfg.ExpectedNode, principal: cfg.ExpectedPrincipal,
		image: imageForNode(cfg.ExpectedNode), observedAt: time.Now().UTC(),
		authority: &protocol.AuthorityStatusSnapshot{
			Version: 1, RealmId: cfg.ExpectedRealm, AuthoritySequence: 42,
			CheckpointDigest: "ac1_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Phase:            "recovery_only", Readiness: "degraded",
			Reason: "authority_restore_verification_required",
		},
		verified: &protocol.AuthorityStatusSnapshot{
			Version: 1, RealmId: cfg.ExpectedRealm, AuthoritySequence: 42,
			CheckpointDigest: "ac1_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Phase:            "ready", Readiness: "ready",
		},
	}
	return openedClient{calls: calls, close: func(context.Context) error {
		factory.closed = append(factory.closed, cfg.ExpectedNode)
		return nil
	}}, nil
}

type fakeProtectedCalls struct {
	node       string
	principal  string
	image      string
	runtimeErr error
	networkErr error
	featureErr error
	malformed  bool
	observedAt time.Time
	authority  *protocol.AuthorityStatusSnapshot
	verified   *protocol.AuthorityStatusSnapshot
}

func (calls fakeProtectedCalls) GetNodeRuntime(context.Context, *connect.Request[protocol.GetNodeRuntimeRequest]) (*connect.Response[protocol.NodeRuntimeResponse], error) {
	if calls.runtimeErr != nil {
		return nil, calls.runtimeErr
	}
	if calls.malformed {
		return nil, nil
	}
	return connect.NewResponse(&protocol.NodeRuntimeResponse{
		Runtime: &protocol.NodeRuntimeSnapshot{
			Node: &protocol.NodeSnapshot{Name: calls.node}, Identity: &protocol.IdentitySnapshot{Principal: calls.principal},
			Readiness: &protocol.ReadinessSnapshot{Ready: true},
		},
		ObservedAt: timestamppb.New(calls.observedAt),
	}), nil
}

func (calls fakeProtectedCalls) GetNetworkStatus(context.Context, *connect.Request[protocol.GetNetworkStatusRequest]) (*connect.Response[protocol.NetworkStatusResponse], error) {
	if calls.networkErr != nil {
		return nil, calls.networkErr
	}
	network := &protocol.NetworkStatusSnapshot{
		Joined: true, StoreEnabled: true, StoreState: "ready",
		ReachabilityMode: "public_direct", ReachabilityState: "public", Reachable: true,
	}
	if calls.node == "node-c" {
		network.ReachabilityMode = "outbound_only"
		network.ReachabilityState = "outbound_only"
		network.Reachable = false
		network.StoreEnabled = false
		network.StoreState = "disabled"
	}
	return connect.NewResponse(&protocol.NetworkStatusResponse{Network: network}), nil
}

func (calls fakeProtectedCalls) GetNodeFeatures(context.Context, *connect.Request[protocol.GetNodeFeaturesRequest]) (*connect.Response[protocol.NodeFeaturesResponse], error) {
	if calls.featureErr != nil {
		return nil, calls.featureErr
	}
	return connect.NewResponse(&protocol.NodeFeaturesResponse{Features: &protocol.NodeFeaturesSnapshot{ImageReference: calls.image}}), nil
}

func (calls fakeProtectedCalls) InspectRealmAuthority(
	context.Context,
	*connect.Request[protocol.InspectRealmAuthorityRequest],
) (*connect.Response[protocol.InspectRealmAuthorityResponse], error) {
	return connect.NewResponse(&protocol.InspectRealmAuthorityResponse{Authority: calls.authority}), nil
}

func (calls fakeProtectedCalls) VerifyRestoredAuthority(
	context.Context,
	*connect.Request[protocol.VerifyRestoredAuthorityRequest],
) (*connect.Response[protocol.VerifyRestoredAuthorityResponse], error) {
	return connect.NewResponse(&protocol.VerifyRestoredAuthorityResponse{Authority: calls.verified}), nil
}

type fixedFactory struct {
	calls   protectedCalls
	openErr error
	close   func(context.Context) error
}

func (factory fixedFactory) Open(configurationcmd.Config) (openedClient, error) {
	if factory.openErr != nil {
		return openedClient{}, factory.openErr
	}
	closeClient := factory.close
	if closeClient == nil {
		closeClient = func(context.Context) error { return nil }
	}
	return openedClient{calls: factory.calls, close: closeClient}, nil
}

func TestCommandUsesThreeSeparateManifestBoundContextsAndRedactsJSON(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "deployment", "testdata", "public-direct.json")
	contextFile := writeTopologyContexts(t)
	factory := &fakeFactory{}
	var stdout, stderr bytes.Buffer
	command := Command{
		Base: configurationcmd.Config{ContextFile: contextFile, Output: "json", Timeout: time.Second},
		Out:  &stdout, Err: &stderr, Factory: factory,
	}
	code := command.Run(context.Background(), []string{"status", "--manifest", manifestPath})
	require.Zero(t, code, stderr.String())
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, factory.opened)
	require.ElementsMatch(t, factory.opened, factory.closed)
	var result map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	require.Equal(t, "ready", result["outcome"])
	for _, secret := range []string{"operator@", "host-pin-", "operator-primary", "p1_", "registry.example", "sha256:"} {
		require.NotContains(t, stdout.String()+stderr.String(), secret)
	}
}

func TestRecoverCommandVerifiesManifestBoundAuthorityWithoutIdentifierLeaks(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "deployment", "testdata", "public-direct.json")
	contextFile := writeTopologyContexts(t)
	factory := &fakeFactory{}
	var stdout, stderr bytes.Buffer

	code := (Command{
		Base: configurationcmd.Config{ContextFile: contextFile, Output: "json", Timeout: time.Second},
		Out:  &stdout, Err: &stderr, Factory: factory,
	}).Run(context.Background(), []string{"recover", "--manifest", manifestPath})

	require.Zero(t, code, stderr.String())
	require.Equal(t, []string{"node-b", "node-c", "node-a"}, factory.opened)
	require.ElementsMatch(t, factory.opened, factory.closed)
	var status deployment.AuthorityRecoveryStatus
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &status))
	require.Equal(t, deployment.AuthorityRecoveryOutcomeVerified, status.Outcome)
	for _, protected := range []string{
		recoveryRealmIDForCLI, "ac1_", "authority-state-primary",
		"authority-backup-primary", "authority-checkpoints-primary", "operator@",
	} {
		require.NotContains(t, stdout.String()+stderr.String(), protected)
	}
}

func TestRecoveryProbeRejectsAuthorityContextMismatchBeforeOpeningClient(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*configurationcmd.StoredContext)
	}{
		{
			name: "realm",
			mutate: func(stored *configurationcmd.StoredContext) {
				stored.ExpectedRealm = "r1_11112233445566778899aabbccddeeff"
			},
		},
		{
			name: "authority state reference",
			mutate: func(stored *configurationcmd.StoredContext) {
				stored.AuthorityStateRef = "other-state"
			},
		},
		{
			name: "authority backup reference",
			mutate: func(stored *configurationcmd.StoredContext) {
				stored.AuthorityBackupRef = "other-backup"
			},
		},
		{
			name: "checkpoint repository reference",
			mutate: func(stored *configurationcmd.StoredContext) {
				stored.CheckpointRepositoryRef = "other-repository"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contextFile := writeTopologyContexts(t)
			raw, err := os.ReadFile(contextFile)
			require.NoError(t, err)
			var contexts configurationcmd.ContextFile
			require.NoError(t, json.Unmarshal(raw, &contexts))
			stored := contexts.Contexts["ssh-node-a"]
			test.mutate(&stored)
			contexts.Contexts["ssh-node-a"] = stored
			raw, err = json.Marshal(contexts)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(contextFile, raw, 0o600))
			factory := &fakeFactory{}

			_, err = (RecoveryProbe{
				Base:    configurationcmd.Config{ContextFile: contextFile, Timeout: time.Second},
				Factory: factory,
			}).Open(context.Background(), recoveryTarget())

			require.EqualError(t, err, string(deployment.AuthorityRecoveryReasonContextMismatch))
			require.Empty(t, factory.opened)
		})
	}
}

func TestProbeRejectsManifestToContextBindingMismatchBeforeOpeningClient(t *testing.T) {
	contextFile := writeTopologyContexts(t)
	tests := []struct {
		name   string
		mutate func(*deployment.NodeStatusTarget)
		reason deployment.ProbeErrorCode
	}{
		{
			name: "pin",
			mutate: func(target *deployment.NodeStatusTarget) {
				target.HostKeyPinRef = "different-pin"
			},
			reason: deployment.ProbeHostKeyMismatch,
		},
		{
			name: "signer alias",
			mutate: func(target *deployment.NodeStatusTarget) {
				target.OperatorSignerAlias = "different-signer"
			},
			reason: deployment.ProbeLocalSignerUnavailable,
		},
		{
			name: "node slot",
			mutate: func(target *deployment.NodeStatusTarget) {
				target.Slot = "node-z"
			},
			reason: deployment.ProbeRemoteInvalidResponse,
		},
		{
			name: "principal",
			mutate: func(target *deployment.NodeStatusTarget) {
				target.ExpectedNodePrincipal = "p1_jjkwa23wqggjpivnxdb45wpe575akea3eyytyr2slvuhg7ujsspq"
			},
			reason: deployment.ProbeRemoteInvalidResponse,
		},
		{
			name: "missing SSH alias",
			mutate: func(target *deployment.NodeStatusTarget) {
				target.SSHAlias = "missing-context"
			},
			reason: deployment.ProbeTunnelFailure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factory := &fakeFactory{}
			probe := Probe{
				Base:    configurationcmd.Config{ContextFile: contextFile, Timeout: time.Second},
				Factory: factory,
			}
			target := topologyTarget("host-pin-a")
			test.mutate(&target)
			_, err := probe.Observe(context.Background(), target)
			require.EqualError(t, err, string(test.reason))
			require.Empty(t, factory.opened)
		})
	}
}

func TestCommandHumanOutputIncludesReadinessAndJoinedTruth(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "deployment", "testdata", "public-direct.json")
	contextFile := writeTopologyContexts(t)
	var stdout, stderr bytes.Buffer
	code := (Command{
		Base: configurationcmd.Config{ContextFile: contextFile, Timeout: time.Second},
		Out:  &stdout, Err: &stderr, Factory: &fakeFactory{},
	}).Run(context.Background(), []string{"status", "--manifest", manifestPath})
	require.Zero(t, code, stderr.String())
	require.Contains(t, stdout.String(), "READINESS")
	require.Contains(t, stdout.String(), "JOINED")
	require.Contains(t, stdout.String(), "node-a")
	require.Contains(t, stdout.String(), "true")
}

func TestCommandRejectsNonRegularManifestWithoutPathLeak(t *testing.T) {
	path := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := (Command{
		Base: configurationcmd.Config{Output: "json"},
		Out:  &stdout, Err: &stderr,
	}).Run(context.Background(), []string{"status", "--manifest", path})
	require.Equal(t, 2, code)
	require.Empty(t, stdout.String())
	require.NotContains(t, stderr.String(), path)
}

func TestClassifyProbeErrorPreservesTunnelAndRemoteFailureClasses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want deployment.ProbeError
	}{
		{
			name: "host key mismatch",
			err:  client.ErrSSHHostKeyMismatch,
			want: deployment.ProbeError(deployment.ProbeHostKeyMismatch),
		},
		{
			name: "tunnel failure through connect",
			err:  connect.NewError(connect.CodeUnavailable, client.ErrSSHTunnelFailure),
			want: deployment.ProbeError(deployment.ProbeTunnelFailure),
		},
		{
			name: "deadline through connect",
			err:  connect.NewError(connect.CodeDeadlineExceeded, errors.New("deadline")),
			want: deployment.ProbeError(deployment.ProbeTunnelTimeout),
		},
		{
			name: "local signer",
			err:  deployment.ProbeError(deployment.ProbeLocalSignerUnavailable),
			want: deployment.ProbeError(deployment.ProbeLocalSignerUnavailable),
		},
		{
			name: "remote unauthenticated",
			err:  connect.NewError(connect.CodeUnauthenticated, errors.New("session")),
			want: deployment.ProbeError(deployment.ProbeRemoteUnauthenticated),
		},
		{
			name: "remote denied",
			err:  connect.NewError(connect.CodePermissionDenied, errors.New("denied")),
			want: deployment.ProbeError(deployment.ProbeRemoteDenied),
		},
		{
			name: "remote unavailable",
			err:  connect.NewError(connect.CodeUnavailable, errors.New("offline")),
			want: deployment.ProbeError(deployment.ProbeNodeUnavailable),
		},
		{
			name: "invalid response",
			err:  errors.New("malformed response"),
			want: deployment.ProbeError(deployment.ProbeRemoteInvalidResponse),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, classifyProbeError(test.err), test.want.Error())
		})
	}
}

func TestClassifyAuthorityRecoveryErrorUsesStableStructuredReason(t *testing.T) {
	connectErr := connect.NewError(
		connect.CodeFailedPrecondition,
		errors.New("protected checkpoint path and operator detail"),
	)
	detail, err := connect.NewErrorDetail(&protocol.Error{
		Code: "authority_not_ready", Category: "authority",
		Message: "protected checkpoint path and operator detail",
		Domain:  "authority", Operation: "verify_restored_authority",
		Reason: "authority_recovery_required",
	})
	require.NoError(t, err)
	connectErr.AddDetail(detail)

	got := classifyAuthorityRecoveryError(connectErr)

	require.EqualError(
		t,
		got,
		string(deployment.AuthorityRecoveryReasonCheckpointMismatch),
	)
	require.NotContains(t, got.Error(), "protected checkpoint")

	forkErr := connect.NewError(
		connect.CodeFailedPrecondition,
		errors.New("different protected detail"),
	)
	forkDetail, err := connect.NewErrorDetail(&protocol.Error{
		Code: "authority_not_ready", Category: "authority",
		Message: "different protected detail",
		Domain:  "authority", Operation: "verify_restored_authority",
		Reason: "checkpoint_history_fork",
	})
	require.NoError(t, err)
	forkErr.AddDetail(forkDetail)
	require.EqualError(
		t,
		classifyAuthorityRecoveryError(forkErr),
		string(deployment.AuthorityRecoveryReasonFork),
	)
}

func TestProbeMapsEveryStableFailureClass(t *testing.T) {
	contextFile := writeTopologyContexts(t)
	validCalls := func() fakeProtectedCalls {
		return fakeProtectedCalls{
			node:      "node-a",
			principal: "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
			image:     imageForNode("node-a"),
		}
	}
	tests := []struct {
		name    string
		factory fixedFactory
		mutate  func(*deployment.NodeStatusTarget)
		want    deployment.ProbeErrorCode
	}{
		{
			name:    "host key mismatch",
			factory: fixedFactory{calls: validCalls()},
			mutate: func(target *deployment.NodeStatusTarget) {
				target.HostKeyPinRef = "different-pin"
			},
			want: deployment.ProbeHostKeyMismatch,
		},
		{
			name:    "tunnel timeout",
			factory: fixedFactory{openErr: client.ErrSSHTunnelTimeout},
			want:    deployment.ProbeTunnelTimeout,
		},
		{
			name:    "tunnel failure",
			factory: fixedFactory{openErr: client.ErrSSHTunnelFailure},
			want:    deployment.ProbeTunnelFailure,
		},
		{
			name:    "local signer unavailable",
			factory: fixedFactory{openErr: deployment.ProbeError(deployment.ProbeLocalSignerUnavailable)},
			want:    deployment.ProbeLocalSignerUnavailable,
		},
		{
			name: "remote unauthenticated",
			factory: fixedFactory{calls: func() fakeProtectedCalls {
				calls := validCalls()
				calls.runtimeErr = connect.NewError(connect.CodeUnauthenticated, errors.New("session"))
				return calls
			}()},
			want: deployment.ProbeRemoteUnauthenticated,
		},
		{
			name: "remote denied",
			factory: fixedFactory{calls: func() fakeProtectedCalls {
				calls := validCalls()
				calls.networkErr = connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
				return calls
			}()},
			want: deployment.ProbeRemoteDenied,
		},
		{
			name: "node unavailable",
			factory: fixedFactory{calls: func() fakeProtectedCalls {
				calls := validCalls()
				calls.featureErr = connect.NewError(connect.CodeUnavailable, errors.New("offline"))
				return calls
			}()},
			want: deployment.ProbeNodeUnavailable,
		},
		{
			name: "remote invalid response",
			factory: fixedFactory{calls: func() fakeProtectedCalls {
				calls := validCalls()
				calls.malformed = true
				return calls
			}()},
			want: deployment.ProbeRemoteInvalidResponse,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := topologyTarget("host-pin-a")
			if test.mutate != nil {
				test.mutate(&target)
			}
			probe := Probe{
				Base:    configurationcmd.Config{ContextFile: contextFile, Timeout: time.Second},
				Factory: test.factory,
			}
			_, err := probe.Observe(context.Background(), target)
			require.EqualError(t, err, string(test.want))
		})
	}
}

func TestProbeCloseCannotOutliveNodeDeadline(t *testing.T) {
	contextFile := writeTopologyContexts(t)
	factory := fixedFactory{
		calls: fakeProtectedCalls{
			node:      "node-a",
			principal: "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
			image:     imageForNode("node-a"),
		},
		close: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := (Probe{
		Base:    configurationcmd.Config{ContextFile: contextFile, Timeout: time.Second},
		Factory: factory,
	}).Observe(ctx, topologyTarget("host-pin-a"))
	require.EqualError(t, err, string(deployment.ProbeTunnelTimeout))
	require.Less(t, time.Since(started), 250*time.Millisecond)
}

func writeTopologyContexts(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "deployment", "testdata", "public-direct.json"))
	require.NoError(t, err)
	var manifest struct {
		OperatorSignerAlias string `json:"operator_signer_alias"`
		Authority           struct {
			Slot      string `json:"slot"`
			StateRef  string `json:"state_ref"`
			BackupRef string `json:"backup_ref"`
		} `json:"authority"`
		CheckpointRepository struct {
			Reference string `json:"reference"`
		} `json:"checkpoint_repository"`
		Nodes []struct {
			Slot string `json:"slot"`
			Host struct {
				SSHAlias      string `json:"ssh_alias"`
				HostKeyPinRef string `json:"host_key_pin_ref"`
			} `json:"host"`
			Principal string `json:"expected_node_principal"`
		} `json:"nodes"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))
	contexts := configurationcmd.ContextFile{Contexts: map[string]configurationcmd.StoredContext{}}
	for _, node := range manifest.Nodes {
		stored := configurationcmd.StoredContext{
			Addr: "unix:///run/ardents/operator.sock", SSH: "operator@" + node.Slot,
			SSHKnownHosts: "pins/" + node.Slot, SSHOperatorSocket: "/run/ardents/operator.sock",
			SignerFile: "signers/operator.json", SignerAlias: manifest.OperatorSignerAlias,
			HostKeyPinRef: node.Host.HostKeyPinRef, ExpectedNode: node.Slot, ExpectedPrincipal: node.Principal,
		}
		if node.Slot == manifest.Authority.Slot {
			stored.ExpectedRealm = recoveryRealmIDForCLI
			stored.AuthorityStateRef = manifest.Authority.StateRef
			stored.AuthorityBackupRef = manifest.Authority.BackupRef
			stored.CheckpointRepositoryRef = manifest.CheckpointRepository.Reference
		}
		contexts.Contexts[node.Host.SSHAlias] = stored
	}
	encoded, err := json.Marshal(contexts)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "contexts.json")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	return path
}

const recoveryRealmIDForCLI = "r1_00112233445566778899aabbccddeeff"

func topologyTarget(pin string) deployment.NodeStatusTarget {
	return deployment.NodeStatusTarget{
		Slot: "node-a", SSHAlias: "ssh-node-a", HostKeyPinRef: pin,
		OperatorSignerAlias:   "operator-primary",
		ExpectedNodePrincipal: "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
	}
}

func recoveryTarget() deployment.AuthorityRecoveryTarget {
	return deployment.AuthorityRecoveryTarget{
		Slot: "node-a", Role: "authority", SSHAlias: "ssh-node-a",
		HostKeyPinRef: "host-pin-a", OperatorSignerAlias: "operator-primary",
		ExpectedNodePrincipal:   "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		ExpectedRealmID:         recoveryRealmIDForCLI,
		AuthorityStateRef:       "authority-state-primary",
		AuthorityBackupRef:      "authority-backup-primary",
		CheckpointRepositoryRef: "authority-checkpoints-primary",
	}
}

func imageForNode(node string) string {
	digest := map[string]string{"node-a": "a", "node-b": "b", "node-c": "c"}[node]
	return "registry.example/ardents/node@sha256:" + strings.Repeat(digest, 64)
}
