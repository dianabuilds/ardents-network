package update

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type rawFile struct {
	Name      string
	Bytes     []byte
	Size      int64
	IsRegular bool
	IsDirect  bool
	state     byte
}

type inventoryResult struct {
	RootPath                                           string
	Marker, Current, SchemaCurrent, RollbackRetirement rawFile
	RootDirs, RootFiles, UnknownRoot                   []string
	Generations, StagingDirs                           []generationFacts
	Transactions                                       []transactionFacts
	CurrentTemps, SchemaTemps                          []rawFile
	InterruptedSelection                               uint64
}

type generationFacts struct {
	Generation         uint64
	Temporary          bool
	HasArtifact        bool
	HasManifest        bool
	Artifact, Manifest rawFile
	DecodedManifest    manifestView
}

type transactionFacts struct {
	Generation uint64
	HasJournal bool
	Journal    journalRawEntries
}

type journalRawEntries map[string]rawFile

var errInventoryInvalid = errors.New("update inventory is invalid")

// collectInventory makes one bounded read-only snapshot after permanent-lock
// ownership. It records raw physical evidence but leaves journal decoding and
// row classification to their dedicated concerns.
func collectInventory(root string) (inventoryResult, error) {
	facts := inventoryResult{RootPath: root}
	if err := validateOwnedPath(root); err != nil {
		return facts, fmt.Errorf("%w: root path: %v", errInventoryInvalid, err)
	}
	entries, err := recoveryReadDir(root, maximumRootEntries)
	if err != nil {
		return facts, err
	}
	present := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		switch name {
		case "generations", "staging", "transactions":
			if !entry.IsDir() {
				return facts, fmt.Errorf("%w: %s is not a directory", errInventoryInvalid, name)
			}
			present[name] = true
			facts.RootDirs = append(facts.RootDirs, name)
		case lockFileName, rootMarkerName, "current", "schema-current", rollbackRetireName:
			if entry.IsDir() {
				return facts, fmt.Errorf("%w: %s is not a file", errInventoryInvalid, name)
			}
			present[name] = true
			facts.RootFiles = append(facts.RootFiles, name)
		default:
			if entry.IsDir() || (!isCanonicalCurrentTemp(name) && !isCanonicalSchemaTemp(name)) {
				return facts, fmt.Errorf("%w: unknown root child %q", errInventoryInvalid, name)
			}
			facts.RootFiles = append(facts.RootFiles, name)
		}
	}
	for _, name := range []string{"generations", "staging", "transactions", lockFileName, rootMarkerName, "current"} {
		if !present[name] {
			return facts, fmt.Errorf("%w: missing root child %q", errInventoryInvalid, name)
		}
	}
	if facts.Marker, err = recoveryReadFile(filepath.Join(root, rootMarkerName), int64(len(rootMarker))); err != nil || string(facts.Marker.Bytes) != rootMarker {
		return facts, fmt.Errorf("%w: marker invalid: %v", errInventoryInvalid, err)
	}
	if facts.Current, err = recoveryReadFile(filepath.Join(root, "current"), maximumRecordBytes); err != nil {
		return facts, fmt.Errorf("%w: current invalid: %v", errInventoryInvalid, err)
	}
	if present["schema-current"] {
		facts.SchemaCurrent, err = recoveryReadFile(filepath.Join(root, "schema-current"), int64(recordHeaderBytes+schemaRecordBodyBytes))
		if err != nil || len(facts.SchemaCurrent.Bytes) != recordHeaderBytes+schemaRecordBodyBytes {
			return facts, fmt.Errorf("%w: schema current invalid: %v", errInventoryInvalid, err)
		}
	}
	if present[rollbackRetireName] {
		facts.RollbackRetirement, err = recoveryReadFile(filepath.Join(root, rollbackRetireName), maximumRecordBytes)
		if err != nil {
			return facts, fmt.Errorf("%w: rollback retirement invalid: %v", errInventoryInvalid, err)
		}
	}
	if facts.Generations, err = readGenerationDir(filepath.Join(root, "generations"), maximumGenerationEntries); err != nil {
		return facts, err
	}
	if facts.StagingDirs, err = readStagingDir(filepath.Join(root, "staging"), maximumStagingEntries); err != nil {
		return facts, err
	}
	if facts.Transactions, err = readTransactions(filepath.Join(root, "transactions")); err != nil {
		return facts, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if isCanonicalCurrentTemp(name) {
			temp, readErr := recoveryReadFile(filepath.Join(root, name), maximumRecordBytes)
			if readErr != nil {
				return facts, fmt.Errorf("%w: current temp %q: %v", errInventoryInvalid, name, readErr)
			}
			temp.Name = name
			facts.CurrentTemps = append(facts.CurrentTemps, temp)
		}
		if isCanonicalSchemaTemp(name) {
			temp, readErr := recoveryReadFile(filepath.Join(root, name), int64(recordHeaderBytes+schemaRecordBodyBytes))
			if readErr != nil || len(temp.Bytes) != recordHeaderBytes+schemaRecordBodyBytes {
				return facts, fmt.Errorf("%w: schema temp %q: %v", errInventoryInvalid, name, readErr)
			}
			temp.Name = name
			facts.SchemaTemps = append(facts.SchemaTemps, temp)
		}
	}
	if len(facts.Transactions) == 1 && len(facts.Transactions[0].Journal) != 0 {
		facts.InterruptedSelection = facts.Transactions[0].Generation
	}
	return facts, nil
}

func readGenerationDir(root string, maximum int) ([]generationFacts, error) {
	return readPayloadDir(root, maximum)
}

// readStagingDir admits only a canonical complete staging generation or the
// S7.2-03 declared temporary name. Both are returned separately so recovery
// can reject their coexistence rather than silently choosing one.
func readStagingDir(root string, maximum int) ([]generationFacts, error) {
	entries, err := recoveryReadDir(root, maximum)
	if err != nil {
		return nil, err
	}
	result := make([]generationFacts, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		isTemporary := strings.HasSuffix(name, ".tmp")
		generationName := name
		if isTemporary {
			generationName = strings.TrimSuffix(name, ".tmp")
		}
		generation, parseErr := canonicalUint(generationName)
		if parseErr != nil || !entry.IsDir() || (isTemporary && name != strconv.FormatUint(generation, 10)+".tmp") {
			return nil, fmt.Errorf("%w: staging child %q invalid", errInventoryInvalid, name)
		}
		var facts generationFacts
		var factsErr error
		if isTemporary {
			facts, factsErr = readTemporaryPayloadFacts(filepath.Join(root, name), generation)
		} else {
			facts, factsErr = readPayloadFacts(filepath.Join(root, name), generation)
		}
		if factsErr != nil {
			return nil, factsErr
		}
		facts.Temporary = isTemporary
		result = append(result, facts)
	}
	return result, nil
}

func readTemporaryPayloadFacts(directory string, generation uint64) (generationFacts, error) {
	children, err := recoveryReadDir(directory, maximumPayloadEntries)
	if err != nil || len(children) > 2 {
		return generationFacts{}, fmt.Errorf("%w: temporary staging %d shape invalid", errInventoryInvalid, generation)
	}
	facts := generationFacts{Generation: generation, Temporary: true}
	for _, child := range children {
		switch child.Name() {
		case "artifact":
			facts.Artifact, err = recoveryReadFile(filepath.Join(directory, child.Name()), maximumArtifactBytes)
			facts.HasArtifact = err == nil
		case "manifest.bin":
			facts.Manifest, err = recoveryReadFile(filepath.Join(directory, child.Name()), maximumRecordBytes)
			facts.HasManifest = err == nil
		default:
			return generationFacts{}, fmt.Errorf("%w: temporary staging %d child invalid", errInventoryInvalid, generation)
		}
		if err != nil {
			return generationFacts{}, fmt.Errorf("%w: temporary staging %d payload: %v", errInventoryInvalid, generation, err)
		}
	}
	if facts.HasManifest {
		view, decodeErr := decodeManifest(facts.Manifest.Bytes)
		if decodeErr != nil || view.Generation != generation {
			return generationFacts{}, fmt.Errorf("%w: temporary staging %d manifest invalid", errInventoryInvalid, generation)
		}
		facts.DecodedManifest = view
	}
	return facts, nil
}

func readPayloadDir(root string, maximum int) ([]generationFacts, error) {
	entries, err := recoveryReadDir(root, maximum)
	if err != nil {
		return nil, err
	}
	result := make([]generationFacts, 0, len(entries))
	for _, entry := range entries {
		generation, err := canonicalUint(entry.Name())
		if err != nil || !entry.IsDir() {
			return nil, fmt.Errorf("%w: generation child %q invalid", errInventoryInvalid, entry.Name())
		}
		facts, factsErr := readPayloadFacts(filepath.Join(root, entry.Name()), generation)
		if factsErr != nil {
			return nil, factsErr
		}
		result = append(result, facts)
	}
	return result, nil
}

func readPayloadFacts(directory string, generation uint64) (generationFacts, error) {
	children, err := recoveryReadDir(directory, maximumPayloadEntries)
	if err != nil || len(children) != 2 || children[0].Name() != "artifact" || children[1].Name() != "manifest.bin" {
		return generationFacts{}, fmt.Errorf("%w: generation %d shape invalid", errInventoryInvalid, generation)
	}
	artifact, artifactErr := recoveryReadFile(filepath.Join(directory, "artifact"), maximumArtifactBytes)
	manifest, manifestErr := recoveryReadFile(filepath.Join(directory, "manifest.bin"), maximumRecordBytes)
	if err := errors.Join(artifactErr, manifestErr); err != nil {
		return generationFacts{}, fmt.Errorf("%w: generation %d payload: %v", errInventoryInvalid, generation, err)
	}
	view, decodeErr := decodeManifest(manifest.Bytes)
	artifactDigest := sha256.Sum256(artifact.Bytes)
	if decodeErr != nil || view.Generation != generation || view.Length != uint64(len(artifact.Bytes)) || view.Artifact != artifactDigest {
		return generationFacts{}, fmt.Errorf("%w: generation %d manifest mismatch", errInventoryInvalid, generation)
	}
	return generationFacts{Generation: generation, HasArtifact: true, HasManifest: true,
		Artifact: artifact, Manifest: manifest, DecodedManifest: view}, nil
}

func readTransactions(root string) ([]transactionFacts, error) {
	entries, err := recoveryReadDir(root, maximumTransactionEntries)
	if err != nil {
		return nil, err
	}
	if len(entries) > 1 {
		return nil, fmt.Errorf("%w: multiple transactions", errInventoryInvalid)
	}
	result := make([]transactionFacts, 0, len(entries))
	for _, entry := range entries {
		generation, err := canonicalUint(entry.Name())
		if err != nil || !entry.IsDir() {
			return nil, fmt.Errorf("%w: transaction %q invalid", errInventoryInvalid, entry.Name())
		}
		children, err := recoveryReadDir(filepath.Join(root, entry.Name()), maximumTransactionEntries)
		if err != nil || len(children) != 1 || children[0].Name() != "journal" || !children[0].IsDir() {
			return nil, fmt.Errorf("%w: transaction %d shape invalid", errInventoryInvalid, generation)
		}
		journal, err := readJournalRaw(filepath.Join(root, entry.Name(), "journal"))
		if err != nil {
			return nil, err
		}
		result = append(result, transactionFacts{Generation: generation, HasJournal: true, Journal: journal})
	}
	return result, nil
}

func canonicalUint(name string) (uint64, error) {
	value, err := strconv.ParseUint(name, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != name {
		return 0, fmt.Errorf("non-canonical numeric name %q", name)
	}
	return value, nil
}

func isCanonicalCurrentTemp(name string) bool {
	const prefix, suffix = ".current.", ".tmp"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return false
	}
	inner := name[len(prefix) : len(name)-len(suffix)]
	if len(inner) != 16 {
		return false
	}
	for _, character := range inner {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func isCanonicalSchemaTemp(name string) bool {
	const prefix, suffix = ".schema-current.", ".tmp"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return false
	}
	inner := name[len(prefix) : len(name)-len(suffix)]
	if len(inner) != 16 {
		return false
	}
	for _, character := range inner {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func (facts inventoryResult) journalLookup(generation uint64) journalRawEntries {
	for _, transaction := range facts.Transactions {
		if transaction.Generation == generation {
			return transaction.Journal
		}
	}
	return nil
}
