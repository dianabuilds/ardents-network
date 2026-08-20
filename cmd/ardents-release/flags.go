package main

import "flag"

// offlineImportFlags is the parsed flag set for the offline-import
// subcommand.
type offlineImportFlags struct {
	stateRoot               string
	metadataDir             string
	rootPath                string
	targetPath              string
	artifactPath            string
	environment             string
	network                 string
	platform                string
	architecture            string
	refTime                 string
	protocolOverlappedSince string
	capacityReady           bool
	drainReady              bool
	emergencyExpiry         string
	emergencyReason         string
	buildSafetyNoNewAfter   string
	buildSafetyTermAfter    string
}

func newOfflineImportFlags() *offlineImportFlags {
	return &offlineImportFlags{}
}

func (raw *offlineImportFlags) register(flags *flag.FlagSet) {
	flags.StringVar(&raw.stateRoot, "state-root", "", "owned release-decision state root")
	flags.StringVar(&raw.metadataDir, "metadata-dir", "", "offline-import metadata directory")
	flags.StringVar(&raw.rootPath, "root", "", "path to the trusted root.json inside metadata-dir")
	flags.StringVar(&raw.targetPath, "target", "", "target path inside the top-level targets role")
	flags.StringVar(&raw.artifactPath, "artifact", "", "path to the artifact bytes")
	flags.StringVar(&raw.environment, "environment", "", "local environment marker")
	flags.StringVar(&raw.network, "network", "", "local network identity")
	flags.StringVar(&raw.platform, "platform", "", "local platform marker (e.g. windows-amd64)")
	flags.StringVar(&raw.architecture, "architecture", "", "local architecture marker")
	flags.StringVar(&raw.refTime, "ref-time", "", "UTC reference time in RFC3339")
	flags.StringVar(&raw.protocolOverlappedSince, "protocol-overlapped-since", "", "RFC3339 moment the current protocol entered overlap")
	flags.BoolVar(&raw.capacityReady, "capacity-ready", false, "current-generation capacity is qualified")
	flags.BoolVar(&raw.drainReady, "drain-ready", false, "drain reserve is qualified")
	flags.StringVar(&raw.emergencyExpiry, "emergency-expiry", "", "RFC3339 moment the 4-of-5 emergency expires")
	flags.StringVar(&raw.emergencyReason, "emergency-reason", "", "named safety reason for the 4-of-5 emergency")
	flags.StringVar(&raw.buildSafetyNoNewAfter, "build-safety-no-new-work-after", "", "RFC3339 no-new-work bound")
	flags.StringVar(&raw.buildSafetyTermAfter, "build-safety-terminate-after", "", "RFC3339 terminal bound")
}
