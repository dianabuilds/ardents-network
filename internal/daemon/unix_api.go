package daemon

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

func newUnixHTTPServer(path string, handler http.Handler) (*http.Server, net.Listener, error) {
	return newUnixHTTPServerWithPermissions(path, handler, ensurePrivateSocketDir, 0o600)
}

func newApplicationUnixHTTPServer(path string, handler http.Handler) (*http.Server, net.Listener, error) {
	return newUnixHTTPServerWithPermissions(path, handler, ensureApplicationSocketDir, 0o660)
}

func newUnixHTTPServerWithPermissions(
	path string,
	handler http.Handler,
	validateDirectory func(string) error,
	mode os.FileMode,
) (*http.Server, net.Listener, error) {
	if runtime.GOOS == "windows" {
		return nil, nil, fmt.Errorf("unix sockets are not supported on windows")
	}
	path = filepath.Clean(path)
	if err := validateDirectory(filepath.Dir(path)); err != nil {
		return nil, nil, err
	}
	if err := removeStaleUnixSocket(path); err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on local API socket: %w", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		_ = listener.Close()
		return nil, nil, fmt.Errorf("protect local API socket: %w", err)
	}
	return newHTTPServer("unix://"+path, handler), &removingListener{Listener: listener, path: path}, nil
}

func ensureApplicationSocketDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect Application Interface socket directory: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o027 != 0 {
		return fmt.Errorf("Application Interface socket directory must deny group writes and other access")
	}
	return nil
}

func ensurePrivateSocketDir(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create local API socket directory: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect local API socket directory: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("local API socket directory must be private")
	}
	return nil
}

func removeStaleUnixSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect local API socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("local API socket path exists and is not a socket")
	}
	connection, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
	if dialErr == nil {
		_ = connection.Close()
		return fmt.Errorf("local API socket is already active")
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) && !errors.Is(dialErr, syscall.ENOENT) {
		return fmt.Errorf("cannot verify existing local API socket: %w", dialErr)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale local API socket: %w", err)
	}
	return nil
}

type removingListener struct {
	net.Listener
	path string
}

func (l *removingListener) Close() error {
	listenErr := l.Listener.Close()
	removeErr := os.Remove(l.path)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(listenErr, removeErr)
}
