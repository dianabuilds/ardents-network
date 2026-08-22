package update

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

// stageFile is the narrow private candidate-file operation boundary used only
// to inject the S7.2-03 write/flush/close failures per Apply invocation.
type stageFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type stageOperations struct {
	openFile        func(string) (stageFile, error)
	renameDirectory func(string, string) error
	acknowledge     func(string) error
}

func nativeStageOperations(ops durabilityOps) stageOperations {
	return stageOperations{
		openFile: func(path string) (stageFile, error) {
			return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		},
		renameDirectory: ops.publishGeneration,
		// publishGeneration has already completed the platform's parent
		// durability acknowledgement. This no-op keeps the operation shape
		// explicit without repeating an already acknowledged native move.
		acknowledge: func(string) error { return nil },
	}
}

func writeStageFile(operations stageOperations, path string, data []byte) error {
	file, err := operations.openFile(path)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = os.ErrInvalid
	}
	return errors.Join(writeErr, file.Sync(), file.Close())
}

func stageTemporaryPath(root string, generation uint64) string {
	return filepath.Join(root, "staging", strconv.FormatUint(generation, 10)+".tmp")
}
