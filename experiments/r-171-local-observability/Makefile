.PHONY: tools-install

tools-install:
ifeq ($(COLLECTOR),otel)
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