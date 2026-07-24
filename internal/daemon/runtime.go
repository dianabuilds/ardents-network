package daemon

import (
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	"ardents/internal/identity"
	identitytrust "ardents/internal/identity/trust"
	networkprivacy "ardents/internal/messaging"
	transport "ardents/internal/network"
	"ardents/internal/policy"
	"ardents/internal/publication"
	"ardents/internal/workload"
	"ardents/internal/workload/execution"
	"ardents/internal/workload/registry"
	"context"
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"
)

type RuntimeManager struct {
	cfgName         string
	bootSources     []string
	workloadSpecs   []registry.Spec
	life            *diagnostics.Machine
	diag            *diagnostics.Recorder
	state           *identity.Store
	keys            identity.KeyStore
	boot            *BootStatus
	ident           identity.Service
	trust           *discovery.TrustEvaluator
	configuredTrust *identitytrust.Registry
	disco           *discovery.Service
	trans           transport.Service
	privacy         *networkprivacy.Channel
	data            *contentLifecycle
	workloads       *workload.Runtime
	publication     publication.Coordinator
	getPrivate      func() ed25519.PrivateKey
	setPrivate      func(ed25519.PrivateKey)
	publish         func(string, map[string]any)
}

func newRuntimeLifecycle(
	cfgName string, bootSources []string, workloadSpecs []registry.Spec,
	life *diagnostics.Machine, diag *diagnostics.Recorder,
	state *identity.Store, keys identity.KeyStore, boot *BootStatus,
	ident identity.Service, trustSvc *discovery.TrustEvaluator, configuredTrust *identitytrust.Registry,
	disco *discovery.Service, trans transport.Service,
	dataSvc *contentLifecycle, workloadRuntime *workload.Runtime, publicationMgr publication.Coordinator,
	getPrivate func() ed25519.PrivateKey,
	setPrivate func(ed25519.PrivateKey),
	publish func(string, map[string]any),
	privateChannels ...*networkprivacy.Channel,
) *RuntimeManager {
	var privacyChannel *networkprivacy.Channel
	if len(privateChannels) > 0 {
		privacyChannel = privateChannels[0]
	}
	return &RuntimeManager{
		cfgName:         cfgName,
		bootSources:     append([]string(nil), bootSources...),
		workloadSpecs:   append([]registry.Spec(nil), workloadSpecs...),
		life:            life,
		diag:            diag,
		state:           state,
		keys:            keys,
		boot:            boot,
		ident:           ident,
		trust:           trustSvc,
		configuredTrust: configuredTrust,
		disco:           disco,
		trans:           trans,
		privacy:         privacyChannel,
		data:            dataSvc,
		workloads:       workloadRuntime,
		publication:     publicationMgr,
		getPrivate:      getPrivate,
		setPrivate:      setPrivate,
		publish:         publish,
	}
}

const startupRollbackTimeout = 5 * time.Second

func (m *RuntimeManager) StartProcessLocked(
	ctx context.Context,
	startBlobExchange func(context.Context) error,
) (context.Context, context.CancelFunc, error) {
	networkCtx, cancel := context.WithCancel(context.Background())
	stopStartupCancel := context.AfterFunc(ctx, cancel)
	if err := m.StartLocked(networkCtx); err != nil {
		stopStartupCancel()
		cancel()
		return nil, nil, err
	}
	if err := startBlobExchange(networkCtx); err != nil {
		stopStartupCancel()
		return nil, nil, m.rollbackDataPlaneStartup(cancel, err)
	}
	stopStartupCancel()
	if err := ctx.Err(); err != nil {
		return nil, nil, m.rollbackDataPlaneStartup(cancel, err)
	}
	return networkCtx, cancel, nil
}

func (m *RuntimeManager) StopProcessLocked(ctx context.Context, cancel context.CancelFunc) error {
	err := m.StopLocked(ctx)
	if cancel != nil {
		cancel()
	}
	return err
}

func (m *RuntimeManager) StartDiscoveryRefreshLoop(
	ctx context.Context,
	configuredInterval time.Duration,
	refresh func(context.Context),
) {
	interval := DiscoveryRefreshInterval(configuredInterval)
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refresh(ctx)
			}
		}
	}()
}

func DiscoveryRefreshInterval(configuredInterval time.Duration) time.Duration {
	if configuredInterval > 0 {
		return configuredInterval
	}
	return discoveryrecord.LocalRecordTTL / 2
}

func (m *RuntimeManager) rollbackDataPlaneStartup(cancel context.CancelFunc, startErr error) error {
	rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), startupRollbackTimeout)
	stopErr := m.StopLocked(rollbackCtx)
	if m.trans.State() != "stopped" {
		if transportStopErr := m.trans.Stop(rollbackCtx); transportStopErr != nil {
			if stopErr == nil {
				stopErr = transportStopErr
			} else {
				stopErr = fmt.Errorf("%v; transport stop: %w", stopErr, transportStopErr)
			}
		}
	}
	rollbackCancel()
	cancel()
	if stopErr != nil {
		m.FailLocked(
			"node.data_plane.rollback_failed",
			"data",
			"data-plane startup rollback failed",
			fmt.Sprintf("start error: %v; rollback error: %v", startErr, stopErr),
			"node runtime may still hold partial startup state",
			"operator",
		)
		return fmt.Errorf("start data-plane exchange: %w; rollback runtime: %v", startErr, stopErr)
	}
	return fmt.Errorf("start data-plane exchange: %w", startErr)
}

const (
	StartupPhaseStateLoad = "node.startup.state_load"
	StartupPhaseIdentity  = "node.startup.identity"
	StartupPhaseDiscovery = "node.startup.discovery"
	StartupPhaseWorkloads = "node.startup.workloads"
	ShutdownPhaseNode     = "node.shutdown"
)

func (m *RuntimeManager) StartLocked(ctx context.Context) error {
	if m.life.State() == diagnostics.Ready || m.life.State() == diagnostics.Degraded {
		return nil
	}

	m.moveLifecycleLocked(diagnostics.Starting)
	if !m.loadDiagnosticsLocked() {
		return m.runtimeFailureLocked("start")
	}
	m.publish("node.starting", map[string]any{"id": m.cfgName, "state": diagnostics.Starting})
	m.diag.RecordEvent("node", "starting", m.cfgName, "node startup started", "", map[string]any{"id": m.cfgName})
	m.diag.MarkRecoveringExcept("", "operation recovered after restart")
	m.moveLifecycleLocked(diagnostics.Initializing)

	if !m.startupStateLoadLocked(ctx) {
		return m.runtimeFailureLocked("start")
	}

	var identPrivateKey ed25519.PrivateKey
	if !m.initializeIdentityLocked(ctx, &identPrivateKey) {
		return m.runtimeFailureLocked("start")
	}

	if err := m.trans.Start(ctx); err != nil {
		m.FailLocked("node.transport.start_failed", "transport", "transport start failed", err.Error(), "network plane unavailable", "restart_required")
		return m.runtimeFailureLocked("start")
	}
	m.setTransportHealthLocked()

	if !m.publishDiscoveryLocked(ctx, identPrivateKey) {
		return m.runtimeFailureLocked("start")
	}

	if !m.startupWorkloadsLocked(ctx) {
		return m.runtimeFailureLocked("start")
	}

	m.finishBootLocked(ctx)
	return m.runtimeFailureLocked("start")
}

func (m *RuntimeManager) runStartupStepLocked(_ context.Context, kind, domain, resource string, recoverable bool, recoveryAction string, fn func() error) bool {
	return RunStartupStep(
		m.diag,
		kind,
		domain,
		resource,
		recoverable,
		recoveryAction,
		m.FailLocked,
		fn,
	)
}

func (m *RuntimeManager) loadDiagnosticsLocked() bool {
	return LoadDiagnosticsForStartup(m.diag, m.FailLocked)
}

func (m *RuntimeManager) startupStateLoadLocked(ctx context.Context) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseStateLoad, "node", "state", false, "", func() error {
		return LoadStartupState(
			m.state.Load,
			func() error { return nil },
			m.data.Load,
			func() error { return m.workloads.LoadContext(ctx) },
		)
	})
}

func (m *RuntimeManager) initializeIdentityLocked(ctx context.Context, out *ed25519.PrivateKey) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseIdentity, "identity", "local", false, "", func() error {
		return InitializeIdentityForStartup(
			func() (identity.Summary, ed25519.PrivateKey, error) {
				summary, privateKey, err := m.ident.EnsureNode(m.state, m.keys)
				if err == nil {
					*out = privateKey
				}
				return summary, privateKey, err
			},
			m.setPrivate,
			m.data.SetLocalNodeID,
			func(principal, public string) error {
				return replaceTrustWithLocalPrincipal(m.trust, m.configuredTrust, principal, public)
			},
			m.disco.Load,
			m.SyncDiscoveryTrustDiagnosticsLocked,
		)
	})
}

func (m *RuntimeManager) publishDiscoveryLocked(ctx context.Context, privateKey ed25519.PrivateKey) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseDiscovery, "discovery", "records", false, "", func() error {
		if err := m.publication.RefreshNetworkPublicationLocked(ctx); err != nil {
			if !networkprivacy.IsChannelGrantFailure(err) {
				return err
			}
			m.degradeDiscoveryPrivacyLocked(networkprivacy.CodeOf(err), 0)
		}
		return PublishDiscoveryForStartup(
			ctx,
			privateKey,
			func(context.Context) error { return nil },
			m.bootstrapDiscoveryLocked,
		)
	})
}

func (m *RuntimeManager) startupWorkloadsLocked(ctx context.Context) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseWorkloads, "workload", "workloads", true, "restart node", func() error {
		return StartWorkloadsForStartup(
			ctx,
			m.workloadSpecs,
			m.workloads.SeedAndReconcile,
		)
	})
}

func (m *RuntimeManager) finishBootLocked(_ context.Context) {
	transportBoot := m.trans.BootstrapStatus()
	m.setTransportHealthLocked()
	m.diag.RecordEvent("transport", "profile_active", m.cfgName, "transport profile is active", "", m.transportProfilePayloadLocked())
	m.syncBootHealthLocked(transportBoot)
	CompleteBoot(
		m.diag,
		transportBoot,
		m.setDiscoveryDegradedLocked,
		m.moveLifecycleLocked,
		m.diag.RetainCurrentHealth,
	)
	m.publish("node.started", map[string]any{"id": m.cfgName, "state": m.life.State()})
	m.diag.RecordEvent("node", "started", m.cfgName, "node startup completed", diagnostics.CurrentPrimaryReasonCode(m.diag), map[string]any{"id": m.cfgName, "state": m.life.State()})
}

func (m *RuntimeManager) setTransportHealthLocked() {
	snapshot := m.trans.ProfileSnapshot()
	diagnostics.ApplyTransportHealth(m.diag, m.trans.State(), m.trans.Reason(), string(snapshot.Profile), string(snapshot.Mode))
}

func (m *RuntimeManager) StopLocked(ctx context.Context) error {
	stop := m.beginStopLocked()
	if stop.noop {
		return nil
	}
	return stop.finishLocked(ctx, stop.runExternal(ctx))
}

type stopTransaction struct {
	manager     *RuntimeManager
	operationID string
	noop        bool
}

func (m *RuntimeManager) beginStopLocked() stopTransaction {
	if m.life.State() == diagnostics.Stopped {
		return stopTransaction{manager: m, noop: true}
	}
	m.moveLifecycleLocked(diagnostics.Stopping)
	m.publish("node.stopping", map[string]any{"id": m.cfgName, "state": diagnostics.Stopping})
	op := m.diag.BeginOperation(ShutdownPhaseNode, "node", m.cfgName, true, "restart node")
	m.diag.MarkRecoveringExcept(op.ID, "operation interrupted by shutdown")
	return stopTransaction{manager: m, operationID: op.ID}
}

func (s stopTransaction) runExternal(ctx context.Context) error {
	if s.noop {
		return nil
	}
	return s.manager.workloads.Shutdown(ctx)
}

func (s stopTransaction) finishLocked(ctx context.Context, workloadErr error) error {
	if s.noop {
		return nil
	}
	m := s.manager
	if workloadErr != nil {
		m.diag.FailOperation(s.operationID, workloadErr.Error())
		m.FailLocked("node.shutdown.workloads_failed", "workload", "workload shutdown failed", workloadErr.Error(), "node stop left workload execution uncertain", "operator")
		return m.runtimeFailureLocked("stop")
	}
	if err := m.publication.WithdrawNetworkPublicationLocked(ctx); err != nil {
		m.diag.FailOperation(s.operationID, err.Error())
		m.FailLocked("node.shutdown.publication_failed", "discovery", "discovery shutdown publication failed", err.Error(), "stopped node may remain discoverable on the network", "operator")
		return m.runtimeFailureLocked("stop")
	}

	if err := m.trans.Stop(ctx); err != nil {
		m.diag.FailOperation(s.operationID, err.Error())
		m.FailLocked("node.shutdown.failed", "node", "shutdown failed", err.Error(), "node safety is uncertain", "terminal")
		return m.runtimeFailureLocked("stop")
	}
	m.diag.CompleteOperation(s.operationID, "node shutdown completed")
	m.clearRuntimeHealthForStopLocked()
	m.moveLifecycleLocked(diagnostics.Stopped)
	m.publish("node.stopped", map[string]any{"id": m.cfgName, "state": diagnostics.Stopped})
	m.diag.RecordEvent("node", "stopped", m.cfgName, "node shutdown completed", "", map[string]any{"id": m.cfgName})
	return nil
}

func (m *RuntimeManager) clearRuntimeHealthForStopLocked() {
	m.boot.SetResult(StoppedBootResult())
	diagnostics.ClearRuntimeHealthForStop(m.diag)
}

func (m *RuntimeManager) runtimeFailureLocked(action string) error {
	detail := ""
	if health := m.diag.Health(); health.PrimaryReason != nil {
		detail = health.PrimaryReason.Detail
	}
	return diagnostics.RuntimeFailure(action, m.life.State() == diagnostics.Failed, detail)
}

func (m *RuntimeManager) moveLifecycleLocked(next string) {
	if err := m.life.Move(next); err != nil {
		m.diag.RecordEvent("node", "lifecycle_transition_rejected", m.cfgName, "lifecycle transition rejected", "node.lifecycle.transition_rejected", map[string]any{
			"from":  m.life.State(),
			"to":    next,
			"error": err.Error(),
		})
	}
}

func (m *RuntimeManager) FailLocked(code, domain, summary, detail, impact, recovery string) {
	m.diag.SetPrimary(diagnostics.HealthFailed, &diagnostics.Reason{
		Code:                   code,
		Domain:                 domain,
		Summary:                summary,
		Detail:                 detail,
		Impact:                 impact,
		Recovery:               recovery,
		OperatorActionRequired: true,
		Resource:               m.cfgName,
	})
	m.diag.SetSubsystem(domain, diagnostics.HealthFailed, &diagnostics.Reason{
		Code:                   code,
		Domain:                 domain,
		Summary:                summary,
		Detail:                 detail,
		Impact:                 impact,
		Recovery:               recovery,
		OperatorActionRequired: true,
		Resource:               m.cfgName,
	})
	if m.life.State() == diagnostics.Starting {
		m.moveLifecycleLocked(diagnostics.Initializing)
	}
	m.moveLifecycleLocked(diagnostics.Failed)
	m.publish("node.failed", map[string]any{"id": m.cfgName, "reason": detail, "code": code})
	m.diag.RecordEvent("node", "failed", m.cfgName, summary, code, map[string]any{"detail": detail})
}

func (m *RuntimeManager) adoptPrimaryReasonLocked(domain string, state string, reason *diagnostics.Reason) {
	diagnostics.AdoptPrimaryReason(m.diag, domain, state, reason)
}

func (m *RuntimeManager) restorePrimaryReasonLocked(domain string) {
	diagnostics.RestorePrimaryReason(m.diag, domain)
}

func (m *RuntimeManager) promoteSubsystemPrimaryLocked(domain string) {
	diagnostics.PromoteSubsystemPrimary(m.diag, domain)
}

func (m *RuntimeManager) SyncObservedTruthLocked() {
	if m.life == nil || m.diag == nil || m.boot == nil || m.trans == nil {
		return
	}
	if !diagnostics.AllowsObservedSync(m.life.State(), diagnostics.Ready, diagnostics.Degraded) {
		return
	}

	transportBoot := m.trans.BootstrapStatus()
	m.SyncDiscoveryTrustDiagnosticsLocked()
	m.setTransportHealthLocked()
	m.syncBootHealthLocked(transportBoot)
	m.syncPrimaryReasonLocked()
	m.syncLifecycleStateLocked()
}

func (m *RuntimeManager) syncBootHealthLocked(transportBoot transport.BootstrapStatus) {
	result := BootResultFromTransport(transportBoot.Joined, transportBoot.State, transportBoot.Reason)
	m.boot.SetResult(result)
	diagnostics.SyncBootHealth(m.diag, result.State, result.Reason)
}

func (m *RuntimeManager) syncPrimaryReasonLocked() {
	diagnostics.SyncPrimaryReason(m.diag)
}

func (m *RuntimeManager) syncLifecycleStateLocked() {
	diagnostics.SyncLifecycleState(m.diag, m.moveLifecycleLocked)
}

func (m *RuntimeManager) RefreshDiscoveryPublicationLocked(ctx context.Context) {
	if err := m.workloads.SyncObserved(ctx); err != nil {
		m.recordDiscoveryRefreshFailureLocked(err)
		return
	}
	m.refreshDiscoveryPublicationAfterObservationLocked(ctx)
}

func (m *RuntimeManager) refreshDiscoveryPublicationAfterObservationLocked(ctx context.Context) {
	if err := m.publication.RefreshNetworkPublicationLocked(ctx); err != nil {
		if networkprivacy.IsChannelGrantFailure(err) {
			current := diagnostics.SubsystemReasonCode(m.diag.Health(), "discovery")
			if current == "" || strings.HasPrefix(current, "privacy.channel_grant.") {
				m.degradeDiscoveryPrivacyLocked(networkprivacy.CodeOf(err), 0)
			}
			return
		}
		m.recordDiscoveryRefreshFailureLocked(err)
		return
	}
	if err := m.refreshPrivateDiscoveryLocked(ctx); err != nil {
		m.recordDiscoveryRefreshFailureLocked(err)
		return
	}
	m.clearDiscoveryRefreshFailureLocked()
}

func (m *RuntimeManager) recordDiscoveryRefreshFailureLocked(err error) {
	diagnostics.RecordDiscoveryRefreshFailure(
		m.diag,
		m.cfgName,
		err,
		m.setDiscoveryDegradedLocked,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
		m.publish,
	)
}

func (m *RuntimeManager) clearDiscoveryRefreshFailureLocked() {
	diagnostics.ClearDiscoveryRefreshFailure(
		m.diag,
		m.setDiscoveryReadyLocked,
		m.restorePrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}

func (n *Node) requireDataMutableLocked(action string) error {
	return n.requireProcessMutableLocked(action)
}

func (n *Node) requireProcessMutableLocked(action string) error {
	switch n.life.State() {
	case diagnostics.Stopped:
		return fmt.Errorf("%s rejected: node is stopped", action)
	case diagnostics.Failed:
		return fmt.Errorf("%s rejected: node is failed", action)
	}
	return nil
}

func (n *Node) emitPolicyDeniedLocked(resource, action string, err error) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	n.publishLocked("policy.denied", map[string]any{"id": resource, "action": action, "reason": reason, "resource": resource})
}

func (n *Node) handleDataPrivacyFailureLocked(err error) bool {
	if !networkprivacy.IsChannelGrantFailure(err) {
		return false
	}
	code := networkprivacy.CodeOf(err)
	reason := &diagnostics.Reason{
		Code: code, Domain: "data", Summary: "private data exchange is unavailable",
		Detail: err.Error(), Impact: "local data remains available but remote blob exchange is disabled",
		Recovery: "operator", OperatorActionRequired: true, Resource: n.cfg.Name,
	}
	n.diag.SetSubsystem("data", diagnostics.HealthDegraded, reason)
	if n.life.State() == diagnostics.Ready || n.life.State() == diagnostics.Initializing {
		if moveErr := n.life.Move(diagnostics.Degraded); moveErr != nil {
			n.diag.RecordEvent("data", "lifecycle_transition_failed", n.cfg.Name, moveErr.Error(), "data.privacy_lifecycle_failed", nil)
		}
	}
	n.publishLocked("data.privacy_degraded", map[string]any{"id": n.cfg.Name, "reason": code})
	n.diag.RecordEvent("data", "privacy_degraded", n.cfg.Name, reason.Summary, code, nil)
	return true
}

const workloadRefreshFailedCode = "workload.observation.refresh_failed"

func (h *workloadHealth) recordWorkloadRefreshFailureLocked(err error) {
	reason := &diagnostics.Reason{
		Code:                   workloadRefreshFailedCode,
		Domain:                 "workload",
		Summary:                "workload observation refresh failed",
		Detail:                 err.Error(),
		Impact:                 "workload and hosted service truth may be stale on operator surfaces",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               h.cfgName,
	}
	h.diag.SetSubsystem("workload", diagnostics.HealthDegraded, reason)
	if h.diag.Health().State != diagnostics.HealthFailed {
		h.adoptPrimaryReasonLocked("workload", diagnostics.HealthDegraded, reason)
		switch h.life.State() {
		case diagnostics.Ready, diagnostics.Degraded:
			h.moveLifecycleLocked(diagnostics.Degraded)
		}
	}
	h.publish("workload.refresh_failed", map[string]any{"id": h.cfgName, "error": err.Error()})
	h.diag.RecordEvent("workload", "refresh_failed", h.cfgName, "workload observation refresh failed", workloadRefreshFailedCode, map[string]any{"detail": err.Error()})
}

func (h *workloadHealth) clearWorkloadRefreshFailureLocked() {
	if workloadReasonCode(h.diag.Health()) != workloadRefreshFailedCode {
		return
	}
	h.diag.ClearSubsystem("workload")
	h.restorePrimaryReasonLocked("workload")
}

func (h *workloadHealth) evaluateWorkloadHealthLocked() {
	var reason *diagnostics.Reason
	for _, item := range h.workload.List() {
		if !workloadImpactsNode(item) {
			continue
		}
		if item.Observed == execution.ObservedRunning || item.Observed == execution.ObservedAccepted {
			continue
		}
		reason = &diagnostics.Reason{
			Code:                   "workload.hosted_service.degraded",
			Domain:                 "workload",
			Summary:                "node-owned hosted service is impaired",
			Detail:                 item.Reason,
			Impact:                 "hosted service publication is unavailable",
			Recovery:               "operator",
			OperatorActionRequired: true,
			Resource:               item.Spec.ID,
		}
		break
	}
	if reason == nil {
		h.diag.ClearSubsystem("workload")
		h.restorePrimaryReasonLocked("workload")
		return
	}
	h.diag.SetSubsystem("workload", diagnostics.HealthDegraded, reason)
	if h.diag.Health().State != diagnostics.HealthFailed {
		h.adoptPrimaryReasonLocked("workload", diagnostics.HealthDegraded, reason)
		h.moveLifecycleLocked(diagnostics.Degraded)
	}
}

func workloadReasonCode(health diagnostics.HealthSummary) string {
	for _, item := range health.Subsystems {
		if item.Domain != "workload" || item.Reason == nil {
			continue
		}
		return item.Reason.Code
	}
	return ""
}

func (h *workloadHealth) adoptPrimaryReasonLocked(domain string, state string, reason *diagnostics.Reason) {
	health := h.diag.Health()
	if health.PrimaryReason != nil && health.PrimaryReason.Domain != domain {
		return
	}
	h.diag.SetPrimary(state, reason)
}

func (h *workloadHealth) restorePrimaryReasonLocked(domain string) {
	health := h.diag.Health()
	if health.PrimaryReason == nil || health.PrimaryReason.Domain != domain {
		return
	}
	h.diag.ClearPrimary()
	for _, item := range h.diag.Health().Subsystems {
		if item.Reason == nil {
			continue
		}
		h.diag.SetPrimary(item.State, item.Reason)
		return
	}
}

func workloadImpactsNode(item execution.Status) bool {
	if item.Spec.Owner != "node" {
		return false
	}
	return len(item.Spec.Services) > 0
}

type workloadPublicationPort struct{ publication.Coordinator }

func (p workloadPublicationPort) ProjectStatus(item execution.Status) execution.Status {
	return p.EffectiveWorkloadStatus(item)
}

func (p workloadPublicationPort) SyncDesired(ctx context.Context) error {
	return p.SyncDesiredLocked(ctx)
}

func (p workloadPublicationPort) SyncLocalDesired() error { return p.SyncLocalDesiredLocked() }
func (p workloadPublicationPort) Capture() any            { return p.CaptureWorkloadPublicationSnapshotLocked() }

func (p workloadPublicationPort) Rollback(ctx context.Context, action string, cause error, snapshot any) error {
	typed, ok := snapshot.(publication.Snapshot)
	if !ok {
		return fmt.Errorf("invalid workload publication snapshot")
	}
	return p.RollbackWorkloadMutationLocked(ctx, action, cause, typed)
}

func (p workloadPublicationPort) HandleError(err error) error { return p.HandleSyncError(err) }

type workloadHealth struct {
	cfgName  string
	life     *diagnostics.Machine
	diag     *diagnostics.Recorder
	workload *execution.Service
	publish  func(string, map[string]any)
}

func newWorkloadRuntime(
	cfgName string,
	life *diagnostics.Machine,
	diag *diagnostics.Recorder,
	policyService policy.Policy,
	executionService *execution.Service,
	publicationService publication.Coordinator,
	publish func(string, map[string]any),
) *workload.Runtime {
	health := &workloadHealth{cfgName: cfgName, life: life, diag: diag, workload: executionService, publish: publish}
	return workload.NewRuntime(workload.RuntimeConfig{
		Execution:   executionService,
		Policy:      policyService,
		Publication: workloadPublicationPort{publicationService},
		Guard:       health.requireMutable,
		Hooks: workload.RuntimeHooks{
			RefreshFailed:     health.recordWorkloadRefreshFailureLocked,
			RefreshSucceeded:  health.clearWorkloadRefreshFailureLocked,
			StateChanged:      func([]execution.Status) { health.publishState() },
			EvaluateHealth:    func([]execution.Status) { health.evaluateWorkloadHealthLocked() },
			ShutdownFailed:    health.shutdownFailed,
			ShutdownSucceeded: health.shutdownSucceeded,
			PolicyDenied:      health.policyDenied,
		},
	})
}

func (h *workloadHealth) requireMutable(action string) error {
	switch h.life.State() {
	case diagnostics.Stopped:
		return fmt.Errorf("%s rejected: node is stopped", action)
	case diagnostics.Failed:
		return fmt.Errorf("%s rejected: node is failed", action)
	default:
		return nil
	}
}

func (h *workloadHealth) moveLifecycleLocked(next string) {
	if err := h.life.Move(next); err != nil {
		h.diag.RecordEvent("node", "lifecycle_transition_rejected", h.cfgName, "lifecycle transition rejected", "node.lifecycle.transition_rejected", map[string]any{
			"from": h.life.State(), "to": next, "error": err.Error(),
		})
	}
}

func (h *workloadHealth) policyDenied(resource, action string, err error) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	h.publish("policy.denied", map[string]any{"id": resource, "action": action, "reason": reason, "resource": resource})
}

func (h *workloadHealth) publishState() {
	for _, item := range h.workload.List() {
		h.publish("workload.updated", map[string]any{"id": item.Spec.ID, "observed": item.Observed, "desired": item.Spec.Desired})
	}
}

func (h *workloadHealth) shutdownFailed(err error) {
	h.diag.SetSubsystem("workload", diagnostics.HealthFailed, &diagnostics.Reason{
		Code: "workload.shutdown.failed", Domain: "workload", Summary: "workload shutdown failed",
		Detail: err.Error(), Impact: "node stop could not fully terminate local workloads",
		Recovery: "operator", OperatorActionRequired: true,
	})
}

func (h *workloadHealth) shutdownSucceeded() { h.diag.ClearSubsystem("workload") }
