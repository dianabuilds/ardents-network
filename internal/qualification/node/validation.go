package node

import (
	"errors"
	"net"
	"path/filepath"
)

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
	if mode == "short" {
		if len(input.Addresses) != 0 || input.SecretRoot != "" || input.Injection != "" || input.ProbePlan != "" {
			return errors.New("node short campaign has irrelevant fields")
		}
		return nil
	}
	return errors.New("node campaign mode is invalid")
}

func validateNodeInjectionInput(input Campaign) error {
	if len(input.Mode) > 32 || input.Mode != "inject" || len(input.Injection) == 0 || len(input.Injection) > 16 ||
		input.FixtureRoot != "" || input.EvidenceRoot != "" || input.ComposeFile != "" {
		return errors.New("node injection input is invalid")
	}
	if input.Injection == "memory" || input.Injection == "cpu" {
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
