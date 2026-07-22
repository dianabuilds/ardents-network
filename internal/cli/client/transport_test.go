package client

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestSSHStreamLocalArgumentsUseProtectedSocketAndNeverLegacyW(t *testing.T) {
	cfg := Config{
		SSH: "ops@alpha.example", SSHPort: 2222,
		SSHIdentity: "identity-file", SSHKnownHosts: "known-hosts-file",
		SSHOperatorSocket: "/var/lib/ardents/secrets/control.sock",
	}
	arguments := sshStreamLocalArguments(cfg, "operator.sock")
	require.Equal(t, []string{
		"-N", "-T", "-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes", "-p", "2222",
		"-o", "IdentitiesOnly=yes", "-i", "identity-file",
		"-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=known-hosts-file",
		"-L", "operator.sock:/var/lib/ardents/secrets/control.sock", "ops@alpha.example",
	}, arguments)
	require.False(t, slices.Contains(arguments, "-W"))
}

func TestSSHStreamLocalTunnelReadinessAndCleanup(t *testing.T) {
	original := newSSHCommand
	t.Cleanup(func() { newSSHCommand = original })
	newSSHCommand = func([]string) *exec.Cmd {
		command := exec.Command(os.Args[0], "-test.run=TestSSHStreamLocalHelperProcess", "--")
		command.Env = append(os.Environ(), "ARDENTS_SSH_HELPER=ready")
		return command
	}
	tunnel := newSSHStreamLocalTransport(Config{SSH: "ops@alpha", SSHPort: 22, SSHOperatorSocket: "/run/ardents/operator.sock"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tunnel.ensureStarted(ctx))
	dir := tunnel.tempDir
	info, err := os.Lstat(tunnel.socket)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSocket)
	require.NoError(t, storage.ValidatePrivateDir(dir))
	require.NoError(t, tunnel.Close())
	_, err = os.Stat(dir)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestSSHStreamLocalStartFailureDoesNotMakeCloseHang(t *testing.T) {
	original := newSSHCommand
	t.Cleanup(func() { newSSHCommand = original })
	newSSHCommand = func([]string) *exec.Cmd { return exec.Command(filepath.Join(t.TempDir(), "missing-ssh")) }
	tunnel := newSSHStreamLocalTransport(Config{SSH: "ops@alpha", SSHPort: 22, SSHOperatorSocket: "/run/ardents/operator.sock"})
	err := tunnel.ensureStarted(context.Background())
	require.ErrorContains(t, err, "start SSH")
	done := make(chan error, 1)
	go func() { done <- tunnel.Close() }()
	select {
	case closeErr := <-done:
		require.NoError(t, closeErr)
	case <-time.After(time.Second):
		t.Fatal("Close hung after SSH Start failure")
	}
}

func TestSSHStreamLocalCloseBeforeDialPreventsLateStart(t *testing.T) {
	started := false
	original := newSSHCommand
	t.Cleanup(func() { newSSHCommand = original })
	newSSHCommand = func([]string) *exec.Cmd {
		started = true
		return exec.Command(os.Args[0], "-test.run=TestSSHStreamLocalHelperProcess", "--")
	}
	tunnel := newSSHStreamLocalTransport(Config{SSH: "ops@alpha", SSHPort: 22, SSHOperatorSocket: "/run/ardents/operator.sock"})
	require.NoError(t, tunnel.Close())
	err := tunnel.ensureStarted(context.Background())
	require.ErrorContains(t, err, "closed")
	require.False(t, started)
	require.Empty(t, tunnel.tempDir)
}

func TestSSHStreamLocalTunnelEarlyExitIsRedactedAndCleansUp(t *testing.T) {
	original := newSSHCommand
	t.Cleanup(func() { newSSHCommand = original })
	newSSHCommand = func([]string) *exec.Cmd {
		command := exec.Command(os.Args[0], "-test.run=TestSSHStreamLocalHelperProcess", "--")
		command.Env = append(os.Environ(), "ARDENTS_SSH_HELPER=exit")
		return command
	}
	tunnel := newSSHStreamLocalTransport(Config{SSH: "ops@alpha", SSHPort: 22, SSHOperatorSocket: "/secret/remote/operator.sock"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := tunnel.ensureStarted(ctx)
	require.ErrorContains(t, err, "exited before readiness")
	require.NotContains(t, err.Error(), "/secret/remote/operator.sock")
	require.NoError(t, tunnel.Close())
}

func TestSSHStreamLocalHelperProcess(t *testing.T) {
	switch os.Getenv("ARDENTS_SSH_HELPER") {
	case "":
		return
	case "exit":
		os.Exit(3)
	case "ready":
		listener, err := net.Listen("unix", "operator.sock")
		if err != nil {
			os.Exit(4)
		}
		defer listener.Close()
		select {}
	default:
		os.Exit(5)
	}
}

func TestControlTransportClassifiesPrincipalEligibleTargets(t *testing.T) {
	_, _, kind, closeTransport := controlTransport(Config{BaseURL: "unix:///run/ardents/operator.sock"})
	require.Equal(t, transportUnix, kind)
	require.NoError(t, closeTransport())

	_, _, kind, closeTransport = controlTransport(Config{BaseURL: "http://127.0.0.1:8080", SSH: "ops@alpha", SSHPort: 22, SSHOperatorSocket: "/run/ardents/operator.sock"})
	require.Equal(t, transportSSHStreamLocal, kind)
	require.NoError(t, closeTransport())

	_, _, kind, closeTransport = controlTransport(Config{BaseURL: "http://127.0.0.1:8080"})
	require.Equal(t, transportHTTP, kind)
	require.NoError(t, closeTransport())
}
