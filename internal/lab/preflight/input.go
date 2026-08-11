package preflight

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func parseInputFile(path string) (input, error) {
	file, err := os.Open(path)
	if err != nil {
		return input{}, err
	}
	defer file.Close()
	return parseInput(file)
}

func parseInput(reader io.Reader) (input, error) {
	properties := make(map[string]string)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(key) == "" {
			return input{}, fmt.Errorf("invalid input line %q", line)
		}
		key = strings.TrimSpace(key)
		if _, duplicate := properties[key]; duplicate {
			return input{}, fmt.Errorf("duplicate input field %q", key)
		}
		properties[key] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return input{}, err
	}

	known := map[string]bool{
		"schema_version": true, "run_id": true, "seed": true,
		"git_revision": true, "git_dirty": true,
		"host_os": true, "host_arch": true, "host_ubuntu_version": true,
		"image_reference": true, "expected_image_manifest_digest": true,
		"observed_image_manifest_digest": true, "image_id": true, "carrier_lab_image_id": true,
		"binary_sha256":   true,
		"go_archive_name": true, "expected_go_archive_sha256": true,
		"observed_go_archive_sha256": true, "repository_mount": true,
		"container_network": true, "go_proxy": true, "go_cache": true,
		"go_mod_cache": true, "tool_bash": true, "tool_git": true,
		"tool_docker_client": true, "tool_docker_server": true,
		"tool_sha256sum": true, "tool_tar": true,
	}
	for key := range properties {
		if !known[key] {
			return input{}, fmt.Errorf("unknown input field %q", key)
		}
	}
	dirty, err := strconv.ParseBool(properties["git_dirty"])
	if err != nil {
		return input{}, fmt.Errorf("git_dirty: %w", err)
	}
	return input{
		SchemaVersion: properties["schema_version"], RunID: properties["run_id"], Seed: properties["seed"],
		GitRevision: properties["git_revision"], GitDirty: dirty,
		HostOS: properties["host_os"], HostArch: properties["host_arch"], HostUbuntuVersion: properties["host_ubuntu_version"],
		ImageReference: properties["image_reference"], ExpectedImageManifestDigest: properties["expected_image_manifest_digest"],
		ObservedImageManifestDigest: properties["observed_image_manifest_digest"], ImageID: properties["image_id"],
		CarrierLabImageID: properties["carrier_lab_image_id"],
		BinarySHA256:      properties["binary_sha256"],
		GoArchiveName:     properties["go_archive_name"], ExpectedGoArchiveSHA256: properties["expected_go_archive_sha256"],
		ObservedGoArchiveSHA256: properties["observed_go_archive_sha256"], RepositoryMount: properties["repository_mount"],
		ContainerNetwork: properties["container_network"], GoProxy: properties["go_proxy"], GoCache: properties["go_cache"],
		GoModCache: properties["go_mod_cache"],
		Tools: toolVersions{Bash: properties["tool_bash"], Git: properties["tool_git"], DockerClient: properties["tool_docker_client"],
			DockerServer: properties["tool_docker_server"], SHA256Sum: properties["tool_sha256sum"], Tar: properties["tool_tar"]},
	}, nil
}

func validateInput(value input) []failureReason {
	var reasons []failureReason
	if value.SchemaVersion != inputSchemaVersion {
		reasons = append(reasons, failure(reasonInvalidSchemaVersion, stageValidateInput, fmt.Sprintf("schema_version must be %q", inputSchemaVersion)))
	}
	required := []struct{ name, value string }{
		{"run_id", value.RunID}, {"seed", value.Seed}, {"git_revision", value.GitRevision},
		{"host_os", value.HostOS}, {"host_arch", value.HostArch}, {"host_ubuntu_version", value.HostUbuntuVersion},
		{"image_reference", value.ImageReference}, {"expected_image_manifest_digest", value.ExpectedImageManifestDigest},
		{"observed_image_manifest_digest", value.ObservedImageManifestDigest}, {"image_id", value.ImageID},
		{"carrier_lab_image_id", value.CarrierLabImageID},
		{"binary_sha256", value.BinarySHA256},
		{"go_archive_name", value.GoArchiveName}, {"expected_go_archive_sha256", value.ExpectedGoArchiveSHA256},
		{"observed_go_archive_sha256", value.ObservedGoArchiveSHA256}, {"repository_mount", value.RepositoryMount},
		{"container_network", value.ContainerNetwork}, {"go_proxy", value.GoProxy}, {"go_cache", value.GoCache}, {"go_mod_cache", value.GoModCache},
	}
	for _, field := range required {
		if field.value == "" {
			reasons = append(reasons, failure(reasonMissingRequiredField, stageValidateInput, "required field is missing: "+field.name))
		}
	}
	if value.GitRevision != "" {
		decoded, err := hex.DecodeString(value.GitRevision)
		if err != nil || (len(decoded) != 20 && len(decoded) != 32) {
			reasons = append(reasons, failure(reasonMissingRequiredField, stageValidateInput, "git_revision must be a full hexadecimal object ID"))
		}
	}
	if value.BinarySHA256 != "" {
		decoded, err := hex.DecodeString(value.BinarySHA256)
		if err != nil || len(decoded) != sha256.Size {
			reasons = append(reasons, failure(reasonMissingRequiredField, stageValidateInput, "binary_sha256 must be a full SHA-256 digest"))
		}
	}
	if value.CarrierLabImageID != "" {
		algorithm, digest, found := strings.Cut(value.CarrierLabImageID, ":")
		decoded, err := hex.DecodeString(digest)
		if !found || algorithm != "sha256" || err != nil || len(decoded) != sha256.Size {
			reasons = append(reasons, failure(reasonMissingRequiredField, stageValidateInput, "carrier_lab_image_id must be a full sha256 image ID"))
		}
	}
	tools := []struct{ name, value string }{
		{"bash", value.Tools.Bash}, {"git", value.Tools.Git}, {"docker_client", value.Tools.DockerClient},
		{"docker_server", value.Tools.DockerServer}, {"sha256sum", value.Tools.SHA256Sum}, {"tar", value.Tools.Tar},
	}
	for _, tool := range tools {
		if tool.value == "" {
			reasons = append(reasons, failure(reasonMissingRequiredTool, stageValidateInput, "required tool is unavailable: "+tool.name))
		}
	}
	return reasons
}
