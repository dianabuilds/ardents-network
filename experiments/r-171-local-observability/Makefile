.PHONY: tools-install

tools-install:
ifeq ($(COLLECTOR),monitor-tool)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-monitor.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" $(if $(filter 1,$(NODE_FIXTURE)),-NodeFixture,)
else ifeq ($(COLLECTOR),browser-relay)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-browser-relay.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),prometheus-source)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-prometheus-source.ps1 -PrimaryRoot "$(PRIMARY_ROOT)" -ArtifactRoot "$(ARTIFACT_ROOT)" -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),grafana-plugin-trees)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-plugin-trees.ps1 -ArtifactRoot "$(ARTIFACT_ROOT)" -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),loki-source)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-loki-source.ps1 -PrimaryRoot "$(ARTIFACT_ROOT)" -PluginRoot "$(PLUGIN_ROOT)" -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),grafana-plugins)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-plugins.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),otel-minimal-image)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-collector-image.ps1 -ArtifactRoot "$(ARTIFACT_ROOT)" -EvidenceRoot "$(EVIDENCE_ROOT)" -Variant minimal
else ifeq ($(COLLECTOR),otel-minimal)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-minimal.ps1 -ArtifactRoot "$(ARTIFACT_ROOT)" -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),otel-builder)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-builder.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),otel-image)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-collector-image.ps1 -ArtifactRoot "$(ARTIFACT_ROOT)" -EvidenceRoot "$(EVIDENCE_ROOT)"
else ifeq ($(COLLECTOR),otel)
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-collector.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)"
else
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-images.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)"
endif
.PHONY: probe
probe:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)"

.PHONY: probe-restart
probe-restart:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -RestartProbe

.PHONY: probe-resources
probe-resources:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -RestartProbe -ResourceProbe

.PHONY: probe-backend-outage
probe-backend-outage:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -RestartProbe -BackendProbe
.PHONY: probe-storage-pressure
probe-storage-pressure:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -RestartProbe -StorageProbe

.PHONY: probe-otel
probe-otel:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -Collector otel -RestartProbe

.PHONY: probe-otel-resources
probe-otel-resources:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -Collector otel -RestartProbe -ResourceProbe

.PHONY: probe-otel-backend-outage
probe-otel-backend-outage:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -Collector otel -RestartProbe -BackendProbe

.PHONY: probe-otel-minimal-backend-outage
probe-otel-minimal-backend-outage:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -Collector otel -MinimalCollector -RestartProbe -BackendProbe

.PHONY: probe-otel-retry-exhaustion
probe-otel-retry-exhaustion:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -Collector otel -MinimalCollector -BackendProbe -RetryExhaustionProbe -PatchedPluginRoot "$(PLUGIN_ROOT)"

.PHONY: probe-otel-collector-storage
probe-otel-collector-storage:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -Collector otel -MinimalCollector -RestartProbe -CollectorStorageProbe -PatchedPluginRoot "$(PLUGIN_ROOT)"
.PHONY: probe-otel-persistent
probe-otel-persistent:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)" -Collector otel -MinimalCollector -RestartProbe -PersistentProbe -PatchedPluginRoot "$(PLUGIN_ROOT)"
