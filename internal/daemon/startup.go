package daemon

import (
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityapi "ardents/internal/identity"
	transport "ardents/internal/network"
	domainworkload "ardents/internal/workload/registry"
	"context"
	"crypto/ed25519"
	"fmt"
	"maps"
	"strings"
)

func CompleteBoot(
	diag *diagnostics.Recorder,
	transportBoot transport.BootstrapStatus,
	setDiscoveryDegraded func(string),
	moveLifecycle func(string),
	retainCurrentHealth func(),
) string {
	if transportBoot.State == "degraded" {
		setDiscoveryDegraded(transportBoot.Reason)
	}

	next := "ready"
	switch diag.Health().State {
	case diagnostics.HealthFailed:
		next = "failed"
	case diagnostics.HealthDegraded:
		next = "degraded"
	}
	moveLifecycle(next)
	if next == "ready" {
		retainCurrentHealth()
	}
	return next
}

func DegradeTransportBootstrap(
	diag *diagnostics.Recorder,
	cfgName string,
	code, summary, detail, impact string,
	payload map[string]any,
	adoptPrimary func(domain string, state string, reason *diagnostics.Reason),
	moveLifecycle func(string),
) {
	reason := &diagnostics.Reason{
		Code:                   code,
		Domain:                 "transport",
		Summary:                summary,
		Detail:                 detail,
		Impact:                 impact,
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               cfgName,
	}
	diag.SetSubsystem("transport", diagnostics.HealthDegraded, reason)
	if diag.Health().State != diagnostics.HealthFailed {
		adoptPrimary("transport", diagnostics.HealthDegraded, reason)
		moveLifecycle("degraded")
	}
	diag.RecordEvent("transport", "degraded", cfgName, summary, code, cloneMap(payload))
}

func DegradeDiscoveryImport(
	diag *diagnostics.Recorder,
	recordID string,
	detail string,
	setDiscoveryDegraded func(string),
	adoptPrimary func(domain string, state string, reason *diagnostics.Reason),
	moveLifecycle func(string),
) {
	summary := "bootstrap discovery import was degraded"
	reason := &diagnostics.Reason{
		Code:                   "discovery.bootstrap.import_degraded",
		Domain:                 "discovery",
		Summary:                summary,
		Detail:                 detail,
		Impact:                 "remote discovery catalog is incomplete",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               recordID,
	}
	setDiscoveryDegraded(detail)
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	if diag.Health().State != diagnostics.HealthFailed {
		adoptPrimary("discovery", diagnostics.HealthDegraded, reason)
		moveLifecycle("degraded")
	}
	diag.RecordEvent("discovery", "bootstrap_import_degraded", recordID, summary, reason.Code, map[string]any{"detail": detail})
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	maps.Copy(out, in)
	return out
}

func ImportBootstrapEntries(
	localPrincipal string,
	entries []discovery.Entry,
	importRecord func(discovery.Record) (bool, error),
	degradeImport func(recordID, detail string),
	syncTrust func(),
) bool {
	hadImportErrors := false
	for _, entry := range entries {
		if entry.Record.NodeID() == localPrincipal {
			continue
		}
		applied, err := importRecord(entry.Record)
		if err != nil {
			hadImportErrors = true
			degradeImport(entry.Record.RecordID(), err.Error())
			continue
		}
		if applied {
			continue
		}
	}
	syncTrust()
	return hadImportErrors
}

func NetworkBootstrapSources(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if strings.HasPrefix(trimmed, "/") {
			out = append(out, trimmed)
		}
	}
	return out
}

func RecordBootstrapDial(rec *diagnostics.Recorder, nodeName string, report transport.BootstrapDialReport) {
	if rec == nil {
		return
	}
	if report.Success {
		rec.RecordEvent("transport", "bootstrap_dial_succeeded", nodeName, "bootstrap peer dial succeeded", "", map[string]any{
			"peer": report.Peer,
		})
		return
	}
	rec.RecordEvent("transport", "bootstrap_dial_failed", nodeName, "bootstrap peer dial failed", "transport.bootstrap.dial_failed", map[string]any{
		"peer":   report.Peer,
		"detail": report.Detail,
	})
}

func LoadDiagnosticsForStartup(
	diag *diagnostics.Recorder,
	fail func(code, domain, summary, detail, impact, recovery string),
) bool {
	if err := diag.Load(); err != nil {
		if ledgerErr, ok := diagnostics.IsCorruptLedger(err); ok {
			return handleCorruptDiagnosticsLedger(diag, fail, ledgerErr)
		}
		failDiagnosticsLedgerLoad(fail, err.Error())
		return false
	}
	retainLoadedDiagnosticsHealth(diag)
	return true
}

func handleCorruptDiagnosticsLedger(
	diag *diagnostics.Recorder,
	fail func(code, domain, summary, detail, impact, recovery string),
	ledgerErr *diagnostics.CorruptLedgerError,
) bool {
	diag.SetSubsystem("diagnostics", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:     "diagnostics.ledger.corrupt",
		Domain:   "diagnostics",
		Summary:  "diagnostics ledger needed recovery",
		Detail:   ledgerErr.Error(),
		Impact:   "pending operation history may be incomplete",
		Recovery: "automatic",
	})
	if ledgerErr.Fatal {
		failDiagnosticsLedgerLoad(fail, ledgerErr.Error())
		return false
	}
	return true
}

func failDiagnosticsLedgerLoad(
	fail func(code, domain, summary, detail, impact, recovery string),
	detail string,
) {
	fail(
		"diagnostics.ledger.load_failed",
		"diagnostics",
		"diagnostics ledger load failed",
		detail,
		"node recovery state is unavailable",
		"restart_required",
	)
}

func retainLoadedDiagnosticsHealth(diag *diagnostics.Recorder) {
	loaded := diag.Health()
	if loaded.PrimaryReason != nil || len(loaded.Subsystems) != 0 || loaded.State != diagnostics.HealthReady {
		diag.RetainCurrentHealth()
	}
}

func PublishDiscoveryForStartup(
	ctx context.Context,
	_ ed25519.PrivateKey,
	refreshNetworkPublication func(context.Context) error,
	bootstrapDiscovery func(context.Context) error,
) error {
	if err := refreshNetworkPublication(ctx); err != nil {
		return err
	}
	return bootstrapDiscovery(ctx)
}

func StartWorkloadsForStartup(
	ctx context.Context,
	workloadSpecs []domainworkload.Spec,
	seedWorkloads func(context.Context, []domainworkload.Spec) error,
) error {
	return seedWorkloads(ctx, append([]domainworkload.Spec(nil), workloadSpecs...))
}

func LoadStartupState(
	loadState func() error,
	loadDiscovery func() error,
	loadData func() error,
	loadWorkloads func() error,
) error {
	if err := loadState(); err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	if err := loadDiscovery(); err != nil {
		return fmt.Errorf("load discovery: %w", err)
	}
	if err := loadData(); err != nil {
		return fmt.Errorf("load data: %w", err)
	}
	if err := loadWorkloads(); err != nil {
		return fmt.Errorf("load workloads: %w", err)
	}
	return nil
}

func InitializeIdentityForStartup(
	ensureNode func() (identityapi.Summary, ed25519.PrivateKey, error),
	setPrivate func(ed25519.PrivateKey),
	setLocalDataNodeID func(string) error,
	trustLocalPrincipal func(string, string) error,
	loadDiscovery func() error,
	syncTrustDiagnostics func(),
) error {
	summary, privateKey, err := ensureNode()
	if err != nil {
		return err
	}
	setPrivate(privateKey)
	if err := setLocalDataNodeID(summary.Principal); err != nil {
		return err
	}
	if err := trustLocalPrincipal(summary.Principal, summary.PublicKey); err != nil {
		return err
	}
	if err := loadDiscovery(); err != nil {
		return fmt.Errorf("load discovery: %w", err)
	}
	syncTrustDiagnostics()
	return nil
}

func RunStartupStep(
	diag *diagnostics.Recorder,
	kind, domain, resource string,
	recoverable bool,
	recoveryAction string,
	fail func(code, domain, summary, detail, impact, recovery string),
	fn func() error,
) bool {
	op := diag.BeginOperation(kind, domain, resource, recoverable, recoveryAction)
	if err := fn(); err != nil {
		diag.FailOperation(op.ID, err.Error())
		code := "node." + strings.ReplaceAll(kind, ".", "_") + ".failed"
		summary := "startup step failed"
		if kind == "node.startup.state_load" {
			code = "node.state.load_failed"
			summary = "state load failed"
		}
		fail(code, domain, summary, err.Error(), "node startup could not complete", "restart_required")
		return false
	}
	diag.CompleteOperation(op.ID, kind+" completed")
	return true
}

func (m *RuntimeManager) transportProfilePayloadLocked() map[string]any {
	snapshot := m.trans.ProfileSnapshot()
	return map[string]any{
		"profile":              string(snapshot.Profile),
		"mode":                 string(snapshot.Mode),
		"health":               string(snapshot.Health),
		"active_families":      transportFamilies(snapshot.ActiveFamilies),
		"suppressed_families":  transportFamilies(snapshot.SuppressedFamilies),
		"switch_reason":        string(snapshot.SwitchReason),
		"switch_automatic":     snapshot.SwitchAutomatic,
		"reduced_capabilities": append([]string(nil), snapshot.ReducedCapabilities...),
		"recovery_state":       string(snapshot.RecoveryState),
	}
}

func transportFamilies(items []transport.Family) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}

func (m *RuntimeManager) bootstrapDiscoveryLocked(ctx context.Context) error {
	sources := NetworkBootstrapSources(m.bootSources)
	if len(sources) == 0 {
		m.diag.SetSubsystem("transport", diagnostics.HealthReady, nil)
		return nil
	}
	result, err := discovery.FetchPrivateRecords(ctx, sources, m.privacy, messagingCarrier{m.trans})
	if err != nil {
		m.degradeTransportBootstrapLocked(
			"transport.bootstrap.fetch_failed",
			err.Error(),
			"bootstrap peer records could not be retrieved",
			"node remains controllable but remote discovery is incomplete",
		)
		return nil
	}
	if m.stopAfterPrivateBootstrapFailureLocked(result) {
		return nil
	}
	if len(result.Entries) == 0 && result.Replayed == 0 && len(m.disco.Entries()) == 0 {
		m.degradeTransportBootstrapLocked(
			"transport.bootstrap.empty",
			"no discovery records returned by bootstrap peers",
			"bootstrap peers did not provide discovery records",
			"node remains controllable but remote discovery is incomplete",
		)
		return nil
	}
	hadImportErrors := m.importBootstrapEntriesLocked(result.Entries)
	if !hadImportErrors && result.Reason == "" {
		m.setDiscoveryReadyLocked()
	}
	m.diag.SetSubsystem("transport", diagnostics.HealthReady, nil)
	m.diag.RecordEvent("transport", "bootstrap_synced", m.cfgName, "bootstrap peer records synchronized", "", map[string]any{
		"records": len(result.Entries), "rejected": result.Rejected, "replayed": result.Replayed,
	})
	return nil
}

func (m *RuntimeManager) refreshPrivateDiscoveryLocked(ctx context.Context) error {
	sources := NetworkBootstrapSources(m.bootSources)
	if len(sources) == 0 {
		return nil
	}
	result, err := discovery.FetchPrivateRecords(ctx, sources, m.privacy, messagingCarrier{m.trans})
	if err != nil {
		return err
	}
	if result.Reason != "" {
		m.degradeDiscoveryPrivacyLocked(result.Reason, result.Rejected)
		if len(result.Entries) == 0 {
			return nil
		}
	}
	m.importBootstrapEntriesLocked(result.Entries)
	m.diag.RecordEvent("discovery", "remote_refreshed", m.cfgName, "remote discovery records refreshed", "", map[string]any{
		"records": len(result.Entries), "rejected": result.Rejected, "replayed": result.Replayed,
	})
	return nil
}

func (m *RuntimeManager) stopAfterPrivateBootstrapFailureLocked(result discovery.PrivateFetchResult) bool {
	if result.Reason == "" {
		return false
	}
	m.degradeDiscoveryPrivacyLocked(result.Reason, result.Rejected)
	if len(result.Entries) > 0 {
		return false
	}
	m.diag.SetSubsystem("transport", diagnostics.HealthReady, nil)
	return true
}

func (m *RuntimeManager) degradeDiscoveryPrivacyLocked(code string, rejected int) {
	DegradeDiscoveryPrivacy(
		m.diag, m.cfgName, code, rejected,
		m.setDiscoveryDegradedLocked,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}

func (m *RuntimeManager) importBootstrapEntriesLocked(entries []discovery.Entry) bool {
	return ImportBootstrapEntries(
		m.ident.NodeSummary().Principal,
		entries,
		func(record discovery.Record) (bool, error) {
			result, err := m.disco.Import(record, discoveryrecord.Bootstrap)
			if err != nil {
				return false, err
			}
			return result.Applied, nil
		},
		m.degradeDiscoveryImportLocked,
		m.SyncDiscoveryTrustDiagnosticsLocked,
	)
}

func (m *RuntimeManager) degradeTransportBootstrapLocked(code, detail, summary, impact string) {
	snapshot := m.trans.ProfileSnapshot()
	payload := m.transportProfilePayloadLocked()
	payload["detail"] = detail
	qualifiedDetail := "profile " + string(snapshot.Profile) + ", mode " + string(snapshot.Mode) + ": " + detail
	DegradeTransportBootstrap(
		m.diag,
		m.cfgName,
		code,
		summary,
		qualifiedDetail,
		impact,
		payload,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}

func (m *RuntimeManager) degradeDiscoveryImportLocked(recordID, detail string) {
	DegradeDiscoveryImport(
		m.diag,
		recordID,
		detail,
		m.setDiscoveryDegradedLocked,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}

func (n *Node) handleBootstrapDialLocked(report transport.BootstrapDialReport) {
	n.mu.Lock()
	defer n.mu.Unlock()
	RecordBootstrapDial(n.diag, n.cfg.Name, report)
}

func (n *Node) onReachabilityChanged() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.cancel == nil {
		return
	}
	n.runtimeMgr.RefreshDiscoveryPublicationLocked(context.Background())
}
