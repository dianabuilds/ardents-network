package infrastructure

import "github.com/dianabuilds/ardents-network/internal/successor/release"

// ProgramInputs projects public inventory bytes into the fixed infrastructure Release
// targets. It supplies neither Enrollment provenance nor Release authority.
func ProgramInputs(files interface{ File(string) ([]byte, bool) }) (release.Inputs, error) {
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
	return release.Inputs{RootBytes: root, Artifact: program, TargetPath: ProgramTarget, Files: metadata}, nil
}
