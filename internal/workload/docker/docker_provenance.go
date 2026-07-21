package docker

import (
	"fmt"
	"strings"

	"github.com/distribution/reference"
)

func (e *Executor) admitImage(image string) error {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return fmt.Errorf("container image reference is invalid")
	}
	if _, ok := named.(reference.Digested); !ok {
		return fmt.Errorf("container image must use an immutable sha256 digest")
	}
	registry := strings.ToLower(reference.Domain(named))
	if _, allowed := e.allowedRegistries[registry]; !allowed {
		return fmt.Errorf("container image registry %s is not allowed", registry)
	}
	return nil
}
