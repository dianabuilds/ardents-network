//go:build h4_3b_multihost || h4_8_a11

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"
)

func (remote h43RemoteC2) captureEvidence(t *testing.T) []byte {
	t.Helper()
	output, err := remote.captureEvidenceContext(t.Context())
	if err != nil {
		t.Fatalf("capture bounded A11 remote evidence: %v\n%s", err, output)
	}
	return output
}

func (remote h43RemoteC2) captureFailureEvidence() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return remote.captureEvidenceContext(ctx)
}

func (remote h43RemoteC2) captureEvidenceContext(ctx context.Context) ([]byte, error) {
	command := h48A11RemoteEvidenceCommand(remote.environment)
	output, err := remote.commandContext(ctx, command).CombinedOutput()
	if err != nil {
		return output, err
	}
	if len(output) == 0 || len(output) > 1<<20 {
		return output, errors.New("A11 remote evidence is empty or exceeds 1048576 bytes")
	}
	return output, nil
}

func h48A11RemoteEvidenceCommand(environment h43MultiHostEnvironment) string {
	directory := h43ShellQuote(environment.remoteDirectory)
	container := h43ShellQuote(environment.container)
	return "set -u; printf 'schema=ardents-h4-8-a11-remote-evidence-v1\\n[container-state]\\n'; " +
		"if container_state=$(docker inspect " + container + " --format '{{json .State}}' 2>&1); then " +
		"printf '%s\\n' \"$container_state\"; else printf '{\"container_available\":false}\\ncontainer_error=%s\\n' \"$container_state\"; fi; " +
		"printf '[staged-inventory-sha256]\\n'; if [ -d " + directory + " ]; then (cd " + directory + " && " +
		"find . -type f -print | LC_ALL=C sort | while IFS= read -r path; do bytes=$(wc -c < \"$path\"); digest=$(sha256sum \"$path\" | cut -d' ' -f1); printf '%s\\t%s\\t%s\\n' \"$digest\" \"$bytes\" \"${path#./}\"; done; " +
		"printf '[role-exit-statuses]\\n'; if [ -f remote-role-exit-statuses.jsonl ]; then cat remote-role-exit-statuses.jsonl; else printf 'unavailable\\n'; fi; " +
		"printf '[role-output]\\n'; for file in *.log *.err; do if [ -f \"$file\" ]; then printf '===%s===\\n' \"$file\"; cat \"$file\"; fi; done); " +
		"else printf 'staged_root_available=false\\n[role-exit-statuses]\\nunavailable\\n[role-output]\\n'; fi"
}
