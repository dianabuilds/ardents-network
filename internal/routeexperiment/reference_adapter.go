package routeexperiment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

type referenceResult struct {
	Status              string `json:"status"`
	TorVersion          string `json:"tor_version"`
	PythonVersion       string `json:"python_version"`
	ChutneyRevision     string `json:"chutney_revision"`
	NetworkFlavor       string `json:"network_flavor"`
	OfflineNamespace    bool   `json:"offline_network_namespace"`
	ElapsedMilliseconds int64  `json:"elapsed_milliseconds"`
	OutputSHA256        string `json:"output_sha256,omitempty"`
	Failure             string `json:"failure,omitempty"`
}

func runTorReference(ctx context.Context, identity preflight.RunLayout, manifest inputManifest) (referenceResult, error) {
	_, _, runDirectory, evidenceDirectory, err := identity.OwnedPaths(true, true)
	if err != nil {
		return referenceResult{}, err
	}
	chutney, torRoot, pythonPackages, err := prepareReferenceRuntime(ctx, manifest.referenceDirectory, runDirectory)
	if err != nil {
		return referenceResult{}, err
	}
	tor := filepath.Join(torRoot, "usr", "bin", "tor")
	gencert := filepath.Join(torRoot, "usr", "bin", "tor-gencert")
	libraryPath := filepath.Join(torRoot, "lib", "x86_64-linux-gnu") + ":" + filepath.Join(torRoot, "usr", "lib", "x86_64-linux-gnu")
	referenceEnvironment := []string{
		"LD_LIBRARY_PATH=" + libraryPath,
		"PYTHONPATH=" + pythonPackages + ":" + filepath.Join(chutney, "lib"),
		"CHUTNEY_PATH=" + chutney, "CHUTNEY_TOR=" + tor, "CHUTNEY_TOR_GENCERT=" + gencert,
		"CHUTNEY_DATA_DIR=" + filepath.Join(runDirectory, "tor-network"), "CHUTNEY_DNS_CONF=/dev/null",
	}
	environment := append(os.Environ(), referenceEnvironment...)
	torVersion, err := observedReferenceVersion(ctx, environment, tor, "--version")
	if err != nil {
		return referenceResult{}, err
	}
	pythonVersion, err := observedReferenceVersion(ctx, environment, "python3", "--version")
	if err != nil {
		return referenceResult{}, err
	}
	result := referenceResult{
		Status: "failed", TorVersion: torVersion, PythonVersion: pythonVersion,
		ChutneyRevision: "988fc372cc418fbecc60558fe27e75d07d76b996",
		NetworkFlavor:   "bridges+hs-v3",
	}
	started := time.Now()
	referenceContext, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	referenceArguments := []string{
		filepath.Join(chutney, "tools", "test-network.sh"), "--chutney-path", chutney,
		"--tor", tor, "--tor-gencert", gencert, "--flavor", result.NetworkFlavor,
		"--offline", "--data", "10485760", "--start-time", "180", "--bootstrap-time", "90", "--stop-time", "0",
	}
	command := isolatedReferenceCommand(referenceContext, referenceEnvironment, referenceArguments)
	command.Env = environment
	output, runErr := command.CombinedOutput()
	result.ElapsedMilliseconds = time.Since(started).Milliseconds()
	if len(output) > 8*1024*1024 {
		output = output[len(output)-8*1024*1024:]
		runErr = errors.Join(runErr, errors.New("tor reference output exceeded 8 MiB"))
	}
	logPath := filepath.Join(evidenceDirectory, "tor-reference.log")
	if writeErr := os.WriteFile(logPath, output, 0o600); writeErr != nil {
		return result, errors.Join(runErr, writeErr)
	}
	result.OutputSHA256, _ = hashFile(logPath)
	result.OfflineNamespace = runErr == nil
	if runErr != nil {
		result.Failure = runErr.Error()
		return result, fmt.Errorf("tor/Chutney reference failed: %w", runErr)
	}
	result.Status = "passed"
	return result, nil
}

func isolatedReferenceCommand(ctx context.Context, environment, referenceArguments []string) *exec.Cmd {
	arguments := []string{"--non-interactive", "unshare", "--net", "env"}
	arguments = append(arguments, environment...)
	arguments = append(arguments,
		"bash", "-c",
		`uid=$1; gid=$2; shift 2; ip link set lo up; exec setpriv --reuid "$uid" --regid "$gid" --clear-groups "$@"`,
		"carrier-lab-reference", strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid()),
	)
	arguments = append(arguments, referenceArguments...)
	return exec.CommandContext(ctx, "sudo", arguments...)
}

func observedReferenceVersion(ctx context.Context, environment []string, name string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	first, _, _ := strings.Cut(value, "\n")
	first = strings.TrimSpace(first)
	if first == "" {
		return "", errors.New("reference version output is empty")
	}
	return first, nil
}

func prepareReferenceRuntime(ctx context.Context, inputsRoot, runDirectory string) (string, string, string, error) {
	inputs, err := readReferenceLock(filepath.Join(inputsRoot, "reference.lock"))
	if err != nil {
		return "", "", "", err
	}
	runtimeRoot := filepath.Join(runDirectory, "reference-runtime")
	chutney := filepath.Join(runtimeRoot, "chutney")
	torRoot := filepath.Join(runtimeRoot, "tor-root")
	pythonPackages := filepath.Join(runtimeRoot, "python-packages")
	for _, directory := range []string{chutney, torRoot, pythonPackages} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return "", "", "", err
		}
	}
	if err := runReferencePreparation(ctx, "tar", "-xzf", filepath.Join(inputsRoot, inputs.Archive), "--strip-components=1", "-C", chutney); err != nil {
		return "", "", "", err
	}
	packages := make([]string, 0, len(inputs.Packages))
	for name := range inputs.Packages {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	for _, name := range packages {
		if err := runReferencePreparation(ctx, "dpkg-deb", "--extract", filepath.Join(inputsRoot, "packages", name), torRoot); err != nil {
			return "", "", "", err
		}
	}
	wheels := make([]string, 0, len(inputs.Wheels))
	for name := range inputs.Wheels {
		wheels = append(wheels, name)
	}
	sort.Strings(wheels)
	for _, name := range wheels {
		if err := runReferencePreparation(ctx, "python3", "-m", "zipfile", "-e", filepath.Join(inputsRoot, "wheelhouse", name), pythonPackages); err != nil {
			return "", "", "", err
		}
	}
	return chutney, torRoot, pythonPackages, nil
}

func runReferencePreparation(ctx context.Context, name string, arguments ...string) error {
	output, err := exec.CommandContext(ctx, name, arguments...).CombinedOutput()
	if err != nil {
		if len(output) > 4*1024 {
			output = output[len(output)-4*1024:]
		}
		return fmt.Errorf("prepare reference with %s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}
