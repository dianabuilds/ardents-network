package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type toolingBaseInspect struct {
	ID          string   `json:"Id"`
	RepoDigests []string `json:"RepoDigests"`
}

func verifyLocalToolingBaseWithDocker(identity toolBundle) (string, error) {
	run := func(name string, arguments ...string) ([]byte, error) {
		return exec.Command(name, arguments...).CombinedOutput()
	}
	return verifyLocalToolingBase(identity, run)
}

func verifyLocalToolingBase(identity toolBundle, run externalCommand) (string, error) {
	output, err := run("docker", "image", "inspect", identity.BaseImage)
	if err != nil {
		return "", fmt.Errorf("pinned tooling base image is not present locally: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var inspections []toolingBaseInspect
	if err := json.Unmarshal(output, &inspections); err != nil || len(inspections) != 1 {
		return "", errors.New("cannot decode the local tooling base image identity")
	}
	inspection := inspections[0]
	if !validImageID(inspection.ID) {
		return "", errors.New("local tooling base has an invalid image ID")
	}
	found := false
	for _, digest := range inspection.RepoDigests {
		found = found || digest == identity.BaseImage
	}
	if !found {
		return "", errors.New("local tooling base does not expose the locked repository digest")
	}
	return inspection.ID, nil
}
