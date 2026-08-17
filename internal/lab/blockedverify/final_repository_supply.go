package blockedverify

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type finalVerifierSupplyLock struct {
	Schema           string `json:"schema"`
	GoBuilderImageID string `json:"go_builder_image_id"`
	GoBuilderVersion string `json:"go_builder_version"`
	GoArchiveSHA256  string `json:"go_archive_sha256"`
	GoRecipeSHA256   string `json:"go_builder_recipe_sha256"`
	GoModuleSHA256   string `json:"go_module_cache_sha256"`
	ToolImageID      string `json:"tool_image_id"`
	ToolLockSHA256   string `json:"tool_lock_sha256"`
	CarrierSHA256    string `json:"carrier_sha256"`
}

func verifyFinalRepositorySupply(workspace string, spec finalSpec) []string {
	if err := verifyFinalRepositorySupplyValue(workspace, spec); err != nil {
		return []string{err.Error()}
	}
	return nil
}

func verifyFinalRepositorySupplyValue(workspace string, spec finalSpec) error {
	root, err := filepath.Abs(workspace)
	if err != nil || workspace == "" {
		return errors.New("final verifier repository root is unavailable")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(root) {
		return errors.New("final verifier repository root is aliased")
	}
	if info, statErr := os.Stat(filepath.Join(root, ".git")); statErr != nil || !info.IsDir() {
		return errors.New("final verifier repository identity is unavailable")
	}
	commit, err := boundedSupplyGitOutput("git", "-C", root, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(commit)) != spec.RepositoryCommit {
		return errors.New("final verifier repository commit differs from the manifest")
	}
	archiveHash, err := finalRepositoryArchiveHash(root, spec.RepositoryCommit)
	if err != nil || archiveHash != spec.SourceSHA256 {
		return errors.Join(err, errors.New("final verifier repository archive differs from the manifest"))
	}
	raw, err := boundedSupplyGitOutput("git", "-C", root, "show",
		spec.RepositoryCommit+":tests/live/stage5-final/supply.lock.json")
	if err != nil || len(raw) == 0 || len(raw) > 4<<10 {
		return errors.Join(err, errors.New("accepted repository supply lock is unavailable"))
	}
	recipe, err := boundedSupplyGitOutput("git", "-C", root, "show",
		spec.RepositoryCommit+":tests/live/stage5-final/go-builder.Dockerfile")
	recipeDigest := sha256.Sum256(recipe)
	if err != nil || hex.EncodeToString(recipeDigest[:]) != spec.ProductReceipt.GoRecipeSHA256 {
		return errors.Join(err, errors.New("accepted Go builder recipe differs from its product receipt"))
	}
	toolLock, err := boundedSupplyGitOutput("git", "-C", root, "show",
		spec.RepositoryCommit+":lab/carrier/tools.lock")
	toolDigest := sha256.Sum256(toolLock)
	if err != nil || hex.EncodeToString(toolDigest[:]) != spec.ToolReceipt.ToolLockSHA256 {
		return errors.Join(err, errors.New("accepted Carrier tool lock differs from its product receipt"))
	}
	return verifyFinalSupplyLockBytes(raw, spec)
}

func verifyFinalSupplyLockBytes(raw []byte, spec finalSpec) error {
	var lock finalVerifierSupplyLock
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("accepted repository supply lock is malformed")
	}
	canonical, err := json.MarshalIndent(lock, "", "  ")
	digest := sha256.Sum256(raw)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) ||
		hex.EncodeToString(digest[:]) != spec.SupplyLock.SHA256 || int64(len(raw)) != spec.SupplyLock.Bytes {
		return errors.New("accepted repository supply lock differs from its manifest commitment")
	}
	if lock.Schema != "ardents-h3-s5-supply-lock-v1" || lock.GoBuilderImageID != spec.GoBuilderImageID ||
		lock.GoBuilderVersion != spec.GoBuilderVersion ||
		lock.GoArchiveSHA256 != "708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89" ||
		lock.GoArchiveSHA256 != spec.ProductReceipt.GoArchiveSHA256 ||
		lock.GoRecipeSHA256 != spec.ProductReceipt.GoRecipeSHA256 ||
		lock.GoModuleSHA256 != spec.ProductReceipt.GoModuleSHA256 ||
		lock.ToolImageID != spec.ToolImageID || lock.ToolLockSHA256 != spec.ToolReceipt.ToolLockSHA256 ||
		lock.CarrierSHA256 != spec.ToolReceipt.CarrierSHA256 {
		return errors.New("final supply differs from the accepted repository lock")
	}
	return nil
}
