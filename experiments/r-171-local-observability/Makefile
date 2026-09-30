.PHONY: tools-install

tools-install:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File install-images.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)"
.PHONY: probe
probe:
	powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot "$(EVIDENCE_ROOT)" -RunName "$(RUN_NAME)"
