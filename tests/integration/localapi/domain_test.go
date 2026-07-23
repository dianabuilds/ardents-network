//go:build integration

package localapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cliclient "ardents/internal/cli/client"
	runtimeinfra "ardents/internal/daemon"
	rpcadapter "ardents/internal/localapi"
	localauth "ardents/internal/localapi/auth"
	ardentsv1 "ardents/internal/localapi/protocol"
	transport "ardents/internal/network"
	"ardents/tests/testkit"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConnectRPCExposesNodeAndDiagnostics(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "LCI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		NodeProfile: transport.NodeProfileServiceNode,
		Name:        "connect",
		Boot:        runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:        runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)

	status, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
	require.NoError(t, err)
	require.Equal(t, "ready", status.Msg.GetSnapshot().GetNode().GetState())

	diag, err := client.GetDiagnostics(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDiagnosticsRequest{}))
	require.NoError(t, err)
	require.Equal(t, "ready", diag.Msg.GetDiagnostics().GetHealth().GetState())
	require.NotEmpty(t, diag.Msg.GetDiagnostics().GetRecentEvents())
}

func TestConnectRPCProjectsDegradedHostedServiceAndDiagnostics(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "LCI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "connect-degraded",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.invalid",
			Kind:    "unsupported",
			Owner:   "node",
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.invalid",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://127.0.0.1:9000"},
			}},
		}},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)

	status, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
	require.NoError(t, err)
	require.Equal(t, "degraded", status.Msg.GetSnapshot().GetNode().GetState())

	diag, err := client.GetDiagnostics(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDiagnosticsRequest{}))
	require.NoError(t, err)
	require.Equal(t, "degraded", diag.Msg.GetDiagnostics().GetHealth().GetState())
	require.NotNil(t, diag.Msg.GetDiagnostics().GetHealth().GetPrimaryReason())
	require.Equal(t, "workload.hosted_service.degraded", diag.Msg.GetDiagnostics().GetHealth().GetPrimaryReason().GetCode())
	require.Equal(t, "workload", diag.Msg.GetDiagnostics().GetHealth().GetPrimaryReason().GetDomain())

	workload, err := client.GetWorkloadStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetWorkloadStatusRequest{Id: "work.invalid"}))
	require.NoError(t, err)
	require.Equal(t, "failed", workload.Msg.GetObserved())
	require.Contains(t, workload.Msg.GetReason(), "unsupported workload kind")
	require.Len(t, workload.Msg.GetPublishedServices(), 1)
	require.False(t, workload.Msg.GetPublishedServices()[0].GetPublished())
	require.NotEmpty(t, workload.Msg.GetPublishedServices()[0].GetReason())
}

func TestConnectRPCProjectsTransportModeAndPeerLossTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "LCI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "transport-variant"},
		Speed:       "default",
		Environment: "local",
	})

	remote := testkit.StartTransport(t)
	bootSources := append([]string(nil), remote.Endpoints()...)

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "connect-transport-truth",
		Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), bootSources...)},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)

	var readyStatus *connect.Response[ardentsv1.NodeStatusResponse]
	testkit.WaitForCondition(t, 5*time.Second, "connect status shows ready transport truth", func() (bool, string) {
		resp, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
		if err != nil {
			return false, err.Error()
		}
		snap := resp.Msg.GetSnapshot()
		if !snap.GetBoot().GetJoined() || snap.GetBoot().GetState() != "ready" || snap.GetTrans().GetState() != "ready" {
			return false, snap.GetBoot().GetState() + "/" + snap.GetTrans().GetState()
		}
		readyStatus = resp
		return true, ""
	})

	require.NotNil(t, readyStatus)
	require.ElementsMatch(t, bootSources, readyStatus.Msg.GetSnapshot().GetBoot().GetSource())
	require.Empty(t, readyStatus.Msg.GetSnapshot().GetTrans().GetReason())
	require.Equal(t, "tcp_only", readyStatus.Msg.GetSnapshot().GetTransport().GetProfile())
	require.Equal(t, "ready", readyStatus.Msg.GetSnapshot().GetTransport().GetHealth())
	require.ElementsMatch(t, []string{"tcp"}, readyStatus.Msg.GetSnapshot().GetTransport().GetActiveFamilies())
	require.Contains(t, readyStatus.Msg.GetSnapshot().GetTransport().GetSuppressedFamilies(), "quic")
	require.Equal(t, "startup_default", readyStatus.Msg.GetSnapshot().GetTransport().GetSwitchReason())

	require.NoError(t, remote.Stop(t.Context()))

	testkit.WaitForCondition(t, 5*time.Second, "connect status shows degraded transport truth after peer loss", func() (bool, string) {
		status, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
		if err != nil {
			return false, err.Error()
		}
		snap := status.Msg.GetSnapshot()
		if snap.GetBoot().GetState() != "degraded" || snap.GetTrans().GetState() != "degraded" {
			return false, snap.GetBoot().GetState() + "/" + snap.GetTrans().GetState()
		}
		if len(snap.GetBoot().GetSource()) == 0 {
			return false, "missing boot source"
		}
		if snap.GetTransport().GetProfile() != "tcp_only" || snap.GetTransport().GetMode() != "restricted_defense" || snap.GetTransport().GetHealth() != "degraded" {
			return false, snap.GetTransport().GetProfile() + "/" + snap.GetTransport().GetMode() + "/" + snap.GetTransport().GetHealth()
		}
		if snap.GetTransport().GetSwitchReason() != "health_degraded" || !snap.GetTransport().GetSwitchAutomatic() {
			return false, snap.GetTransport().GetSwitchReason()
		}
		if snap.GetTransport().GetRecoveryState() == "" {
			return false, "missing recovery state"
		}
		return true, ""
	})

	testkit.WaitForCondition(t, 5*time.Second, "connect diagnostics explain transport degradation", func() (bool, string) {
		diag, err := client.GetDiagnostics(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDiagnosticsRequest{}))
		if err != nil {
			return false, err.Error()
		}
		health := diag.Msg.GetDiagnostics().GetHealth()
		if health.GetState() != "degraded" || health.GetPrimaryReason() == nil {
			return false, health.GetState()
		}
		domain := health.GetPrimaryReason().GetDomain()
		if domain != "boot" && domain != "transport" {
			return false, domain
		}
		for _, subsystem := range health.GetSubsystems() {
			if subsystem.GetDomain() != "transport" || subsystem.GetReason() == nil {
				continue
			}
			if subsystem.GetReason().GetDetail() == "" {
				return false, "missing transport subsystem detail"
			}
			if subsystem.GetReason().GetCode() != "transport.bootstrap.degraded" && subsystem.GetReason().GetCode() != "transport.bootstrap.fetch_failed" && subsystem.GetReason().GetCode() != "transport.bootstrap.empty" {
				return false, subsystem.GetReason().GetCode()
			}
			if detail := subsystem.GetReason().GetDetail(); !strings.HasPrefix(detail, "profile tcp_only, mode restricted_defense:") {
				return false, detail
			}
			return true, ""
		}
		return false, "missing transport subsystem detail"
	})
}

func TestConnectRPCProjectsTCPWSSProfileTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "LCI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "transport-variant"},
		Speed:       "default",
		Environment: "local",
	})

	certPath, keyPath := testkit.WriteWSSCert(t)
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name:        "connect-transport-wss",
		NodeProfile: transport.NodeProfileServiceNode,
		Boot:        runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:        runtimeinfra.DataConfig{Dir: t.TempDir()},
		Transport: runtimeinfra.TransportConfig{
			Profile:             transport.ProfileTCPWSS,
			WSSPort:             testkit.ReserveLoopbackTCPPort(t),
			WSSCertPath:         certPath,
			WSSKeyPath:          keyPath,
			WSSCAPath:           testkit.WSSCAPath(certPath),
			WSSAdvertiseAddress: "127.0.0.1",
		},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)

	testkit.WaitForCondition(t, 5*time.Second, "connect status shows tcp_wss transport truth", func() (bool, string) {
		resp, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
		if err != nil {
			return false, err.Error()
		}
		snap := resp.Msg.GetSnapshot()
		if snap.GetNode().GetState() != "ready" || snap.GetTrans().GetState() != "ready" {
			return false, snap.GetNode().GetState() + "/" + snap.GetTrans().GetState()
		}
		if snap.GetTransport().GetProfile() != "tcp_wss" || snap.GetTransport().GetHealth() != "ready" {
			return false, snap.GetTransport().GetProfile() + "/" + snap.GetTransport().GetHealth()
		}
		if len(snap.GetTransport().GetActiveFamilies()) != 2 {
			return false, strings.Join(snap.GetTransport().GetActiveFamilies(), ",")
		}
		return true, ""
	})

	status, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"tcp", "wss"}, status.Msg.GetSnapshot().GetTransport().GetActiveFamilies())
	require.Contains(t, status.Msg.GetSnapshot().GetTransport().GetSuppressedFamilies(), "quic")
	require.Equal(t, "startup_default", status.Msg.GetSnapshot().GetTransport().GetSwitchReason())
	require.Equal(t, "steady", status.Msg.GetSnapshot().GetTransport().GetMode())
	require.Empty(t, status.Msg.GetSnapshot().GetTransport().GetReducedFeatures())
}

func TestConnectRPCReadRequiresExactAction(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "LCI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "connect-authz",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	mux := http.NewServeMux()
	path, handler, err := rpcadapter.NewHandler(testkit.ConnectDependencies(rt.Runtime), localauth.Config{
		Token:        "limited-token",
		SubjectID:    "connect-test",
		Capabilities: []string{"node.status"},
	})
	require.NoError(t, err)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := cliclient.NewService(srv.Client(), srv.URL, connect.WithGRPC())
	req := connect.NewRequest(&ardentsv1.GetDataInventoryRequest{})
	req.Header().Set("Authorization", "Bearer limited-token")
	_, err = client.GetDataInventory(context.Background(), req)
	require.Error(t, err)

	connectErr, ok := errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestConnectRPCDataRoundTripAndErrors(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "LCI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "connect",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{
			Dir:                      t.TempDir(),
			DefaultLocalRetentionTTL: time.Hour,
			DefaultRelayRetentionTTL: time.Hour,
			MaxRelayRetentionBytes:   1024,
		},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)

	emptyInventory, err := client.GetDataInventory(
		context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDataInventoryRequest{}),
	)
	require.NoError(t, err)
	require.Zero(t, emptyInventory.Msg.GetObjects())
	require.Zero(t, emptyInventory.Msg.GetManifests())
	require.Zero(t, emptyInventory.Msg.GetBlobs())

	withPayload, err := client.PublishBlob(context.Background(), testkit.AuthorizedRequest(&ardentsv1.PublishBlobRequest{
		Blob: &ardentsv1.BlobSnapshot{MediaType: "text/plain", Payload: []byte("hello")},
	}))
	require.NoError(t, err)
	require.Equal(t, "available-local", withPayload.Msg.GetState())

	publishedBlob, err := client.PublishBlob(context.Background(), testkit.AuthorizedRequest(&ardentsv1.PublishBlobRequest{
		Blob: &ardentsv1.BlobSnapshot{MediaType: "application/octet-stream", Hash: "sha256:connect-data"},
	}))
	require.NoError(t, err)

	manifest, err := client.PublishManifest(context.Background(), testkit.AuthorizedRequest(&ardentsv1.PublishManifestRequest{
		Manifest: &ardentsv1.ManifestSnapshot{
			Kind:      "message-attachment",
			Owner:     "node",
			Access:    "participants",
			Retention: "temporary",
			Encrypted: true,
			Refs:      []*ardentsv1.RefSnapshot{{Kind: "blob", Id: publishedBlob.Msg.GetReference()}},
		},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, manifest.Msg.GetId())

	_, err = client.RetainBlob(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RetainBlobRequest{
		Id:        publishedBlob.Msg.GetReference(),
		ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
	}))
	require.Error(t, err)

	inventory, err := client.GetDataInventory(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDataInventoryRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 1, inventory.Msg.GetManifests())
	require.EqualValues(t, 2, inventory.Msg.GetBlobs())
	require.EqualValues(t, 1, inventory.Msg.GetLocalBlobs())

	_, found := rt.Data.GetObject("missing")
	require.False(t, found)

	_, err = client.PinBlob(context.Background(), testkit.AuthorizedRequest(&ardentsv1.PinBlobRequest{
		Id: "missing-blob",
	}))
	require.Error(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())

	_, err = client.GetDataInventory(context.Background(), connect.NewRequest(&ardentsv1.GetDataInventoryRequest{}))
	require.Error(t, err)
	connectErr, ok = errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())

	bareTokenReq := connect.NewRequest(&ardentsv1.GetDataInventoryRequest{})
	bareTokenReq.Header().Set("Authorization", testkit.ConnectAuthConfig().Token)
	_, err = client.GetDataInventory(context.Background(), bareTokenReq)
	require.Error(t, err)
	connectErr, ok = errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())

	_, found = rt.Data.GetObject("missing")
	require.False(t, found)
}

func TestConnectRPCMutationsPreserveConflictAndNotFoundCodes(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "LCI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "connect-workload-errors",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)
	spec := &ardentsv1.WorkloadSpecSnapshot{
		Id:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  helperProcessConfig(t, "sleep"),
		Desired: "present",
	}
	_, err := client.RegisterWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RegisterWorkloadRequest{Spec: spec}))
	require.NoError(t, err)

	_, err = client.RegisterWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RegisterWorkloadRequest{Spec: spec}))
	require.Error(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodeAlreadyExists, connectErr.Code())

	_, err = client.RestartWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RestartWorkloadRequest{Id: "missing-workload"}))
	require.Error(t, err)
	connectErr, ok = errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func helperProcessConfig(t *testing.T, mode string) string {
	t.Helper()
	return testkit.HelperProcessConfig(t, mode)
}
