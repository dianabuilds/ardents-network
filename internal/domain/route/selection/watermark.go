package selection

import (
	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

type generationWatermark struct {
	generation uint64
	name       string
}

func loadWatermark(root string) (generationWatermark, bool, error) {
	generation, name, exists, err := durablefile.LoadWatermark(root, readBounded, "entry generation watermark is invalid")
	if err != nil || !exists {
		return generationWatermark{}, exists, err
	}
	return generationWatermark{generation: generation, name: name}, true, nil
}

func replaceWatermark(root string, generation uint64, name string) error {
	return durablefile.ReplaceWatermark(root, generation, name, "entry generation watermark value is invalid")
}
