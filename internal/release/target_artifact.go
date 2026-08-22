package release

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

func verifyArtifactDigest(artifact, expected []byte) error {
	actual := sha256.Sum256(artifact)
	if !bytes.Equal(actual[:], expected) {
		return errors.New("artifact digest does not match the target identity")
	}
	return nil
}

func confineTargetPath(targetPath string) error {
	if targetPath == "" {
		return errors.New("target path is empty")
	}
	if strings.Contains(targetPath, `\`) {
		return errors.New("target path uses a non-canonical separator")
	}
	decoded, err := url.PathUnescape(targetPath)
	if err != nil {
		return fmt.Errorf("decode target path: %w", err)
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+decoded), "/")
	if cleaned != decoded {
		return errors.New("target path is not confined")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return errors.New("target path escapes the offline envelope")
		}
	}
	return nil
}
