package routeexperiment

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type referenceInputs struct {
	Packages      map[string]string
	Wheels        map[string]string
	Archive       string
	ArchiveSHA256 string
}

func verifyReferenceDirectory(repositoryRoot, directory string) (string, error) {
	directory, err := canonicalDirectory(directory)
	if err != nil {
		return "", err
	}
	repositoryLock := filepath.Join(repositoryRoot, "carrier-lab", "reference.lock")
	preparedLock := filepath.Join(directory, "reference.lock")
	repositoryDigest, err := hashFile(repositoryLock)
	if err != nil {
		return "", err
	}
	preparedDigest, err := hashFile(preparedLock)
	if err != nil || preparedDigest != repositoryDigest {
		return "", errors.New("prepared reference lock differs from the repository lock")
	}
	inputs, err := readReferenceLock(repositoryLock)
	if err != nil {
		return "", err
	}
	rootEntries, err := os.ReadDir(directory)
	if err != nil || len(rootEntries) != 4 {
		return "", errors.New("prepared reference root is not exact")
	}
	expectedRoot := map[string]bool{"packages": true, "wheelhouse": true, "reference.lock": true, inputs.Archive: true}
	for _, entry := range rootEntries {
		if !expectedRoot[entry.Name()] {
			return "", fmt.Errorf("unexpected prepared reference input %s", entry.Name())
		}
	}
	packageDirectory := filepath.Join(directory, "packages")
	entries, err := os.ReadDir(packageDirectory)
	if err != nil || len(entries) != len(inputs.Packages) {
		return "", errors.New("prepared Tor package set is not exact")
	}
	for _, entry := range entries {
		expected, found := inputs.Packages[entry.Name()]
		if entry.IsDir() || !found {
			return "", fmt.Errorf("unexpected prepared Tor input %s", entry.Name())
		}
		digest, err := hashFile(filepath.Join(packageDirectory, entry.Name()))
		if err != nil || digest != expected {
			return "", fmt.Errorf("prepared Tor input %s has the wrong digest", entry.Name())
		}
	}
	wheelDirectory := filepath.Join(directory, "wheelhouse")
	wheels, err := os.ReadDir(wheelDirectory)
	if err != nil || len(wheels) != len(inputs.Wheels) {
		return "", errors.New("prepared Chutney wheel set is not exact")
	}
	for _, entry := range wheels {
		expected, found := inputs.Wheels[entry.Name()]
		digest, hashErr := hashFile(filepath.Join(wheelDirectory, entry.Name()))
		if entry.IsDir() || !found || hashErr != nil || digest != expected {
			return "", fmt.Errorf("prepared Chutney wheel %s is invalid", entry.Name())
		}
	}
	archiveDigest, err := hashFile(filepath.Join(directory, inputs.Archive))
	if err != nil || archiveDigest != inputs.ArchiveSHA256 {
		return "", errors.New("prepared Chutney archive has the wrong digest")
	}
	return repositoryDigest, nil
}

func readReferenceLock(path string) (referenceInputs, error) {
	file, err := os.Open(path)
	if err != nil {
		return referenceInputs{}, err
	}
	defer file.Close()
	result := referenceInputs{Packages: make(map[string]string), Wheels: make(map[string]string)}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		fields := strings.Split(scanner.Text(), "\t")
		if lineNumber == 1 {
			if len(fields) != 3 || fields[0] != "meta" || fields[1] != "schema" || fields[2] != "carrier-lab-reference/v1" {
				return referenceInputs{}, errors.New("reference lock schema is invalid")
			}
			continue
		}
		switch {
		case len(fields) >= 5 && fields[0] == "package":
			if err := addReferencePackage(result.Packages, fields[3], fields[4]); err != nil {
				return referenceInputs{}, err
			}
		case len(fields) >= 6 && fields[0] == "tor" && fields[1] == "package":
			if err := addReferencePackage(result.Packages, fields[3], fields[4]); err != nil {
				return referenceInputs{}, err
			}
		case len(fields) >= 7 && fields[0] == "chutney" && fields[1] == "revision":
			result.Archive, result.ArchiveSHA256 = fields[3], fields[4]
		case len(fields) >= 7 && fields[0] == "wheel":
			if err := addReferencePackage(result.Wheels, fields[3], fields[4]); err != nil {
				return referenceInputs{}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return referenceInputs{}, err
	}
	if len(result.Packages) != 13 || len(result.Wheels) != 3 || result.Archive == "" || !validSHA256(result.ArchiveSHA256) {
		return referenceInputs{}, errors.New("reference lock input closure is incomplete")
	}
	return result, nil
}

func addReferencePackage(packages map[string]string, name, digest string) error {
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".deb") && !strings.HasSuffix(name, ".whl") || !validSHA256(digest) || packages[name] != "" {
		return errors.New("reference lock package entry is invalid or duplicated")
	}
	packages[name] = digest
	return nil
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
