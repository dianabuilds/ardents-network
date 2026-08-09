package directcontrol

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func finishDirectControl(layout controlLayout, summary *directControlSummary, started time.Time, runErr error) error {
	summary.ElapsedMilliseconds = time.Since(started).Milliseconds()
	if runErr != nil {
		summary.Status = "failed"
		summary.Failure = runErr.Error()
	}
	cleanupErr := removeControlRunDirectory(layout)
	if cleanupErr == nil {
		summary.Checks["cleanup_complete"] = true
	} else {
		summary.Status = "failed"
		summary.Failure = errors.Join(runErr, cleanupErr).Error()
	}
	evidenceErr := writeDirectJSON(filepath.Join(layout.evidenceDir, "direct-control.json"), summary)
	return errors.Join(runErr, cleanupErr, evidenceErr)
}

func requireDirectBinary(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("binary path must be absolute and clean")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("binary path is not a regular file")
	}
	return nil
}

func hashDirectBinary(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func reserveControlAddress() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	address := listener.Addr().String()
	return address, listener.Close()
}

func waitDirectReady(ctx context.Context, path string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() <= directEvidenceCap {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return errors.New("direct TLS child did not become ready")
}

func readDirectEvidence(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) > directEvidenceCap {
		return errors.New("direct TLS role evidence exceeds its cap")
	}
	return json.Unmarshal(data, target)
}

type directChild struct {
	command  *exec.Cmd
	output   bytes.Buffer
	startErr error
	waited   bool
	exitCode int
}

func startDirectChild(ctx context.Context, binaryPath string, arguments ...string) directChild {
	child := directChild{command: exec.CommandContext(ctx, binaryPath, arguments...)}
	child.command.Stdout = &child.output
	child.command.Stderr = &child.output
	child.startErr = child.command.Start()
	return child
}

func (child *directChild) wait() int {
	if child == nil || child.startErr != nil {
		return -1
	}
	if child.waited {
		return child.exitCode
	}
	err := child.command.Wait()
	child.waited = true
	child.exitCode = 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			child.exitCode = exitError.ExitCode()
		} else {
			child.exitCode = -1
		}
	}
	return child.exitCode
}

func (child *directChild) stop() {
	if child == nil || child.startErr != nil || child.waited {
		return
	}
	_ = child.command.Process.Kill()
	child.wait()
}
