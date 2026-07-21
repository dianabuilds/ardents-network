package docker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerEndpointRequiresAuthenticatedRemoteTransport(t *testing.T) {
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	require.NoError(t, validateDockerEndpoint("unix:///var/run/docker.sock", false))
	require.NoError(t, validateDockerEndpoint("npipe:////./pipe/docker_engine", false))
	require.ErrorContains(t, validateDockerEndpoint("tcp://engine.example:2375", false), "requires TLS")
	require.NoError(t, validateDockerEndpoint("tcp://engine:2375", true))
	require.ErrorContains(t, validateDockerEndpoint("ssh://engine.example", false), "not supported")

	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", "/run/ardents/docker-certs")
	require.NoError(t, validateDockerEndpoint("tcp://engine.example:2376", false))
}
