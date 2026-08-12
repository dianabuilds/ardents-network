package node

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (observer *nodeObserver) forceEMFILE(ctx context.Context) error {
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "node1"); err != nil {
		return err
	}
	observer.captureLogs(ctx, "node1")
	if _, err := observer.compose(ctx, "up", "-d", "--no-deps", "--force-recreate", "node1_emfile"); err != nil {
		return err
	}
	if err := observer.waitServiceReady(ctx, 15*time.Second, "node1_emfile"); err != nil {
		return errors.New("low-FD Node did not reach the isolated EMFILE cell")
	}
	raw, injectErr := observer.compose(ctx, "run", "--rm", "--no-deps", "emfile_harness", "/usr/local/bin/ardents-qualify",
		"inject-node", "--mode", "emfile", "--addresses", "172.30.3.11:4401")
	if appendErr := observer.appendCandidateEvidence(raw); appendErr != nil {
		return appendErr
	}
	if err := observer.waitStopped(ctx, 5*time.Second, "node1_emfile"); err != nil {
		return errors.Join(injectErr, errors.New("deliberate EMFILE did not cause bounded candidate exit"))
	}
	logs, logErr := observer.compose(ctx, "logs", "--no-color", "node1_emfile")
	if appendErr := observer.appendCandidateEvidence(logs); appendErr != nil {
		return appendErr
	}
	if logErr != nil || !strings.Contains(string(logs), "too many open files") {
		return errors.Join(injectErr, logErr, errors.New("EMFILE cell did not prove an actual descriptor exhaustion"))
	}
	if _, err := observer.compose(ctx, "rm", "-f", "node1_emfile"); err != nil {
		return err
	}
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	if err := observer.waitReady(ctx, 15*time.Second); err != nil {
		return errors.New("node did not recover after the isolated EMFILE cell")
	}
	return nil
}
