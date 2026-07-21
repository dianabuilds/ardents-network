package docker

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func validateDockerEndpoint(raw string, allowInsecureRemote bool) error {
	endpoint, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("docker endpoint configuration is invalid")
	}
	switch strings.ToLower(endpoint.Scheme) {
	case "unix", "npipe":
		return nil
	case "tcp":
		if allowInsecureRemote {
			return nil
		}
		if os.Getenv("DOCKER_TLS_VERIFY") != "1" || strings.TrimSpace(os.Getenv("DOCKER_CERT_PATH")) == "" {
			return fmt.Errorf("remote Docker endpoint requires TLS verification and client certificates")
		}
		return nil
	default:
		return fmt.Errorf("docker endpoint scheme is not supported")
	}
}
