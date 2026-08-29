package resource

import (
	"errors"
	"io/fs"
	"path/filepath"
)

func measureManagedStorage(roots []string, maximumBytes uint64, maximumFiles int) (uint64, uint64, error) {
	var total uint64
	files := 0
	for _, root := range roots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return 0, 0, errors.New("managed storage root is not a clean absolute path")
		}
		err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() || info.Size() < 0 {
				return errors.New("managed storage contains a non-regular entry")
			}
			files++
			if files > maximumFiles || uint64(info.Size()) > maximumBytes-total {
				return errors.New("managed storage exceeds its profile ceiling")
			}
			total += uint64(info.Size())
			return nil
		})
		if err != nil {
			return 0, 0, err
		}
	}
	return total, uint64(files), nil
}
