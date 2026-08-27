//go:build ignore

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	lifecycleSchema = "ardents-r095-fixture-state-v1"
	readySchema     = "ardents-r095-fixture-ready-v1"
	fixtureFloor    = uint64(7)
)

var buildID = "unset"

type lifecycleState struct {
	Schema   string `json:"schema"`
	Identity string `json:"identity"`
	Starts   uint64 `json:"starts"`
	Floor    uint64 `json:"floor"`
}

type readyResult struct {
	Schema   string `json:"schema"`
	Build    string `json:"build"`
	Identity string `json:"identity"`
	Starts   uint64 `json:"starts"`
	Floor    uint64 `json:"floor"`
}

func main() {
	mode := flag.String("mode", "serve", "serve or probe")
	stateRoot := flag.String("state-root", "", "absolute protected state root")
	runtimeRoot := flag.String("runtime-root", "", "absolute runtime root")
	flag.Parse()
	var err error
	switch *mode {
	case "serve":
		err = serveLifecycle(*stateRoot, *runtimeRoot)
	case "probe":
		err = probeLifecycle(*runtimeRoot)
	default:
		err = errors.New("unsupported lifecycle fixture mode")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveLifecycle(stateRoot, runtimeRoot string) error {
	if buildID != "v1" && buildID != "v2" {
		return errors.New("fixture-build-id")
	}
	if err := validatePrivateRoot("state", stateRoot); err != nil {
		return err
	}
	if err := validatePrivateRoot("runtime", runtimeRoot); err != nil {
		return err
	}
	lock, err := acquireLifecycleLock(filepath.Join(stateRoot, "owner.lock"))
	if err != nil {
		return err
	}
	defer releaseLifecycleLock(lock)

	state, err := advanceLifecycleState(stateRoot)
	if err != nil {
		return err
	}
	result := readyResult{Schema: readySchema, Build: buildID, Identity: state.Identity,
		Starts: state.Starts, Floor: state.Floor}
	socketPath := filepath.Join(runtimeRoot, "endpoint.sock")
	readyPath := filepath.Join(runtimeRoot, "ready.json")
	if err := removeExpectedRuntime(socketPath, os.ModeSocket); err != nil {
		return err
	}
	if err := removeExpectedRuntime(readyPath, 0); err != nil {
		return err
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("attachment-bind-failed: %w", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(readyPath)
		_ = os.Remove(socketPath)
	}()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return err
	}
	if err := writeJSONAtomic(readyPath, result); err != nil {
		return err
	}

	serveErrors := make(chan error, 1)
	serveDone := make(chan struct{})
	go acceptLifecycle(listener, result, serveErrors, serveDone)
	defer func() {
		_ = listener.Close()
		<-serveDone
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case <-signals:
		return nil
	case err := <-serveErrors:
		return err
	}
}

func probeLifecycle(runtimeRoot string) error {
	if err := validatePrivateRoot("runtime", runtimeRoot); err != nil {
		return err
	}
	connection, err := net.DialTimeout("unix", filepath.Join(runtimeRoot, "endpoint.sock"), 2*time.Second)
	if err != nil {
		return fmt.Errorf("readiness-connect-failed: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := connection.Write([]byte("ready?\n")); err != nil {
		return err
	}
	line, err := bufio.NewReader(connection).ReadBytes('\n')
	if err != nil {
		return err
	}
	var result readyResult
	if err := json.Unmarshal(line, &result); err != nil {
		return err
	}
	if result.Schema != readySchema || (result.Build != "v1" && result.Build != "v2") ||
		result.Identity == "" || result.Starts == 0 || result.Floor != fixtureFloor {
		return errors.New("invalid-readiness-proof")
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func acceptLifecycle(listener net.Listener, result readyResult, failures chan<- error, done chan<- struct{}) {
	defer close(done)
	for {
		connection, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			select {
			case failures <- err:
			default:
			}
			return
		}
		_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
		line, readErr := bufio.NewReader(connection).ReadString('\n')
		if readErr == nil && line == "ready?\n" {
			readErr = json.NewEncoder(connection).Encode(result)
		}
		_ = connection.Close()
		if readErr != nil {
			select {
			case failures <- readErr:
			default:
			}
			return
		}
	}
}

func validatePrivateRoot(class, root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("%s-root-invalid", class)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("%s-root-invalid: %w", class, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s-root-type", class)
	}
	if info.Mode().Perm() != 0o700 {
		return fmt.Errorf("%s-root-permissions", class)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("%s-root-owner", class)
	}
	return nil
}

func acquireLifecycleLock(path string) (*os.File, error) {
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = lock.Close()
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, errors.New("owner-busy")
	}
	return lock, nil
}

func releaseLifecycleLock(lock *os.File) {
	_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	_ = lock.Close()
}

func advanceLifecycleState(root string) (lifecycleState, error) {
	path := filepath.Join(root, "fixture-state.json")
	state := lifecycleState{Schema: lifecycleSchema, Floor: fixtureFloor}
	data, err := os.ReadFile(path)
	if err == nil {
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			return lifecycleState{}, errors.New("state-file-invalid")
		}
		if err := json.Unmarshal(data, &state); err != nil {
			return lifecycleState{}, errors.New("state-file-invalid")
		}
		if state.Schema != lifecycleSchema || state.Identity == "" || state.Floor != fixtureFloor {
			return lifecycleState{}, errors.New("state-file-invalid")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		identity := make([]byte, 16)
		if _, err := rand.Read(identity); err != nil {
			return lifecycleState{}, err
		}
		state.Identity = hex.EncodeToString(identity)
	} else {
		return lifecycleState{}, err
	}
	state.Starts++
	if err := writeJSONAtomic(path, state); err != nil {
		return lifecycleState{}, err
	}
	return state, nil
}

func writeJSONAtomic(path string, value any) error {
	temporary := path + ".next"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	encoderErr := json.NewEncoder(file).Encode(value)
	syncErr := file.Sync()
	closeErr := file.Close()
	if encoderErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(temporary)
		return errors.Join(encoderErr, syncErr, closeErr)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	err = directory.Sync()
	return errors.Join(err, directory.Close())
}

func removeExpectedRuntime(path string, expected os.FileMode) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if expected == os.ModeSocket {
		if info.Mode()&os.ModeSocket == 0 {
			return errors.New("unexpected-runtime-entry")
		}
	} else if !info.Mode().IsRegular() {
		return errors.New("unexpected-runtime-entry")
	}
	return os.Remove(path)
}
