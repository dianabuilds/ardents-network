package blockedentry

import (
	"os"
	"path/filepath"
)

func secretArtifacts(secretRoot string, config Config) ([]artifactCommitment, error) {
	generated := []string{"candidate/client.stderr", "candidate/server.stderr", "capture/packets.bin"}
	if config.CampaignSpecPath == "" {
		for _, path := range generated {
			absolute := filepath.Join(secretRoot, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(absolute, []byte("secret-only fixture\n"), 0o600); err != nil {
				return nil, err
			}
		}
	}
	paths := []string{"canaries.json",
		filepath.ToSlash(filepath.Join("supply", filepath.Base(config.RunnerPath))),
		filepath.ToSlash(filepath.Join("supply", filepath.Base(config.ClientPath))),
		filepath.ToSlash(filepath.Join("supply", filepath.Base(config.ServerPath)))}
	if config.CampaignSpecPath == "" {
		paths = append(paths, generated...)
	} else {
		paths = append(paths, "final-spec.json")
		paths = append(paths, finalConfigurationPaths...)
	}
	artifacts := make([]artifactCommitment, 0, len(paths))
	for _, path := range paths {
		value, err := commitment(secretRoot, path)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, value)
	}
	return artifacts, nil
}
