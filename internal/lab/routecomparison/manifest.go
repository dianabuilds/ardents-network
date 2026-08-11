package routecomparison

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/lab/sourceidentity"
)

type inputManifest struct {
	SchemaVersion       string `json:"schema_version"`
	RunID               string `json:"run_id"`
	CreatedUTC          string `json:"created_utc"`
	Classification      string `json:"classification"`
	SourceSHA256        string `json:"source_sha256"`
	ApplicationImage    string `json:"application_image"`
	ToolImage           string `json:"tool_image"`
	ReferenceLockSHA256 string `json:"reference_lock_sha256"`
	referenceDirectory  string
	Host                hostManifest                  `json:"host"`
	Network             networkManifest               `json:"network"`
	Workloads           map[string][]manifestWorkload `json:"workloads"`
}

type hostManifest struct {
	GOOS           string `json:"goos"`
	GOARCH         string `json:"goarch"`
	OSRelease      string `json:"os_release"`
	Kernel         string `json:"kernel"`
	DockerVersion  string `json:"docker_version"`
	ComposeVersion string `json:"compose_version"`
}

type networkManifest struct {
	RateMbit         int `json:"rate_mbit"`
	EndpointDelayMS  int `json:"endpoint_egress_delay_ms"`
	QdiscPacketLimit int `json:"qdisc_packet_limit"`
	LossPercent      int `json:"loss_percent"`
}

type manifestWorkload struct {
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Direction       string `json:"direction,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
	Seed            string `json:"seed"`
}

func prepareManifest(ctx context.Context, runID, repositoryRoot, applicationImage, toolImage, referenceDirectory string) (inputManifest, error) {
	if !validImageID(applicationImage) || !validImageID(toolImage) {
		return inputManifest{}, errors.New("route experiment requires immutable sha256 image IDs")
	}
	classification, osRelease, kernel := runnerClassification()
	if classification != "official" {
		return inputManifest{}, errors.New("R-013 verdict requires native Ubuntu 26.04 linux/amd64")
	}
	referenceDigest, err := verifyReferenceDirectory(repositoryRoot, referenceDirectory)
	if err != nil {
		return inputManifest{}, err
	}
	sourceDigest, err := sourceidentity.SourceSHA256(repositoryRoot)
	if err != nil {
		return inputManifest{}, err
	}
	dockerVersion, err := commandVersion(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return inputManifest{}, err
	}
	composeVersion, err := commandVersion(ctx, "docker", "compose", "version", "--short")
	if err != nil {
		return inputManifest{}, err
	}
	master := make([]byte, 32)
	if _, err := rand.Read(master); err != nil {
		return inputManifest{}, err
	}
	return inputManifest{
		SchemaVersion: experimentSchema, RunID: runID, CreatedUTC: time.Now().UTC().Format(time.RFC3339Nano),
		Classification: classification, SourceSHA256: sourceDigest, ApplicationImage: applicationImage, ToolImage: toolImage,
		ReferenceLockSHA256: referenceDigest, referenceDirectory: referenceDirectory,
		Host:      hostManifest{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, OSRelease: osRelease, Kernel: kernel, DockerVersion: dockerVersion, ComposeVersion: composeVersion},
		Network:   networkManifest{RateMbit: 100, EndpointDelayMS: 40, QdiscPacketLimit: 1000, LossPercent: 0},
		Workloads: fixedWorkloadSchedule(master),
	}, nil
}

func fixedWorkloadSchedule(master []byte) map[string][]manifestWorkload {
	result := make(map[string][]manifestWorkload, 3)
	for _, condition := range []string{"direct", "c3", "c5-c2"} {
		workloads := make([]manifestWorkload, 0, 26)
		for index := 1; index <= 20; index++ {
			name := fmt.Sprintf("setup-%02d", index)
			workloads = append(workloads, manifestWorkload{Name: name, Kind: "setup", Seed: deriveSeed(master, condition, name)})
		}
		for _, direction := range []string{directionUpload, directionDownload} {
			for index := 1; index <= 3; index++ {
				name := fmt.Sprintf("%s-%02d", direction, index)
				workloads = append(workloads, manifestWorkload{Name: name, Kind: "stream", Direction: direction, DurationSeconds: 60, Seed: deriveSeed(master, condition, name)})
			}
		}
		result[condition] = workloads
	}
	return result
}

func deriveSeed(master []byte, condition, name string) string {
	digest := sha256.New()
	_, _ = digest.Write(master)
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(condition))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(name))
	return hex.EncodeToString(digest.Sum(nil))
}

func runnerClassification() (string, string, string) {
	osRelease, _ := os.ReadFile("/etc/os-release")
	kernel, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	classification := "development"
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" && strings.Contains(string(osRelease), `VERSION_ID="26.04"`) && !strings.Contains(strings.ToLower(string(kernel)), "microsoft") {
		classification = "official"
	}
	return classification, strings.TrimSpace(string(osRelease)), strings.TrimSpace(string(kernel))
}

func validImageID(value string) bool {
	algorithm, digest, found := strings.Cut(value, ":")
	decoded, err := hex.DecodeString(digest)
	return found && algorithm == "sha256" && err == nil && len(decoded) == sha256.Size
}

func commandVersion(ctx context.Context, name string, arguments ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, arguments...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("observe %s version: %w", name, err)
	}
	value := strings.TrimSpace(string(output))
	if value == "" || strings.ContainsAny(value, "\r\n\t") {
		return "", errors.New("observed tool version is empty or multiline")
	}
	return value, nil
}

func canonicalDirectory(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("reference directory must be absolute and canonical")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("reference directory is not a real directory")
	}
	return path, nil
}
