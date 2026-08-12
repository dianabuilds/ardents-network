package node

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func (observer *nodeObserver) capturePreflightEvidence(ctx context.Context) error {
	manifest, err := os.ReadFile(filepath.Join(observer.input.FixtureRoot, "manifest.json"))
	if err != nil || len(manifest) > 64<<10 {
		return errors.Join(err, errors.New("node fixture manifest evidence is invalid"))
	}
	seal, err := os.ReadFile(filepath.Join(observer.input.FixtureRoot, ".ardents-node-manifest.sha256"))
	if err != nil || len(seal) > 128 {
		return errors.Join(err, errors.New("node fixture manifest seal evidence is invalid"))
	}
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
	files := map[string][]byte{"manifest.json": manifest, "manifest.sha256": seal, "compose-resolved.yaml": config,
		"docker-version.json": version, "docker-info.json": info}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(observer.input.EvidenceRoot, name), raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (observer *nodeObserver) captureCandidateIdentity(ctx context.Context) error {
	image, err := observer.dockerBounded(ctx, 2<<20, 32<<10, "image", "inspect", "ardents-node:"+observer.imageTag)
	if err != nil || !bytes.Contains(image, []byte(observer.sourceDigest)) {
		return errors.Join(err, errors.New("node image is not bound to the captured source digest"))
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
	if err != nil || len(strings.Fields(string(identities))) != 5 {
		return errors.Join(err, errors.New("node topology identity is incomplete"))
	}
	arguments := append([]string{"inspect"}, strings.Fields(string(identities))...)
	topology, err := observer.dockerBounded(ctx, 4<<20, 32<<10, arguments...)
	if err != nil {
		return err
	}
	files := map[string][]byte{"image-inspect.json": image, "build-identity.json": build, "topology-inspect.json": topology}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(observer.input.EvidenceRoot, name), raw, 0o600); err != nil {
			return err
		}
	}
	return nil
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
	if err != nil || len(observer.collectorID) < 12 || len(observer.collectorID) > 64 {
		return errors.Join(err, errors.New("node resource collector did not start"))
	}
	inspect, err := observer.dockerBounded(ctx, 256<<10, 4096, "inspect", observer.collectorID)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(observer.input.EvidenceRoot, "collector-inspect.json"), inspect, 0o600)
}
