package entry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type generationWatermark struct {
	generation uint64
	name       string
}

func loadWatermark(root string) (generationWatermark, bool, error) {
	raw, err := readBounded(filepath.Join(root, "watermark"), 96)
	if os.IsNotExist(err) {
		return generationWatermark{}, false, nil
	}
	if err != nil {
		return generationWatermark{}, false, err
	}
	line := strings.TrimSuffix(string(raw), "\n")
	parts := strings.Split(line, " ")
	if len(parts) != 2 || string(raw) != line+"\n" || !stateName.MatchString(parts[1]) {
		return generationWatermark{}, false, errors.New("entry generation watermark is invalid")
	}
	generation, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || generation == 0 || fmt.Sprintf("%d %s\n", generation, parts[1]) != string(raw) {
		return generationWatermark{}, false, errors.New("entry generation watermark is invalid")
	}
	return generationWatermark{generation: generation, name: parts[1]}, true, nil
}

func replaceWatermark(root string, generation uint64, name string) error {
	if generation == 0 || !stateName.MatchString(name) {
		return errors.New("entry generation watermark value is invalid")
	}
	temporary, err := os.CreateTemp(root, ".watermark-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(fmt.Sprintf("%d %s\n", generation, name))
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(path, filepath.Join(root, "watermark")); err != nil {
		return err
	}
	return syncDirectory(root)
}
