package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

type nodeEvidenceFile struct {
	name string
	raw  []byte
}

func (observer *nodeObserver) capturePreflightEvidence(ctx context.Context) error {
	files, err := readNodeFixtureEvidence(observer.input.FixtureRoot)
	if err != nil {
		return err
	}
	manifest := files[0].raw
	config, err := observer.composeBounded(ctx, 2<<20, "config")
	if err != nil {
		return err
	}
	version, err := observer.dockerBounded(ctx, 256<<10, 32<<10, "version", "--format", "{{json .}}")
	if err != nil {
		return err
	}
	info, err := observer.dockerBounded(ctx, 512<<10, 32<<10, "info", "--format", "{{json .}}")
	if err != nil {
		return err
	}
	files = append(files, nodeEvidenceFile{"compose-resolved.yaml", config},
		nodeEvidenceFile{"docker-version.json", version}, nodeEvidenceFile{"docker-info.json", info})
	if err := observer.writeEvidenceFiles(files); err != nil {
		return err
	}
	observer.composeFile = filepath.Join(observer.input.EvidenceRoot, "compose-resolved.yaml")
	return observer.freezeCampaignManifest(manifest, config)
}

func readNodeFixtureEvidence(root string) ([]nodeEvidenceFile, error) {
	manifest, err := byteio.ReadFile(filepath.Join(root, "manifest.json"), 64<<10)
	if err != nil {
		return nil, fmt.Errorf("read node fixture manifest evidence: %w", err)
	}
	if len(manifest) == 0 {
		return nil, errors.New("node fixture manifest evidence is empty")
	}
	seal, err := byteio.ReadFile(filepath.Join(root, ".ardents-node-manifest.sha256"), 128)
	if err != nil {
		return nil, fmt.Errorf("read node fixture manifest seal evidence: %w", err)
	}
	if len(strings.TrimSpace(string(seal))) != 64 {
		return nil, errors.New("node fixture manifest seal evidence is invalid")
	}
	return []nodeEvidenceFile{{"manifest.json", manifest}, {"manifest.sha256", seal}}, nil
}

func (observer *nodeObserver) captureCandidateIdentity(ctx context.Context) error {
	if err := validateNodeSourceIdentity(observer.input.EvidenceRoot, observer.sourceRoot, observer.sourceDigest); err != nil {
		return err
	}
	image, err := observer.dockerBounded(ctx, 2<<20, 32<<10, "image", "inspect", "ardents-node:"+observer.imageTag)
	if err != nil {
		return err
	}
	label, err := observer.dockerBounded(ctx, 128, 4096, "image", "inspect", "--format",
		"{{index .Config.Labels \"org.opencontainers.image.revision\"}}", "ardents-node:"+observer.imageTag)
	if err != nil {
		return err
	}
	if string(bytesTrimSpace(label)) != observer.sourceDigest {
		return errors.New("node image is not bound to the captured source digest")
	}
	id, err := observer.serviceID(ctx, "source1")
	if err != nil {
		return err
	}
	build, err := observer.dockerBounded(ctx, 1<<20, 32<<10, "exec", id, "/usr/local/bin/ardents-qualify", "build-info-node")
	if err != nil {
		return err
	}
	identities, err := observer.composeBounded(ctx, 4096, "ps", "-q")
	if err != nil {
		return err
	}
	if len(strings.Fields(string(identities))) != 5 {
		return errors.New("node topology identity is incomplete")
	}
	arguments := append([]string{"inspect"}, strings.Fields(string(identities))...)
	topology, err := observer.dockerBounded(ctx, 4<<20, 32<<10, arguments...)
	if err != nil {
		return err
	}
	return observer.writeEvidenceFiles([]nodeEvidenceFile{
		{"image-inspect.json", image}, {"build-identity.json", build}, {"topology-inspect.json", topology},
	})
}

func (observer *nodeObserver) startCollector(ctx context.Context) error {
	arguments := []string{"run", "-d", "--pull", "never", "--name", observer.project + "-collector", "--pid", "host",
		"--network", "none", "--read-only", "--user", "0:0", "--cap-drop", "ALL", "--cap-add", "SYS_PTRACE",
		"--cap-add", "DAC_READ_SEARCH", "--security-opt", "no-new-privileges:true", "--pids-limit", "32",
		"--memory", "128m", "--memory-swap", "128m", "--cpus", "0.5", "--stop-timeout", "2",
		"--label", "ardents.qualification.project=" + observer.project, "ardents-node:" + observer.imageTag,
		"/usr/local/bin/ardents-qualify", "collector-node"}
	raw, err := observer.dockerBounded(ctx, 128, 4096, arguments...)
	observer.collectorID = string(bytesTrimSpace(raw))
	if err != nil {
		return err
	}
	if len(observer.collectorID) < 12 || len(observer.collectorID) > 64 {
		return errors.New("node resource collector did not start")
	}
	inspect, err := observer.dockerBounded(ctx, 256<<10, 4096, "inspect", observer.collectorID)
	if err != nil {
		return err
	}
	return observer.writeEvidenceFiles([]nodeEvidenceFile{{"collector-inspect.json", inspect}})
}

func (observer *nodeObserver) writeEvidenceFiles(files []nodeEvidenceFile) error {
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(observer.input.EvidenceRoot, file.name), file.raw, 0o600); err != nil {
			return fmt.Errorf("write node evidence %s: %w", file.name, err)
		}
	}
	return nil
}
