package infrastructure

import "github.com/dianabuilds/ardents-network/internal/successor/release"

// SourceInputs projects public inventory bytes into the fixed Source Release
// targets. It supplies neither Enrollment provenance nor Release authority.
func SourceInputs(files interface{ File(string) ([]byte, bool) }, local release.LocalEnvironment) (release.Inputs, error) {
	if files == nil {
		return release.Inputs{}, ErrInput
	}
	root, _ := files.File("root.json")
	timestamp, _ := files.File("timestamp.json")
	snapshot, _ := files.File("snapshot.json")
	targets, _ := files.File("targets.json")
	program, _ := files.File(ProgramName)
	metadata, err := release.StaticMetadataFiles(root, timestamp, snapshot, targets)
	if err != nil {
		return release.Inputs{}, err
	}
	return release.Inputs{RootBytes: root, Artifact: program, TargetPath: ProgramTarget, Local: local, Files: metadata}, nil
}
