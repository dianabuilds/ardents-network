package node

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
)

func nodeMachineCommandError(raw []byte, runErr error, expectedPassReason string) error {
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return runErr
		}
		exitCode = exitErr.ExitCode()
	}
	resultErr := classifyNodeMachineResult(raw, exitCode, expectedPassReason)
	if errors.Is(resultErr, errInvalidNodeCampaign) && runErr != nil {
		return errors.Join(runErr, resultErr)
	}
	return resultErr
}

func nodeProductCommandError(runErr error, productMarker, message string) error {
	if runErr == nil {
		return nil
	}
	if strings.Contains(runErr.Error(), productMarker) {
		return errors.New(message)
	}
	return runErr
}

func classifyNodeMachineResult(raw []byte, exitCode int, expectedPassReason string) error {
	result, ok := decodeNodeMachineResult(raw)
	if !ok {
		return invalidNodeCampaign(errors.New("node machine command result is invalid"))
	}
	if exitCode == 0 && result.Verdict == "pass" && result.Reason == expectedPassReason {
		return nil
	}
	if exitCode == 1 && result.Verdict == "fail" {
		return errors.New(result.Reason)
	}
	return invalidNodeCampaign(errors.New("node machine command exit and verdict disagree"))
}

func decodeNodeMachineResult(raw []byte) (Result, bool) {
	var result Result
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Result{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Result{}, false
	}
	if (result.Verdict != "pass" && result.Verdict != "fail" && result.Verdict != "invalid") ||
		len(result.Reason) == 0 || len(result.Reason) > 4096 || result.EvidenceRoot != "" || result.EvidenceDigest != "" {
		return Result{}, false
	}
	return result, true
}
