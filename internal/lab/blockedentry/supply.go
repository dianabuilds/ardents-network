package blockedentry

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func freezeSupply(config Config, secretRoot string) (Config, error) {
	directory := filepath.Join(secretRoot, "supply")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return Config{}, err
	}
	inputs := []struct {
		name   string
		source string
		assign func(string)
	}{
		{"runner", config.RunnerPath, func(path string) { config.RunnerPath = path }},
		{"client", config.ClientPath, func(path string) { config.ClientPath = path }},
		{"server", config.ServerPath, func(path string) { config.ServerPath = path }},
	}
	for _, input := range inputs {
		target := filepath.Join(directory, input.name+filepath.Ext(input.source))
		if err := copyStableExecutable(input.source, target); err != nil {
			return Config{}, err
		}
		input.assign(target)
	}
	return config, nil
}

func copyStableExecutable(source, target string) error {
	pathInfo, err := os.Lstat(source)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 {
		return errors.Join(err, errors.New("supply source is invalid"))
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) ||
		info.Size() < 1 || info.Size() > maximumEvidenceFile {
		return errors.Join(err, errors.New("supply source is not a bounded stable file"))
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, maximumEvidenceFile+1))
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written != info.Size() {
		return errors.Join(copyErr, syncErr, closeErr, errors.New("supply copy is incomplete"))
	}
	return os.Chmod(target, 0o500)
}
