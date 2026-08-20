package main

import "flag"

type offlineImportFlags struct {
	stateRoot    string
	metadataDir  string
	rootPath     string
	targetPath   string
	artifactPath string
	environment  string
	network      string
	platform     string
	architecture string
	refTime      string
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
}
