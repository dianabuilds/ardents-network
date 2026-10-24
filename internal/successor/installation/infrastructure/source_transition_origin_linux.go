package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"syscall"
)

// Detached historical facts from this actual original launcher. They are not
// Release, State, startup or manager authority. The caller retains its actual
// program and original process scope until their required physical Close.
type sourceTransitionOrigin struct {
	Process       sourceTransitionProcess `json:"process"`
	ProgramDevice uint64                  `json:"program_device"`
	ProgramInode  uint64                  `json:"program_inode"`
	ProgramSHA256 string                  `json:"program_sha256"`
	NativeManager string                  `json:"native_manager"`
}

func captureSourceTransitionOrigin(program *os.File, scope *sourceTransitionScope) (origin sourceTransitionOrigin, err error) {
	if program == nil || scope == nil || scope.birth.PID == 0 || scope.birth.Started == 0 {
		return origin, ErrInput
	}
	info, err := program.Stat()
	if err != nil || !sourceAccess(info, info.Mode().Perm(), 0, 0, false) || info.Mode().Perm()&0022 != 0 || info.Size() <= 0 || info.Size() > 256<<20 {
		return origin, errors.Join(ErrBinding, err)
	}
	if _, err = program.Seek(0, io.SeekStart); err != nil {
		return origin, err
	}
	hash := sha256.New()
	buffer := make([]byte, 32<<10)
	n, err := io.CopyBuffer(hash, io.LimitReader(program, info.Size()+1), buffer)
	clear(buffer)
	if err != nil || n != info.Size() {
		return origin, errors.Join(ErrBinding, err)
	}
	after, err := program.Stat()
	if err != nil || !sameSourceFile(info, after) {
		return origin, errors.Join(ErrBinding, err)
	}
	native := info.Sys().(*syscall.Stat_t)
	origin = sourceTransitionOrigin{Process: scope.birth, ProgramDevice: uint64(native.Dev), ProgramInode: native.Ino, ProgramSHA256: hex.EncodeToString(hash.Sum(nil))}
	origin.NativeManager, err = sourceManagerIdentity()
	return origin, err
}
