package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/node/fixture"
)

func validateCampaign(input Campaign) (Campaign, error) {
	if campaignDuration(input.Mode) < 0 {
		return input, errors.New("node campaign mode is invalid")
	}
	if err := validateNodeSpecialInput(input, input.Mode); err != nil {
		return input, err
	}
	var err error
	if input.FixtureRoot, err = filepath.Abs(input.FixtureRoot); err != nil {
		return input, fmt.Errorf("resolve node fixture root: %w", err)
	}
	if input.EvidenceRoot, err = filepath.Abs(input.EvidenceRoot); err != nil {
		return input, fmt.Errorf("resolve node evidence root: %w", err)
	}
	if input.ComposeFile, err = filepath.Abs(input.ComposeFile); err != nil {
		return input, fmt.Errorf("resolve node Compose file: %w", err)
	}
	if err := fixture.Validate(input.FixtureRoot); err != nil {
		return input, fmt.Errorf("validate node fixture: %w", err)
	}
	if _, err := os.Stat(input.ComposeFile); err != nil {
		return input, fmt.Errorf("inspect node Compose file: %w", err)
	}
	if err := os.Mkdir(input.EvidenceRoot, 0o700); err != nil {
		return input, fmt.Errorf("create node evidence root: %w", err)
	}
	owner := filepath.Join(input.EvidenceRoot, ".ardents-node-evidence")
	if err := os.WriteFile(owner, []byte(nodeEvidenceOwner), 0o600); err != nil {
		return input, fmt.Errorf("write node evidence owner: %w", err)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return input, fmt.Errorf("encode node campaign input: %w", err)
	}
	if len(raw) > 16<<10 {
		return input, errors.New("node campaign input exceeds its bound")
	}
	path := filepath.Join(input.EvidenceRoot, "campaign-input.json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return input, fmt.Errorf("write node campaign input: %w", err)
	}
	return input, nil
}

func validateNodeSpecialInput(input Campaign, mode string) error {
	if len(input.Mode) > 32 || input.Mode != mode {
		return errors.New("node campaign mode is invalid")
	}
	if mode == "disk-wrapper" || mode == "evidence-fault" {
		if input.FixtureRoot != "" || input.EvidenceRoot != "" || input.ComposeFile != "" ||
			len(input.Addresses) != 0 || input.SecretRoot != "" || input.Injection != "" || input.ProbePlan != "" {
			return errors.New("node disk wrapper has irrelevant fields")
		}
		return nil
	}
	for _, value := range []string{input.FixtureRoot, input.EvidenceRoot, input.ComposeFile} {
		if len(value) == 0 || len(value) > 4096 {
			return errors.New("node campaign path is invalid")
		}
	}
	if campaignDuration(mode) >= 0 {
		if len(input.Addresses) != 0 || input.SecretRoot != "" || input.Injection != "" || input.ProbePlan != "" {
			return errors.New("node campaign has irrelevant fields")
		}
		return nil
	}
	return errors.New("node campaign mode is invalid")
}

func campaignDuration(mode string) time.Duration {
	selected, found := selectNodeCampaignMode(mode)
	if !found {
		return -1
	}
	return selected.duration
}

func validateNodeInjectionInput(input Campaign) error {
	if len(input.Mode) > 32 || input.Mode != "inject" || len(input.Injection) == 0 || len(input.Injection) > 16 ||
		input.FixtureRoot != "" || input.EvidenceRoot != "" || input.ComposeFile != "" {
		return errors.New("node injection input is invalid")
	}
	if input.Injection == "memory" || input.Injection == "cpu" || input.Injection == "nofile" {
		if len(input.Addresses) != 0 || input.SecretRoot != "" || input.ProbePlan != "" {
			return errors.New("node pressure injection has irrelevant fields")
		}
		return nil
	}
	if input.Injection == "emfile" {
		if len(input.Addresses) != 1 || input.SecretRoot != "" || input.ProbePlan != "" {
			return errors.New("node EMFILE injection input is invalid")
		}
		host, port, err := net.SplitHostPort(input.Addresses[0])
		if err != nil || net.ParseIP(host) == nil || port == "" || len(input.Addresses[0]) > 128 {
			return errors.New("node EMFILE injector address is invalid")
		}
		return nil
	}
	if input.Injection != "probe" || len(input.Addresses) != 2 || len(input.SecretRoot) == 0 ||
		len(input.SecretRoot) > 4096 || len(input.ProbePlan) == 0 || len(input.ProbePlan) > 4096 ||
		!filepath.IsAbs(input.SecretRoot) || !filepath.IsAbs(input.ProbePlan) {
		return errors.New("node probe injection input is invalid")
	}
	for _, address := range input.Addresses {
		if len(address) == 0 || len(address) > 128 {
			return errors.New("node injector address is invalid")
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil || net.ParseIP(host) == nil || port == "" {
			return errors.New("node injector address is invalid")
		}
	}
	return nil
}
