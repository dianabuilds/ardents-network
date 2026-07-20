package execution

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testDigestImage = "docker.io/library/busybox@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestContainerSpecAppliesSafeResourceDefaults(t *testing.T) {
	spec, err := parseContainerSpec(`{"image":"` + testDigestImage + `","user":"65534:65534","env":{"MODE":"safe"}}`)
	require.NoError(t, err)
	require.EqualValues(t, defaultContainerMemory, spec.Resources.MemoryBytes)
	require.EqualValues(t, defaultContainerCPU, spec.Resources.NanoCPUs)
	require.EqualValues(t, defaultContainerPIDs, spec.Resources.PIDs)
	require.EqualValues(t, defaultContainerTmpfs, spec.Resources.TmpfsBytes)
}

func TestDockerSafeErrorDoesNotLeakDaemonDetails(t *testing.T) {
	err := dockerSafeError("create workload container", errors.New("dial /secret/docker.sock token=hidden-value"))
	require.ErrorContains(t, err, "Docker runtime error")
	require.NotContains(t, err.Error(), "docker.sock")
	require.NotContains(t, err.Error(), "hidden-value")
}

func TestContainerSpecRejectsUnsafeOrAmbiguousInputWithoutEchoingSecrets(t *testing.T) {
	tests := []struct {
		name   string
		config string
		reason string
	}{
		{"unknown privileged field", `{"image":"` + testDigestImage + `","user":"65534","privileged":true}`, "unknown field"},
		{"duplicate field", `{"image":"` + testDigestImage + `","user":"65534","user":"1000"}`, "duplicate field"},
		{"root user", `{"image":"` + testDigestImage + `","user":"0"}`, "non-root"},
		{"relative working directory", `{"image":"` + testDigestImage + `","user":"65534","working_dir":"tmp"}`, "clean absolute"},
		{"resource over ceiling", `{"image":"` + testDigestImage + `","user":"65534","resources":{"pids":513}}`, "outside policy bounds"},
		{"secret key", `{"image":"` + testDigestImage + `","user":"65534","env":{"API_TOKEN":"do-not-echo"}}`, "cannot contain secret"},
		{"credential URL", `{"image":"` + testDigestImage + `","user":"65534","env":{"ENDPOINT":"https://user:do-not-echo@example.test"}}`, "cannot contain secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseContainerSpec(tt.config)
			require.ErrorContains(t, err, tt.reason)
			require.NotContains(t, err.Error(), "do-not-echo")
		})
	}
}

func TestContainerSpecEnforcesEnvironmentAndCommandBounds(t *testing.T) {
	oversized := strings.Repeat("x", maxEnvironmentValue+1)
	_, err := parseContainerSpec(`{"image":"` + testDigestImage + `","user":"65534","env":{"VALUE":"` + oversized + `"}}`)
	require.ErrorContains(t, err, "environment exceeds size limit")

	arguments := make([]string, maxCommandArguments+1)
	for index := range arguments {
		arguments[index] = `"x"`
	}
	_, err = parseContainerSpec(`{"image":"` + testDigestImage + `","user":"65534","command":[` + strings.Join(arguments, ",") + `]}`)
	require.ErrorContains(t, err, "command exceeds argument limit")
}

func TestRuntimeGenerationEnvironmentCannotBeWorkloadControlled(t *testing.T) {
	_, err := parseContainerSpec(`{"image":"` + testDigestImage + `","user":"65534","env":{"ARDENTS_WORKLOAD_GENERATION":"forged"}}`)
	require.ErrorContains(t, err, "reserved")

	env := runtimeEnvironment(map[string]string{"PUBLIC_MODE": "safe"}, 73)
	require.Contains(t, env, "PUBLIC_MODE=safe")
	require.Contains(t, env, "ARDENTS_WORKLOAD_GENERATION=73")
}

func TestDockerExecutorTrustAndProvenanceAdmissionFailClosed(t *testing.T) {
	executor, err := NewDockerExecutor(DockerExecutorConfig{
		NodeID: "node-security", AllowedRegistries: []string{"docker.io"}, AllowedPolicyRefs: []string{"trusted"},
	})
	require.NoError(t, err)
	runtime, trustClass := executor.executionClass("")
	require.Equal(t, "runsc", runtime)
	require.Equal(t, "untrusted", trustClass)
	runtime, trustClass = executor.executionClass("trusted")
	require.Equal(t, "runc", runtime)
	require.Equal(t, "trusted", trustClass)
	require.NoError(t, executor.admitImage(testDigestImage))
	require.ErrorContains(t, executor.admitImage("registry.invalid/app@sha256:"+strings.Repeat("a", 64)), "not allowed")
	require.NoError(t, executor.admitPolicyRef("trusted"))
	require.ErrorContains(t, executor.admitPolicyRef("other"), "not allowed")
}
