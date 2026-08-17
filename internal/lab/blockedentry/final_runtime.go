package blockedentry

import (
	"errors"
	"os"
	"path/filepath"
)

const finalRuntimeComposePath = "runtime/blocked-entry.compose.yaml"

func freezeRuntimeCompose(workspace, output string) (artifactCommitment, error) {
	source := filepath.Join(workspace, "tests", "live", "blocked-entry.compose.yaml")
	target := filepath.Join(output, filepath.FromSlash(finalRuntimeComposePath))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return artifactCommitment{}, err
	}
	if err := copyStableArtifact(source, target, 0o400); err != nil {
		return artifactCommitment{}, err
	}
	value, err := commitment(output, finalRuntimeComposePath)
	if err != nil || value.Bytes < 1 {
		return artifactCommitment{}, errors.Join(err, errors.New("runtime Compose commitment is unavailable"))
	}
	return value, nil
}

func freezeRuntimeComposeInput(config Config, secretRoot, sourceRoot string,
	value artifactCommitment,
) (Config, error) {
	if value.Path != finalRuntimeComposePath || !hexDigest(value.SHA256, 32) || value.Bytes < 1 {
		return Config{}, errors.New("runtime Compose commitment is invalid")
	}
	source := filepath.Join(sourceRoot, filepath.FromSlash(finalRuntimeComposePath))
	hash, size, err := hashFile(source)
	if err != nil || hash != value.SHA256 || size != value.Bytes {
		return Config{}, errors.Join(err, errors.New("runtime Compose artifact differs from its commitment"))
	}
	target := filepath.Join(secretRoot, filepath.FromSlash(finalRuntimeComposePath))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return Config{}, err
	}
	if err := copyStableArtifact(source, target, 0o400); err != nil {
		return Config{}, err
	}
	config.RuntimeComposePath = target
	return config, nil
}
