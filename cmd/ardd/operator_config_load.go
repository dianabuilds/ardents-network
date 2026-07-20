package main

import (
	"fmt"
	"os"
	"strings"

	runtimeconfig "ardents/internal/runtime/config"
)

func loadOperatorRuntimeConfig(path string) (runtimeConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("open operator configuration: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("inspect operator configuration: %w", err)
	}
	if !info.Mode().IsRegular() {
		return runtimeConfig{}, fmt.Errorf("operator configuration must be a regular file")
	}
	doc, err := runtimeconfig.Decode(file)
	if err != nil {
		return runtimeConfig{}, err
	}
	doc, err = resolveOperatorDocument(doc)
	if err != nil {
		return runtimeConfig{}, err
	}
	token, err := operatorAPIToken(doc.API.TokenFile)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg, err := runtimeConfigFromDocument(doc, token)
	if err != nil {
		return runtimeConfig{}, err
	}
	manager, err := newOperatorConfigManager(path, doc)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg.Node.OperatorConfig = manager
	return cfg, nil
}

func newOperatorConfigManager(path string, doc runtimeconfig.Document) (*runtimeconfig.Manager, error) {
	manager, err := runtimeconfig.NewManager(path, doc, configureOperatorLogging(doc.Logging))
	if err != nil {
		return nil, err
	}
	if err := manager.RegisterResolver(resolveOperatorDocument); err != nil {
		return nil, err
	}
	if err := registerOperatorCandidateValidator(manager); err != nil {
		return nil, err
	}
	return manager, nil
}

func resolveOperatorDocument(doc runtimeconfig.Document) (runtimeconfig.Document, error) {
	token := strings.TrimSpace(os.Getenv(apiTokenEnv))
	path := strings.TrimSpace(os.Getenv(apiTokenFileEnv))
	if token != "" && path != "" {
		return runtimeconfig.Document{}, fmt.Errorf("configure only one of %s and %s", apiTokenEnv, apiTokenFileEnv)
	}
	if token != "" {
		doc.API.TokenFile = "environment-secret"
	} else if path != "" {
		doc.API.TokenFile = path
	}
	return doc, nil
}

func registerOperatorCandidateValidator(manager *runtimeconfig.Manager) error {
	return manager.RegisterValidator(func(candidate runtimeconfig.Document) error {
		candidateToken, err := operatorAPIToken(candidate.API.TokenFile)
		if err != nil {
			return err
		}
		_, err = runtimeConfigFromDocument(candidate, candidateToken)
		return err
	})
}

func operatorAPIToken(documentPath string) (string, error) {
	if strings.TrimSpace(os.Getenv(apiTokenEnv)) != "" || strings.TrimSpace(os.Getenv(apiTokenFileEnv)) != "" {
		token, err := loadAPIToken()
		if err != nil {
			if strings.Contains(err.Error(), "configure only one") {
				return "", err
			}
			return "", fmt.Errorf("api credential source is unavailable or invalid")
		}
		return token, nil
	}
	if strings.TrimSpace(documentPath) == "" {
		return "", fmt.Errorf("api.token_file or %s is required", apiTokenEnv)
	}
	token, err := readAPITokenFile(documentPath)
	if err != nil {
		return "", fmt.Errorf("api credential source is unavailable or invalid")
	}
	return token, nil
}

func operatorObservabilityToken(documentPath string) (string, error) {
	if strings.TrimSpace(documentPath) == "" {
		return "", nil
	}
	token, err := readAPITokenFile(documentPath)
	if err != nil {
		return "", fmt.Errorf("observability credential source is unavailable or invalid")
	}
	return token, nil
}
