package publisher

import (
	"errors"
	"io"
	"os"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"
)

var errSnapshotStage = errors.New("private snapshot stage is unavailable")

// snapshotStage owns the only writable descriptor. Its caller owns the
// previously admitted directory and the storage reservation.
type snapshotStage struct {
	writer    *os.File
	remaining int64
	failed    bool
}

func openSnapshotStage(directory *os.File, size int64) (*snapshotStage, error) {
	if directory == nil || size < 0 {
		return nil, errSnapshotStage
	}
	// O_EXCL disallows publishing a directory entry for this anonymous inode.
	flags := unix.O_TMPFILE | unix.O_EXCL | unix.O_RDWR | unix.O_CLOEXEC
	fd, err := unix.Openat(int(directory.Fd()), ".", flags, 0o600)
	runtime.KeepAlive(directory)
	if err != nil {
		return nil, err
	}
	writer := os.NewFile(uintptr(fd), "private snapshot stage")
	if size > 0 {
		if err := unix.Fallocate(fd, 0, 0, size); err != nil {
			return nil, errors.Join(err, writer.Close())
		}
	}
	return &snapshotStage{writer: writer, remaining: size}, nil
}

func (stage *snapshotStage) Write(block []byte) (int, error) {
	if stage.writer == nil || stage.failed || int64(len(block)) > stage.remaining {
		stage.failed = true
		return 0, errSnapshotStage
	}
	n, err := stage.writer.Write(block)
	stage.remaining -= int64(n)
	stage.failed = err != nil || n != len(block)
	if err == nil && stage.failed {
		err = io.ErrShortWrite
	}
	return n, err
}

// finish hands off the same inode through a read-only descriptor. The stage
// continues to own the writer on failure, so its caller must always close it.
func (stage *snapshotStage) finish() (*os.File, error) {
	if stage.writer == nil || stage.failed || stage.remaining != 0 {
		return nil, errSnapshotStage
	}
	writer := stage.writer
	original, err := writer.Stat()
	if err != nil {
		return nil, err
	}
	path := "/proc/self/fd/" + strconv.FormatUint(uint64(writer.Fd()), 10)
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	reader := os.NewFile(uintptr(fd), "private snapshot source")
	observed, err := reader.Stat()
	if err != nil || !observed.Mode().IsRegular() || !os.SameFile(original, observed) {
		return nil, errors.Join(errSnapshotStage, err, reader.Close())
	}
	if err := stage.Close(); err != nil {
		return nil, errors.Join(err, reader.Close())
	}
	return reader, nil
}

func (stage *snapshotStage) Close() error {
	if stage.writer == nil {
		return nil
	}
	writer := stage.writer
	stage.writer = nil
	return writer.Close()
}
