package node

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (observer *nodeObserver) forceEMFILE(ctx context.Context) error {
	observer.setExpectedAbsence(true, "node1")
	defer observer.setExpectedAbsence(false, "node1")
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "node1"); err != nil {
		return err
	}
	observer.captureLogs(ctx, "node1")
	if _, err := observer.compose(ctx, "up", "-d", "--no-deps", "--force-recreate", "node1_emfile"); err != nil {
		return err
	}
	if err := observer.waitServiceReady(ctx, 15*time.Second, "node1_emfile"); err != nil {
		return nodeCandidateFailure("low-FD Node did not reach the isolated EMFILE cell", err)
	}
	targetID, err := observer.serviceID(ctx, "node1_emfile")
	if err != nil {
		return err
	}
	limitRaw, limitErr := observer.docker(ctx, "exec", targetID, "/usr/local/bin/ardents-qualify",
		"inject-node", "--mode", "nofile")
	if appendErr := observer.appendCandidateEvidence(limitRaw); appendErr != nil {
		return appendErr
	}
	if err := nodeMachineCommandError(limitRaw, limitErr, "candidate descriptor limit lowered"); err != nil {
		return err
	}
	raw, injectErr := observer.compose(ctx, "run", "--rm", "--no-deps", "emfile_harness", "/usr/local/bin/ardents-qualify",
		"inject-node", "--mode", "emfile", "--addresses", "172.30.3.11:4401")
	if appendErr := observer.appendCandidateEvidence(append(raw, []byte("\n"+errorText(injectErr)+"\n")...)); appendErr != nil {
		return appendErr
	}
	if err := nodeMachineCommandError(raw, injectErr, "EMFILE descriptor occupancy completed"); err != nil {
		return err
	}
	if err := observer.waitStopped(ctx, 5*time.Second, "node1_emfile"); err != nil {
		return nodeCandidateFailure("deliberate EMFILE did not cause bounded candidate exit", err)
	}
	logs, logErr := observer.compose(ctx, "logs", "--no-color", "node1_emfile")
	if appendErr := observer.appendCandidateEvidence(logs); appendErr != nil {
		return appendErr
	}
	if logErr != nil {
		return logErr
	}
	if !strings.Contains(string(logs), "too many open files") {
		return errors.New("EMFILE cell did not prove an actual descriptor exhaustion")
	}
	if _, err := observer.compose(ctx, "rm", "-f", "node1_emfile"); err != nil {
		return err
	}
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	if err := observer.waitServiceReady(ctx, 15*time.Second, "node1"); err != nil {
		return nodeCandidateFailure("node did not recover after the isolated EMFILE cell", err)
	}
	return nil
}
