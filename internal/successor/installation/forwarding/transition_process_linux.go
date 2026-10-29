package forwarding

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type transitionProcess struct {
	PID     uint32 `json:"pid"`
	Parent  uint32 `json:"parent"`
	Device  uint64 `json:"device"`
	Inode   uint64 `json:"inode"`
	Started uint64 `json:"started"`
}

func (s *transitionScope) retainProcess() (err error) {
	s.process, err = os.OpenFile("/proc/"+strconv.Itoa(s.pid), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	info, err := s.process.Stat()
	if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != 0 || native.Dev == 0 || native.Ino == 0 {
		return ErrBinding
	}
	s.processInfo = info
	fd, err := syscall.Openat(int(s.process.Fd()), "stat", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), "original Forwarding coordinator start")
	raw, readErr := readKernelFile(file, 4096)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	text := string(raw)
	end := strings.LastIndex(text, ") ")
	if end < 0 || !strings.HasPrefix(text, strconv.Itoa(s.pid)+" (") {
		return ErrBinding
	}
	fields := strings.Fields(text[end+2:])
	if len(fields) < 20 || fields[1] != strconv.Itoa(os.Getpid()) || fields[2] != strconv.Itoa(s.pid) {
		return ErrBinding
	}
	started, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || started == 0 || fields[19] != strconv.FormatUint(started, 10) {
		return errors.Join(ErrBinding, err)
	}
	s.birth = transitionProcess{PID: uint32(s.pid), Parent: uint32(os.Getpid()), Device: uint64(native.Dev), Inode: native.Ino, Started: started}
	return nil
}
