package siteexperiment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var referenceRoles = []string{"authority", "administration", "gateway", "relay", "client-endpoint", "http-client", "http-application"}

type referenceProcess struct {
	repositoryRoot     string
	project            string
	image              string
	environment        []string
	root               string
	evidence           string
	serviceSocket      string
	routeSocket        string
	containerIDs       []string
	expectedTarget     string
	expectedGeneration uint64
	expectedSuperseded bool
}

func startReferenceApplication(ctx context.Context, repositoryRoot, runDirectory, image, nonce string, sequence int, fixture *authorityFixture, superseded *instanceCredential, progress func(string) error) (*referenceProcess, error) {
	root := filepath.Join(runDirectory, fmt.Sprintf("attempt-%03d", sequence), "reference")
	directories, err := prepareReferenceDirectories(root)
	if err != nil {
		return nil, err
	}
	authorityRole := authorityConfig(fixture)
	if superseded != nil {
		authorityRole.AdminRequests = 2
	}
	if err := writeBoundedJSON(filepath.Join(directories["authority-config"], "authority.json"), authorityRole); err != nil {
		return nil, err
	}
	if err := os.Chmod(filepath.Join(directories["authority-config"], "authority.json"), 0o644); err != nil {
		return nil, err
	}
	if err := writeBoundedJSON(filepath.Join(directories["client-config"], "client.json"), publicClientConfig(fixture)); err != nil {
		return nil, err
	}
	if err := os.Chmod(filepath.Join(directories["client-config"], "client.json"), 0o644); err != nil {
		return nil, err
	}
	adminConfig := administrationRoleConfig{Schema: roleConfigSchema, SupersededCredential: superseded}
	if err := writeBoundedJSON(filepath.Join(directories["admin-config"], "administration.json"), adminConfig); err != nil {
		return nil, err
	}
	if err := os.Chmod(filepath.Join(directories["admin-config"], "administration.json"), 0o644); err != nil {
		return nil, err
	}
	for _, name := range []string{"authority-config", "admin-config", "client-config"} {
		if err := os.Chmod(directories[name], 0o755); err != nil {
			return nil, err
		}
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", runDirectory, sequence)))
	process := &referenceProcess{
		repositoryRoot: repositoryRoot, project: "ardents-gatec-" + hex.EncodeToString(digest[:8]), root: root,
		image:    image,
		evidence: directories["evidence"], serviceSocket: filepath.Join(directories["service"], "app.sock"), routeSocket: filepath.Join(directories["route"], "app.sock"),
		expectedTarget: fixture.target, expectedGeneration: fixture.instanceGeneration,
		expectedSuperseded: superseded != nil,
		environment: append(os.Environ(),
			"GATEC_REFERENCE_IMAGE="+image, "GATEC_NONCE="+nonce,
			"GATEC_CLIENT_DIR="+directories["client"], "GATEC_SERVICE_DIR="+directories["service"], "GATEC_ROUTE_DIR="+directories["route"],
			"GATEC_ADMIN_DIR="+directories["admin"], "GATEC_GATEWAY_AUTHORITY_DIR="+directories["gateway-authority"], "GATEC_GATEWAY_DIR="+directories["gateway"], "GATEC_EVIDENCE_DIR="+directories["evidence"],
			"GATEC_AUTHORITY_CONFIG_DIR="+directories["authority-config"], "GATEC_ADMIN_CONFIG_DIR="+directories["admin-config"], "GATEC_CLIENT_CONFIG_DIR="+directories["client-config"],
		),
	}
	if _, err := process.compose(ctx, append([]string{"up", "--detach", "--no-build", "--pull", "never"}, referenceRoles...)...); err != nil {
		return process, err
	}
	if err := process.captureContainerIDs(ctx); err != nil {
		return process, err
	}
	if err := progress("reference-compose-started"); err != nil {
		return process, err
	}
	if err := process.waitSockets(ctx); err != nil {
		return process, err
	}
	if err := progress("reference-sockets-ready"); err != nil {
		return process, err
	}
	if err := process.waitPublication(ctx); err != nil {
		return process, err
	}
	if err := progress("reference-published"); err != nil {
		return process, err
	}
	if err := process.inspectIsolation(ctx); err != nil {
		return process, err
	}
	if err := progress("reference-isolation-verified"); err != nil {
		return process, err
	}
	return process, nil
}

func (process *referenceProcess) waitPublication(ctx context.Context) error {
	path := filepath.Join(process.evidence, "administration", "publication.json")
	for {
		var publication struct {
			Status              string `json:"status"`
			Target              string `json:"target"`
			InstanceGeneration  uint64 `json:"instance_generation"`
			AuthorityReceived   bool   `json:"authority_received"`
			PrivateKeyReceived  bool   `json:"instance_private_key_received"`
			SupersededAttempted bool   `json:"superseded_publication_attempted"`
			SupersededRejected  bool   `json:"superseded_publication_rejected"`
		}
		_, statErr := os.Stat(path)
		if statErr == nil {
			if err := readStrictEvidence(path, &publication); err != nil {
				return err
			}
			if publication.Status != "published" || publication.Target != process.expectedTarget || publication.InstanceGeneration != process.expectedGeneration || publication.AuthorityReceived || publication.PrivateKeyReceived || publication.SupersededAttempted != process.expectedSuperseded || publication.SupersededRejected != process.expectedSuperseded {
				return scenarioFailure(errors.New("reference Site publication handle is invalid"))
			}
			return nil
		}
		if !os.IsNotExist(statErr) {
			return statErr
		}
		if err := process.detectFailedRole(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (process *referenceProcess) waitSockets(ctx context.Context) error {
	for {
		ready, err := referenceSocketsReady(process.serviceSocket, process.routeSocket)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if err := process.detectFailedRole(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func referenceSocketsReady(paths ...string) (bool, error) {
	ready := true
	for _, path := range paths {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			ready = false
			continue
		}
		if err != nil {
			return false, fmt.Errorf("inspect reference Site socket %q: %w", path, err)
		}
		if info.Mode()&os.ModeSocket == 0 {
			return false, fmt.Errorf("reference Site socket %q has an invalid filesystem type", path)
		}
	}
	return ready, nil
}

func (process *referenceProcess) compose(ctx context.Context, arguments ...string) ([]byte, error) {
	base := []string{"compose", "--project-name", process.project, "--file", filepath.Join(process.repositoryRoot, "reference-site", "compose.yaml")}
	command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
	command.Env = process.environment
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("reference Site Compose: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (process *referenceProcess) wait(ctx context.Context) error {
	if len(process.containerIDs) != len(referenceRoles) {
		return errors.New("reference Site container identity set is incomplete")
	}
	for {
		states, err := process.referenceRoleStates(ctx)
		if err != nil {
			return matrixOperational(err)
		}
		if failed := firstFailedReferenceRole(states); failed != nil {
			return matrixOperational(process.describeFailedRole(ctx, failed))
		}
		complete := true
		for _, state := range states {
			complete = complete && !state.State.Running
		}
		if complete {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (process *referenceProcess) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, downErr := process.compose(ctx, "down", "--volumes", "--remove-orphans", "--timeout", "5")
	for _, arguments := range [][]string{
		{"ps", "--all", "--quiet", "--filter", "label=com.docker.compose.project=" + process.project},
		{"network", "ls", "--quiet", "--filter", "label=com.docker.compose.project=" + process.project},
		{"volume", "ls", "--quiet", "--filter", "label=com.docker.compose.project=" + process.project},
	} {
		output, inspectErr := exec.CommandContext(ctx, "docker", arguments...).Output()
		if inspectErr != nil {
			return errors.Join(downErr, inspectErr)
		}
		if strings.TrimSpace(string(output)) != "" {
			return errors.Join(downErr, errors.New("reference Site Docker resources remain after cleanup"))
		}
	}
	return downErr
}

func verifyReferenceImage(ctx context.Context, image, sourceSHA string) error {
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--pull", "never", "--network", "none", "--entrypoint", "/bin/sh", image, "-c", "cat /usr/share/ardents/gate-c-source.sha256")
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != sourceSHA {
		return errors.New("reference Site image is not bound to the current source identity")
	}
	return nil
}
