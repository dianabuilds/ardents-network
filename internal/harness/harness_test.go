package harness

import "testing"

func TestSmokeContainerOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		state     composeContainerState
		completed bool
		wantErr   bool
	}{
		{name: "running", state: composeContainerState{Running: true}, completed: false},
		{name: "successful", state: composeContainerState{ExitCode: 0}, completed: true},
		{name: "failed", state: composeContainerState{ExitCode: 23}, completed: true, wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			completed, err := smokeContainerOutcome(test.state)
			if completed != test.completed || (err != nil) != test.wantErr {
				t.Fatalf("outcome = (%v, %v), want completed=%v error=%v", completed, err, test.completed, test.wantErr)
			}
		})
	}
}

func TestSmokeNetworkContractRequiresTheExpectedInternalNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		networks []composeNetworkInspect
		want     bool
	}{
		{name: "expected internal", networks: []composeNetworkInspect{{Name: "project_adjacency", Internal: true}}, want: true},
		{name: "expected external", networks: []composeNetworkInspect{{Name: "project_adjacency", Internal: false}}},
		{name: "unexpected internal", networks: []composeNetworkInspect{{Name: "other", Internal: true}}},
		{name: "extra network", networks: []composeNetworkInspect{{Name: "project_adjacency", Internal: true}, {Name: "other", Internal: true}}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := smokeNetworkContract(test.networks, "project_adjacency"); got != test.want {
				t.Fatalf("network contract = %v, want %v", got, test.want)
			}
		})
	}
}

func TestOfficialRunnerClassification(t *testing.T) {
	t.Parallel()

	ubuntu2604 := []byte("ID=ubuntu\nVERSION_ID=\"26.04\"\n")
	tests := []struct {
		name          string
		goos          string
		goarch        string
		kernelRelease string
		osRelease     []byte
		want          bool
	}{
		{name: "native Ubuntu 26.04 x86-64", goos: "linux", goarch: "amd64", kernelRelease: "6.17.0-generic", osRelease: ubuntu2604, want: true},
		{name: "WSL", goos: "linux", goarch: "amd64", kernelRelease: "6.6.87-microsoft-standard-WSL2", osRelease: ubuntu2604},
		{name: "wrong release", goos: "linux", goarch: "amd64", kernelRelease: "6.17.0-generic", osRelease: []byte("ID=ubuntu\nVERSION_ID=\"24.04\"\n")},
		{name: "wrong architecture", goos: "linux", goarch: "arm64", kernelRelease: "6.17.0-generic", osRelease: ubuntu2604},
		{name: "Docker Desktop controller", goos: "windows", goarch: "amd64", osRelease: ubuntu2604},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := officialRunnerFor(test.goos, test.goarch, test.kernelRelease, test.osRelease); got != test.want {
				t.Fatalf("official runner = %v, want %v", got, test.want)
			}
		})
	}
}
