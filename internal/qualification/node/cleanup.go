package node

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

func (observer *nodeObserver) cleanup() {
	observer.cleanupOnce.Do(func() {
		observer.captureLogs(context.Background(), "source1", "source2", "endpoint", "node1", "node2")
		processes, processErr := observer.compose(context.Background(), "ps", "-a", "--format", "{{.Service}}\t{{.State}}\t{{.ExitCode}}\t{{.ID}}")
		observer.recordEvidenceError(processErr)
		observer.recordEvidenceError(os.WriteFile(filepath.Join(observer.input.EvidenceRoot, "processes-final.txt"), processes, 0o600))
		_, observer.cleanupErr = observer.compose(context.Background(), "--profile", "fault", "down", "--remove-orphans", "--rmi", "all", "--timeout", "15")
		observer.cancel()
		observer.work.Wait()
		remaining, remainErr := observer.compose(context.Background(), "ps", "-q")
		if remainErr != nil || len(bytesTrimSpace(remaining)) != 0 {
			observer.cleanupErr = errors.Join(observer.cleanupErr, errors.New("node cleanup left candidate containers"), remainErr)
		}
		observer.cleanupErr = errors.Join(observer.cleanupErr, observer.verifyDockerQuiescence(context.Background()))
	})
}

func (observer *nodeObserver) verifyDockerQuiescence(ctx context.Context) error {
	filters := [][]string{
		{"ps", "-aq", "--filter", "label=com.docker.compose.project=" + observer.project},
		{"network", "ls", "-q", "--filter", "label=com.docker.compose.project=" + observer.project},
		{"volume", "ls", "-q", "--filter", "label=com.docker.compose.project=" + observer.project},
		{"images", "-q", "ardents-node:" + observer.imageTag},
	}
	for _, arguments := range filters {
		raw, err := observer.dockerBounded(ctx, 4096, 4096, arguments...)
		if err != nil || len(bytesTrimSpace(raw)) != 0 {
			return errors.Join(err, errors.New("node cleanup left an owned Docker process, network, volume, or image"))
		}
	}
	return nil
}

func (observer *nodeObserver) captureLogs(ctx context.Context, services ...string) {
	for _, service := range services {
		identity, err := observer.compose(ctx, "ps", "-a", "-q", service)
		id := string(bytesTrimSpace(identity))
		if err != nil || len(id) < 12 {
			observer.recordEvidenceError(errors.Join(err, errors.New("node candidate identity capture failed")))
			continue
		}
		if observer.captured[id] {
			continue
		}
		logs, err := observer.compose(ctx, "logs", "--no-color", service)
		if err != nil {
			observer.recordEvidenceError(err)
			continue
		}
		err = observer.appendCandidateEvidence(logs)
		if err == nil {
			observer.captured[id] = true
		}
	}
}

func (observer *nodeObserver) appendCandidateEvidence(raw []byte) error {
	file, err := os.OpenFile(filepath.Join(observer.input.EvidenceRoot, "candidate-events.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		observer.recordEvidenceError(err)
		return err
	}
	_, writeErr := file.Write(raw)
	err = errors.Join(writeErr, file.Close())
	observer.recordEvidenceError(err)
	return err
}
